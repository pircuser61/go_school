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

func (gb *GoEndBlock) GetTaskHumanStatus() TaskHumanStatus {
	// should not change status returned by worker nodes like approvement, execution, etc.
	return ""
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

	if _, ok := gb.expectedEvents[eventEnd]; ok {
		event, eventErr := gb.RunContext.MakeNodeStartEvent(ctx, gb.Name, gb.GetTaskHumanStatus(), gb.GetStatus())
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
		event, err := runCtx.MakeNodeStartEvent(ctx, name, b.GetTaskHumanStatus(), b.GetStatus())
		if err != nil {
			return nil, false, err
		}
		b.happenedEvents = append(b.happenedEvents, event)
	}

	return b, reEntry, nil
}
