package pipeline

import (
	"context"
	"errors"
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

	return nil
}

func (i *InputBlock) Next() string {
	return i.NextStep
}

func (i InputBlock) IsScenario() bool {
	return false
}

func (i InputBlock) Inputs() map[string]string {
	return i.FunctionInput
}

func (i InputBlock) Outputs() map[string]string {
	return make(map[string]string)
}
