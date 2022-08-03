package pipeline

import (
	"context"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type GoEndBlock struct {
	Name   string
	Title  string
	Input  map[string]string
	Output map[string]string
	Nexts  map[string][]string
}

func (gb *GoEndBlock) GetStatus() Status {
	return StatusFinished
}

func (gb *GoEndBlock) GetTaskHumanStatus() TaskHumanStatus {
	// should not change status returned by worker nodes like approvement, execution, etc.
	return ""
}

func (gb *GoEndBlock) GetType() string {
	return BlockGoEndId
}

func (gb *GoEndBlock) Inputs() map[string]string {
	return gb.Input
}

func (gb *GoEndBlock) Outputs() map[string]string {
	return gb.Output
}

func (gb *GoEndBlock) IsScenario() bool {
	return false
}

// nolint:dupl // not dupl?
func (gb *GoEndBlock) DebugRun(ctx context.Context, _ *stepCtx, runCtx *store.VariableStore) error {
	_, s := trace.StartSpan(ctx, "run_go_block")
	defer s.End()

	runCtx.AddStep(gb.Name)

	values := make(map[string]interface{})

	for ikey, gkey := range gb.Input {
		val, ok := runCtx.GetValue(gkey) // if no value - empty value
		if ok {
			values[ikey] = val
		}
	}

	for ikey, gkey := range gb.Output {
		val, ok := values[ikey]
		if ok {
			runCtx.SetValue(gkey, val)
		}
	}

	return nil
}

func (gb *GoEndBlock) Next(_ *store.VariableStore) ([]string, bool) {
	return nil, true
}

func (gb *GoEndBlock) Skipped(_ *store.VariableStore) []string {
	return nil
}

func (gb *GoEndBlock) GetState() interface{} {
	return nil
}

func (gb *GoEndBlock) Update(_ context.Context, _ *script.BlockUpdateData) (interface{}, error) {
	return nil, nil
}

func (gb *GoEndBlock) Model() script.FunctionModel {
	return script.FunctionModel{
		ID:        BlockGoEndId,
		BlockType: script.TypeGo,
		Title:     BlockGoEndTitle,
		Inputs:    nil,
		Outputs:   nil,
		Sockets:   []string{}, // TODO: по идее, тут нет никаких некстов, возможно, в будущем они понадобятся
	}
}

func createGoEndBlock(name string, ef *entity.EriusFunc) *GoEndBlock {
	b := &GoEndBlock{
		Name:   name,
		Title:  ef.Title,
		Input:  map[string]string{},
		Output: map[string]string{},
		Nexts:  ef.Next,
	}

	for _, v := range ef.Input {
		b.Input[v.Name] = v.Global
	}

	for _, v := range ef.Output {
		b.Output[v.Name] = v.Global
	}
	return b
}
