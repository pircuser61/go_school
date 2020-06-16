package pipeline

import (
	"context"

	"go.opencensus.io/trace"
)

type ConnectorBlock struct {
	Name           BlockName
	FunctionName   string
	FunctionInput  map[string]string
	FunctionOutput map[string]string
	NextStep       BlockName
}

func (cb *ConnectorBlock) Run(ctx context.Context, store *VariableStore) error {
	store.AddStep(cb.Name)

	_, s := trace.StartSpan(ctx, "run_connector_block")
	defer s.End()

	values := make(map[string]interface{})

	for ikey, gkey := range cb.FunctionInput {
		val, _ := store.GetValue(gkey) // if no value - empty value
		values[ikey] = val
	}

	for _, gkey := range cb.FunctionOutput {
		for _, val := range values {
			if val == nil {
				continue
			}

			store.SetValue(gkey, val)

			break
		}
	}

	return nil
}

func (cb *ConnectorBlock) Next() BlockName {
	return cb.NextStep
}
