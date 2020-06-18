package pipeline

import (
	"context"
	"errors"
	"fmt"
	"gitlab.services.mts.ru/erius/pipeliner/internal/store"

	"go.opencensus.io/trace"
)

type InputBlock struct {
	BlockName     string
	FunctionName  string
	FunctionInput map[string]string
	NextStep      string
}

var (
	errValueNotFound = errors.New("value not found")
)

func (i *InputBlock) Run(ctx context.Context, runCtx *store.VariableStore) error {
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

func (i *InputBlock) Next() string {
	return i.NextStep
}
