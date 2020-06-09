package pipeline

import (
	"context"
	"fmt"
)

type InputBlock struct {
	BlockName     string
	FunctionName  string
	FunctionInput map[string]string
	NextStep      string
}

func (i *InputBlock) Run(ctx context.Context, runCtx *VariableStore) error {
	runCtx.AddStep(i.BlockName)
	for k, v := range i.FunctionInput {
		_, ok := runCtx.GetValue(v)
		if !ok {
			return fmt.Errorf("Value for %s not found", k)
		}
	}
	return nil
}

func (i *InputBlock) Next() string {
	return i.NextStep
}
