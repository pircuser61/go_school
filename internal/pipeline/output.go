package pipeline

import (
	"context"
	"fmt"

	"go.opencensus.io/trace"
)

type OutputBlock struct {
	BlockName      BlockName
	FunctionName   string
	FunctionOutput map[string]string
	NextStep       BlockName
}

func (i *OutputBlock) Run(ctx context.Context, runCtx *VariableStore) error {
	_, s := trace.StartSpan(ctx, "run_output_block")
	defer s.End()

	runCtx.AddStep(i.BlockName)

	for k, v := range i.FunctionOutput {
		_, ok := runCtx.GetValue(v)
		if !ok {
			return fmt.Errorf("%w: for %v", errValueNotFound, k)
		}
	}

	return nil
}

func (i *OutputBlock) Next() BlockName {
	return i.NextStep
}
