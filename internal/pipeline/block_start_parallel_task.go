package pipeline

import (
	"context"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type BeginParallelData struct{}

type GoBeginParallelTaskBlock struct {
	Name       string
	ShortName  string
	Title      string
	Input      map[string]string
	Output     map[string]string
	Sockets    []script.Socket
	RunContext *BlockRunContext

	expectedEvents      map[string]struct{}
	happenedEvents      []entity.NodeEvent
	happenedKafkaEvents []entity.NodeKafkaEvent
}

func (gb *GoBeginParallelTaskBlock) CurrentExecutorData() CurrentExecutorData {
	return CurrentExecutorData{}
}

func (gb *GoBeginParallelTaskBlock) GetNewEvents() []entity.NodeEvent {
	return gb.happenedEvents
}

func (gb *GoBeginParallelTaskBlock) GetNewKafkaEvents() []entity.NodeKafkaEvent {
	return gb.happenedKafkaEvents
}

func (gb *GoBeginParallelTaskBlock) Members() []Member {
	return nil
}

func (gb *GoBeginParallelTaskBlock) Deadlines(_ context.Context) ([]Deadline, error) {
	return []Deadline{}, nil
}

func (gb *GoBeginParallelTaskBlock) UpdateManual() bool {
	return false
}

func (gb *GoBeginParallelTaskBlock) GetStatus() Status {
	return StatusFinished
}

func (gb *GoBeginParallelTaskBlock) GetTaskHumanStatus() (status TaskHumanStatus, comment, action string) {
	return StatusExecution, "", ""
}

func (gb *GoBeginParallelTaskBlock) Next(_ *store.VariableStore) ([]string, bool) {
	nexts, ok := script.GetNexts(gb.Sockets, DefaultSocketID)
	if !ok {
		return nil, false
	}

	return nexts, true
}

func (gb *GoBeginParallelTaskBlock) GetState() interface{} {
	return nil
}

func (gb *GoBeginParallelTaskBlock) Update(ctx context.Context) (interface{}, error) {
	err := gb.RunContext.Services.Storage.UnsetIsActive(ctx, gb.RunContext.WorkNumber, gb.Name)
	if err != nil {
		return nil, err
	}

	if _, ok := gb.expectedEvents[eventEnd]; ok {
		status, _, _ := gb.GetTaskHumanStatus()

		event, eventErr := gb.RunContext.MakeNodeEndEvent(ctx, MakeNodeEndEventArgs{
			NodeName:      gb.Name,
			NodeShortName: gb.ShortName,
			HumanStatus:   status,
			NodeStatus:    gb.GetStatus(),
		})
		if eventErr != nil {
			return nil, eventErr
		}

		gb.happenedEvents = append(gb.happenedEvents, event)
	}

	return nil, nil
}

func (gb *GoBeginParallelTaskBlock) Model() script.FunctionModel {
	return script.FunctionModel{
		ID:        BlockGoBeginParallelTaskID,
		BlockType: script.TypeGo,
		Title:     BlockGoBeginParallelTaskTitle,
		Inputs:    nil,
		Outputs:   nil,
		Sockets: []script.Socket{
			script.DefaultSocket,
		},
	}
}

func (gb *GoBeginParallelTaskBlock) BlockAttachments() (ids []string) {
	return ids
}

//nolint:dupl,unparam //its not duplicate
func createGoStartParallelBlock(ctx context.Context, name string, ef *entity.EriusFunc, runCtx *BlockRunContext,
	expectedEvents map[string]struct{},
) (*GoBeginParallelTaskBlock, bool, error) {
	const reEntry = false

	b := &GoBeginParallelTaskBlock{
		Name:       name,
		ShortName:  ef.ShortTitle,
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
		//nolint:gocritic //в этом проекте не принято использовать поинтеры в коллекциях
		for propertyName, v := range ef.Output.Properties {
			if v.Global == "" {
				continue
			}
			b.Output[propertyName] = v.Global
		}
	}

	b.RunContext.VarStore.AddStep(b.Name)

	if _, ok := b.expectedEvents[eventStart]; ok {
		status, _, _ := b.GetTaskHumanStatus()

		event, err := runCtx.MakeNodeStartEvent(ctx, MakeNodeStartEventArgs{
			NodeName:      name,
			NodeShortName: ef.ShortTitle,
			HumanStatus:   status,
			NodeStatus:    b.GetStatus(),
		})
		if err != nil {
			return nil, false, err
		}

		b.happenedEvents = append(b.happenedEvents, event)
	}

	return b, reEntry, nil
}

func (gb *GoBeginParallelTaskBlock) UpdateStateUsingOutput(context.Context, []byte) (state map[string]interface{}, err error) {
	return nil, nil
}

func (gb *GoBeginParallelTaskBlock) UpdateOutputUsingState(context.Context) (output map[string]interface{}, err error) {
	return nil, nil
}
