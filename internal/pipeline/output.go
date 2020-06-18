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

func (i *OutputBlock) Run(ctx context.Context, runCtx *store.VariableStore) error {
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

func (i *OutputBlock) Next() string {
	return i.NextStep
}
