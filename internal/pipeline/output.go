package pipeline

import (
	"context"
	"fmt"
)

type OutputBlock struct {
	BlockName      string
	FunctionName   string
	FunctionOutput map[string]string
	NextStep       string
}

func (i *OutputBlock) Run(ctx context.Context, runCtx *VariableStore) error {
	runCtx.AddStep(i.BlockName)
	for k, v := range i.FunctionOutput {
		_, ok := runCtx.GetValue(v)
		if !ok {
			return fmt.Errorf("Value for %s not found", k)
		}
	}
	return nil
}

func (i *OutputBlock) Next() string {
	return i.NextStep
}
