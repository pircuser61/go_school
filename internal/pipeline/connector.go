package pipeline

import (
	"context"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"

	"go.opencensus.io/trace"
)

type ConnectorBlock struct {
	Name           string
	FunctionName   string
	FunctionInput  map[string]string
	FunctionOutput map[string]string
	NextStep       string
}

func (cb *ConnectorBlock) Run(ctx context.Context, runCtx *store.VariableStore) error {
	return cb.DebugRun(ctx, runCtx)
}

func (cb *ConnectorBlock) DebugRun(ctx context.Context, runCtx *store.VariableStore) error {
	runCtx.AddStep(cb.Name)

	_, s := trace.StartSpan(ctx, "run_connector_block")
	defer s.End()

	values := make(map[string]interface{})

	for ikey, gkey := range cb.FunctionInput {
		val, _ := runCtx.GetValue(gkey) // if no value - empty value
		values[ikey] = val
	}

	for _, gkey := range cb.FunctionOutput {
		for _, val := range values {
			if val == nil {
				continue
			}

			runCtx.SetValue(gkey, val)

			break
		}
	}

	return nil
}

func (cb *ConnectorBlock) Next(runCtx *store.VariableStore) (string, bool) {
	return cb.NextStep, true
}

func (cb *ConnectorBlock) NextSteps() []string {
	return []string{cb.NextStep}
}

func (cb ConnectorBlock) IsScenario() bool {
	return false
}

func (cb ConnectorBlock) Inputs() map[string]string {
	return cb.FunctionInput
}

func (cb ConnectorBlock) Outputs() map[string]string {
	return cb.FunctionOutput
}
