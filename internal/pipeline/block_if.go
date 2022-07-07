package pipeline

import (
	"context"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

const (
	keyIf string = "check"
)

type IF struct {
	Name          string
	FunctionName  string
	FunctionInput map[string]string
	Result        bool
	Nexts         map[string][]string
}

func (e *IF) GetStatus() Status {
	return StatusFinished
}

func (e *IF) GetTaskHumanStatus() TaskHumanStatus {
	return ""
}

func (e *IF) GetType() string {
	return BlockInternalIf
}

func (e *IF) Next(runCtx *store.VariableStore) ([]string, bool) {
	r, err := runCtx.GetBoolWithInput(e.FunctionInput, keyIf)
	if err != nil {
		return []string{}, false
	}

	if r {
		nexts, ok := e.Nexts[trueSocket]
		if !ok {
			return nil, false
		}
		return nexts, true
	}

	nexts, ok := e.Nexts[falseSocket]
	if !ok {
		return nil, false
	}
	return nexts, true
}

func (e *IF) Inputs() map[string]string {
	return e.FunctionInput
}

func (e *IF) Outputs() map[string]string {
	return make(map[string]string)
}

func (e *IF) IsScenario() bool {
	return false
}

func (e *IF) Run(ctx context.Context, runCtx *store.VariableStore) error {
	return e.DebugRun(ctx, runCtx)
}

func (e *IF) DebugRun(ctx context.Context, runCtx *store.VariableStore) error {
	_, s := trace.StartSpan(ctx, "run_if_block")
	defer s.End()

	runCtx.AddStep(e.Name)

	r, err := runCtx.GetBoolWithInput(e.FunctionInput, keyIf)
	if err != nil {
		return err
	}

	e.Result = r

	return nil
}

func (e *IF) GetState() interface{} {
	return nil
}

func (e *IF) Update(_ context.Context, _ *script.BlockUpdateData) (interface{}, error) {
	return nil, nil
}

func (e *IF) Model() script.FunctionModel {
	return script.FunctionModel{
		ID:        BlockGoIfID,
		BlockType: script.TypeIF,
		Title:     BlockGoIfTitle,
		Inputs:    nil,
		Outputs:   nil,
		Params: &script.FunctionParams{
			Type: BlockGoIfID,
			Params: &script.ConditionParams{
				Type: "",
			},
		},
		Sockets: []string{DefaultSocket},
	}
}
