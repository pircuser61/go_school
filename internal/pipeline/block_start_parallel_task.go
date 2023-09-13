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
	Title      string
	Input      map[string]string
	Output     map[string]string
	Sockets    []script.Socket
	RunContext *BlockRunContext

	expectedEvents map[string]struct{}
	happenedEvents []entity.NodeEvent
}

func (gb *GoBeginParallelTaskBlock) GetNewEvents() []entity.NodeEvent {
	return gb.happenedEvents
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

func (gb *GoBeginParallelTaskBlock) GetTaskHumanStatus() TaskHumanStatus {
	return StatusExecution
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
	if _, ok := gb.expectedEvents[eventEnd]; ok {
		event, eventErr := gb.RunContext.MakeNodeStartEvent(ctx, gb.Name, gb.GetTaskHumanStatus(), gb.GetStatus())
		if eventErr != nil {
			return nil, eventErr
		}
		gb.happenedEvents = append(gb.happenedEvents, event)
	}

	return nil, nil
}

func (gb *GoBeginParallelTaskBlock) Model() script.FunctionModel {
	return script.FunctionModel{
		ID:        BlockGoBeginParallelTaskId,
		BlockType: script.TypeGo,
		Title:     BlockGoBeginParallelTaskTitle,
		Inputs:    nil,
		Outputs:   nil,
		Sockets: []script.Socket{
			script.DefaultSocket,
		},
	}
}

//nolint:dupl,unparam //its not duplicate
func createGoStartParallelBlock(ctx context.Context, name string, ef *entity.EriusFunc, runCtx *BlockRunContext,
	expectedEvents map[string]struct{}) (*GoBeginParallelTaskBlock, bool, error) {
	const reEntry = false

	b := &GoBeginParallelTaskBlock{
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
