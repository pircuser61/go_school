package pipeline

import (
	"context"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type GoEndBlock struct {
	Name       string
	Title      string
	Input      map[string]string
	Output     map[string]string
	Sockets    []script.Socket
	RunContext *BlockRunContext
}

func (gb *GoEndBlock) Members() []Member {
	return nil
}

func (gb *GoEndBlock) Deadlines() []Deadline {
	return []Deadline{}
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

func (gb *GoEndBlock) Update(ctx context.Context) (interface{}, error) {
	if err := gb.RunContext.Storage.StopTaskBlocks(ctx, gb.RunContext.TaskID); err != nil {
		return nil, err
	}
	if err := gb.RunContext.updateTaskStatus(ctx, db.RunStatusFinished); err != nil {
		return nil, err
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

//nolint:dupl //its not duplicate
func createGoEndBlock(name string, ef *entity.EriusFunc, runCtx *BlockRunContext) *GoEndBlock {
	b := &GoEndBlock{
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
