package pipeline

import (
	"context"
	"errors"
	"fmt"

	"go.opencensus.io/trace"
)

type InputBlock struct {
	BlockName     BlockName
	FunctionName  string
	FunctionInput map[string]string
	NextStep      BlockName
}

var (
	errValueNotFound = errors.New("value not found")
)

func (i *InputBlock) Run(ctx context.Context, runCtx *VariableStore) error {
	_, s := trace.StartSpan(ctx, "run_input_block")
	defer s.End()

	runCtx.AddStep(i.BlockName)

	for k, v := range i.FunctionInput {
		_, ok := runCtx.GetValue(v)
		if !ok {
			return fmt.Errorf("%w for %s", errValueNotFound, k)
		}
	}

	return nil
}

func (i *InputBlock) Next() BlockName {
	return i.NextStep
}
