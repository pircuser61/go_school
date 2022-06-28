package pipeline

import (
	"context"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type GoWaitForAllInputsBlock struct {
	Name             string
	Title            string
	Input            map[string]string
	Output           map[string]string
	NextStep         []string
	IncomingBlockIds []string
}

func (gb *GoWaitForAllInputsBlock) GetStatus() Status {
	return StatusFinished
}

func (gb *GoWaitForAllInputsBlock) GetTaskHumanStatus() TaskHumanStatus {
	return StatusNew
}

func (gb *GoWaitForAllInputsBlock) GetType() string {
	return BlockWaitForAllInputsId
}

func (gb *GoWaitForAllInputsBlock) Inputs() map[string]string {
	return gb.Input
}

func (gb *GoWaitForAllInputsBlock) Outputs() map[string]string {
	return gb.Output
}

func (gb *GoWaitForAllInputsBlock) IsScenario() bool {
	return false
}

func (gb *GoWaitForAllInputsBlock) Run(ctx context.Context, runCtx *store.VariableStore) error {
	return gb.DebugRun(ctx, runCtx)
}

// nolint:dupl // not dupl?
func (gb *GoWaitForAllInputsBlock) DebugRun(ctx context.Context, runCtx *store.VariableStore) error {
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

func (gb *GoWaitForAllInputsBlock) Next(_ *store.VariableStore) ([]string, bool) {
	return gb.NextStep, true
}

func (gb *GoWaitForAllInputsBlock) NextSteps() []string {
	nextSteps := gb.NextStep

	return nextSteps
}

func (gb *GoWaitForAllInputsBlock) GetState() interface{} {
	return nil
}

func (gb *GoWaitForAllInputsBlock) Update(_ context.Context, _ *script.BlockUpdateData) (interface{}, error) {
	return nil, nil
}

func (gb *GoWaitForAllInputsBlock) Model() script.FunctionModel {
	return script.FunctionModel{
		ID:        BlockWaitForAllInputsId,
		BlockType: script.TypeGo,
		Title:     BlockGoWaitForAllInputsTitle,
		Inputs:    nil,
		Outputs:   nil,
		NextFuncs: []string{script.Next},
	}
}

func createGoWaitForAllInputsBlock(name string, ef *entity.EriusFunc) *GoWaitForAllInputsBlock {
	b := &GoWaitForAllInputsBlock{
		Name:     name,
		Title:    ef.Title,
		Input:    map[string]string{},
		Output:   map[string]string{},
		NextStep: ef.Next,
	}

	for _, v := range ef.Input {
		b.Input[v.Name] = v.Global
	}

	for _, v := range ef.Output {
		b.Output[v.Name] = v.Global
	}

	return b
}
