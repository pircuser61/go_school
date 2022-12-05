package pipeline

import (
	"context"
	"time"

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
}

func (gb *GoBeginParallelTaskBlock) Members() []Member {
	return nil
}

func (gb *GoBeginParallelTaskBlock) CheckSLA() (bool, bool, time.Time) {
	return false, false, time.Time{}
}

func (gb *GoBeginParallelTaskBlock) UpdateManual() bool {
	return false
}

func (gb *GoBeginParallelTaskBlock) GetStatus() Status {
	return StatusFinished
}

func (gb *GoBeginParallelTaskBlock) GetTaskHumanStatus() TaskHumanStatus {
	return ""
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

func (gb *GoBeginParallelTaskBlock) Update(_ context.Context) (interface{}, error) {
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

//nolint:dupl //its not duplicate
func createGoStartParallelBlock(name string, ef *entity.EriusFunc, runCtx *BlockRunContext) *GoBeginParallelTaskBlock {
	b := &GoBeginParallelTaskBlock{
		Name:       name,
		Title:      ef.Title,
		Input:      map[string]string{},
		Output:     map[string]string{},
		Sockets:    entity.ConvertSocket(ef.Sockets),
		RunContext: runCtx,
	}

	for _, v := range ef.Input {
		b.Input[v.Name] = v.Global
	}

	for _, v := range ef.Output {
		b.Output[v.Name] = v.Global
	}

	b.RunContext.VarStore.AddStep(b.Name)
	return b
}
