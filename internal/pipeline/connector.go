package pipeline

import (
	"context"
	"fmt"
	"go.opencensus.io/trace"
)

type ConnectorBlock struct {
	BlockName      string
	FunctionName   string
	FunctionInput  map[string]string
	FunctionOutput map[string]string
	NextStep       string
}


func (fb *ConnectorBlock) Run(ctx context.Context, store *VariableStore) error {
	ctx, s := trace.StartSpan(ctx, "run_function_block")
	defer s.End()
	values := make(map[string]interface{})
	for ikey, gkey := range fb.FunctionInput {
		fmt.Println(ikey, gkey)
		val, _ := store.GetValue(gkey) // if no value - empty value
		values[ikey] = val
	}
	for k, val := range values {
		fmt.Println(k, val, val == nil)
	}
	for _, gkey := range fb.FunctionOutput {
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

func (fb *ConnectorBlock) Next() string {
	return fb.NextStep
}