package pipeline

import (
	"context"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type GoTestBlock struct {
	Name     string
	Title    string
	Input    map[string]string
	Output   map[string]string
	NextStep string
}

func (gb *GoTestBlock) GetTaskStatus() TaskHumanStatus {
	return ""
}

func (gb *GoTestBlock) GetType() string {
	return BlockGoTestID
}

func (gb *GoTestBlock) Inputs() map[string]string {
	return gb.Input
}

func (gb *GoTestBlock) Outputs() map[string]string {
	return gb.Output
}

func (gb *GoTestBlock) IsScenario() bool {
	return false
}

func (gb *GoTestBlock) Run(ctx context.Context, runCtx *store.VariableStore) error {
	return gb.DebugRun(ctx, runCtx)
}

func (gb *GoTestBlock) DebugRun(ctx context.Context, runCtx *store.VariableStore) error {
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

func (gb *GoTestBlock) Next(_ *store.VariableStore) (string, bool) {
	return gb.NextStep, true
}

func (gb *GoTestBlock) NextSteps() []string {
	nextSteps := []string{gb.NextStep}

	return nextSteps
}

func (gb *GoTestBlock) GetState() interface{} {
	return nil
}

func (gb *GoTestBlock) Update(_ context.Context, _ *script.BlockUpdateData) (interface{}, error) {
	return nil, nil
}
