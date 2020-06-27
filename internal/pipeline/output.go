package pipeline

import (
	"context"
	"fmt"
	"gitlab.services.mts.ru/erius/pipeliner/internal/store"

	"go.opencensus.io/trace"
)

type OutputBlock struct {
	BlockName      string
	FunctionName   string
	FunctionOutput map[string]string
	NextStep       string
}

func (o *OutputBlock) Run(ctx context.Context, runCtx *store.VariableStore, deep int) error {
	_, s := trace.StartSpan(ctx, "run_output_block")
	defer s.End()

	runCtx.AddStep(o.BlockName)

	for k, v := range o.FunctionOutput {
		_, ok := runCtx.GetValue(v)
		if !ok {
			return fmt.Errorf("%w: for %v", errValueNotFound, k)
		}
	}

	return nil
}

func (o *OutputBlock) Next() string {
	return o.NextStep
}

func (o OutputBlock) IsScenario() bool {
	return false
}



func  (o OutputBlock) Inputs() map[string]string {
	return  make(map[string]string)
}

func (o OutputBlock) Outputs() map[string]string {
	return o.FunctionOutput
}