package pipeline

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
)

type FunctionBlock struct {
	BlockName      string
	FunctionName   string
	FunctionInput  map[string]string
	FunctionOutput map[string]string
	NextStep       string
	runURL	   string
}

func (fb *FunctionBlock) Run(ctx context.Context, store *VariableStore) error {
	ctx, s := trace.StartSpan(ctx, "run_function_block")
	defer s.End()

	url := fmt.Sprintf("https://openfaas.dev.autobp.mts.ru/function/%s.openfaas-fn", fb.FunctionName)
	fmt.Println(url)
	return nil
}

func (fb *FunctionBlock) Next() string {
	return fb.NextStep
}

func NewFunction(name string, content map[string]interface{}) (*FunctionBlock, error) {
	fname, ok := content["name"].(string)
	if !ok {
		fname = ""
	}
	next, ok := content["next"].(string)
	if !ok {
		next = ""
	}
	inputs, ok := content["input"].([]interface{})
	if !ok {
		return nil, errors.New("invalid input format")
	}
	finput, err := createFuncParams(inputs)
	if err != nil {
		return nil, errors.Errorf("invalid input format: %s", err.Error())
	}
	outputs, ok := content["output"].([]interface{})
	foutput, err := createFuncParams(outputs)
	if err != nil {
		return nil, errors.Errorf("invalid output format: %s", err.Error())
	}
	fb := FunctionBlock{
		BlockName:      name,
		FunctionName:   fname,
		FunctionInput:  finput,
		FunctionOutput: foutput,
		NextStep:       next,
	}
	return &fb, nil
}

func createFuncParams(inp []interface{}) (map[string]string, error) {
	out := make(map[string]string)
	for _, v := range inp {
		inputParams, ok := v.(map[string]interface{})
		if !ok {
			return nil, errors.Errorf("can't convert %v to map", v)
		}
		varN, ok := inputParams["name"]
		if !ok {
			return nil, errors.New("can't get variable name")
		}
		varGN, ok := inputParams["global"]
		if !ok {
			return nil, errors.New("can't get variable global name")
		}
		varName, ok := varN.(string)
		if !ok {
			return nil, errors.Errorf("can't convert %v to string", varN)
		}
		varGlobalName, ok := varGN.(string)
		if !ok {
			return nil, errors.Errorf("can't convert %v to string", varGN)
		}
		out[varName] = varGlobalName
	}
	return out, nil
}
