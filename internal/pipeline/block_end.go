package pipeline

import (
	c "context"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type GoEndBlock struct {
	Name    string
	Title   string
	Input   map[string]string
	Output  map[string]string
	Sockets []script.Socket

	expectedEvents map[string]struct{}
	happenedEvents []entity.NodeEvent

	RunContext *BlockRunContext
}

func (gb *GoEndBlock) GetNewEvents() []entity.NodeEvent {
	return gb.happenedEvents
}

func (gb *GoEndBlock) Members() []Member {
	return nil
}

func (gb *GoEndBlock) Deadlines(_ c.Context) ([]Deadline, error) {
	return []Deadline{}, nil
}

func (gb *GoEndBlock) GetStatus() Status {
	return StatusFinished
}

func (gb *GoEndBlock) UpdateManual() bool {
	return false
}

func (gb *GoEndBlock) GetTaskHumanStatus() (TaskHumanStatus TaskHumanStatus, comment string) {
	// should not change status returned by worker nodes like approvement, execution, etc.
	return "", ""
}

func (gb *GoEndBlock) Next(_ *store.VariableStore) ([]string, bool) {
	return nil, true
}

func (gb *GoEndBlock) GetState() interface{} {
	return nil
}

func (gb *GoEndBlock) Update(ctx c.Context) (interface{}, error) {
	if err := gb.RunContext.Services.Storage.StopTaskBlocks(ctx, gb.RunContext.TaskID); err != nil {
		return nil, err
	}
	if err := gb.RunContext.updateTaskStatus(ctx, db.RunStatusFinished, "", db.SystemLogin); err != nil {
		return nil, err
	}

	nodeEvents, err := gb.RunContext.GetCancelledStepsEvents(ctx)
	if err != nil {
		return nil, err
	}
	for _, event := range nodeEvents {
		// event for this node will spawn later
		if event.NodeName == gb.Name {
			continue
		}
		gb.happenedEvents = append(gb.happenedEvents, event)
	}

	if _, ok := gb.expectedEvents[eventEnd]; ok {
		status, _ := gb.GetTaskHumanStatus()
		event, eventErr := gb.RunContext.MakeNodeEndEvent(ctx, gb.Name, status, gb.GetStatus())
		if eventErr != nil {
			return nil, eventErr
		}
		gb.happenedEvents = append(gb.happenedEvents, event)
	}

	return nil, nil
}

func (gb *GoEndBlock) Model() script.FunctionModel {
	return script.FunctionModel{
		ID:        BlockGoEndId,
		BlockType: script.TypeGo,
		Title:     BlockGoEndTitle,
		Inputs:    nil,
		Outputs:   nil,
		Sockets:   []script.Socket{}, // TODO: по идее, тут нет никаких некстов, возможно, в будущем они понадобятся
	}
}

//nolint:dupl,unparam //its not duplicate
func createGoEndBlock(ctx c.Context, name string, ef *entity.EriusFunc, runCtx *BlockRunContext,
	expectedEvents map[string]struct{}) (*GoEndBlock, bool, error) {
	const reEntry = false

	b := &GoEndBlock{
		Name:       name,
		Title:      ef.Title,
		Input:      map[string]string{},
		Output:     map[string]string{},
		Sockets:    entity.ConvertSocket(ef.Sockets),
		RunContext: runCtx,

		expectedEvents: expectedEvents,
		happenedEvents: make([]entity.NodeEvent, 0),
	}

	for _, v := range ef.Input {
		b.Input[v.Name] = v.Global
	}
	if ef.Output != nil {
		for propertyName, v := range ef.Output.Properties {
			b.Output[propertyName] = v.Global
		}
	}

	b.RunContext.VarStore.AddStep(b.Name)

	if _, ok := b.expectedEvents[eventStart]; ok {
		status, _ := b.GetTaskHumanStatus()
		event, err := runCtx.MakeNodeStartEvent(ctx, name, status, b.GetStatus())
		if err != nil {
			return nil, false, err
		}
		b.happenedEvents = append(b.happenedEvents, event)
	}

	return b, reEntry, nil
}
