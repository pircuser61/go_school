package pipeline

import (
	"context"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type GoStartBlock struct {
	Name     string
	Title    string
	Input    map[string]string
	Output   map[string]string
	NextStep []string
}

func (gb *GoStartBlock) GetStatus() Status {
	return StatusFinished
}

func (gb *GoStartBlock) GetTaskHumanStatus() TaskHumanStatus {
	return StatusNew
}

func (gb *GoStartBlock) GetType() string {
	return BlockGoStartId
}

func (gb *GoStartBlock) Inputs() map[string]string {
	return gb.Input
}

func (gb *GoStartBlock) Outputs() map[string]string {
	return gb.Output
}

func (gb *GoStartBlock) IsScenario() bool {
	return false
}

func (gb *GoStartBlock) Run(ctx context.Context, runCtx *store.VariableStore) error {
	return gb.DebugRun(ctx, runCtx)
}

// nolint:dupl // not dupl?
func (gb *GoStartBlock) DebugRun(ctx context.Context, runCtx *store.VariableStore) error {
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

func (gb *GoStartBlock) Next(_ *store.VariableStore) ([]string, bool) {
	return gb.NextStep, true
}

func (gb *GoStartBlock) NextSteps() []string {
	nextSteps := gb.NextStep

	return nextSteps
}

func (gb *GoStartBlock) GetState() interface{} {
	return nil
}

func (gb *GoStartBlock) Update(_ context.Context, _ *script.BlockUpdateData) (interface{}, error) {
	return nil, nil
}

func (gb *GoStartBlock) Model() script.FunctionModel {
	return script.FunctionModel{
		ID:        BlockGoStartId,
		BlockType: script.TypeGo,
		Title:     BlockGoStartTitle,
		Inputs:    nil,
		Outputs:   nil,
		NextFuncs: []string{script.Next},
	}
}

func createGoStartBlock(name string, ef *entity.EriusFunc) *GoStartBlock {
	b := &GoStartBlock{
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
