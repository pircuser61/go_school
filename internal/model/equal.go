package model

import (
	"context"
	"github.com/pkg/errors"
)

type IF struct {
	FunctionName  string
	FunctionInput map[string]string
	Result        bool
	OnTrue        string
	OnFalse       string
}

func (e *IF) Run(ctx context.Context, runCtx *VariableStore) error {
	r, err := runCtx.GetBoolWithInput(e.FunctionInput, "var")
	if err != nil {
		return err
	}
	e.Result = r
	return nil
}

func (e *IF) Next() string {
	if e.Result {
		return e.OnTrue
	}
	return e.OnFalse
}

func NewIF(content map[string]interface{}) (Runner, error) {
	fname, ok := content["name"].(string)
	if !ok {
		fname = ""
	}
	onTrue, ok := content["on_true"].(string)
	if !ok {
		onTrue = ""
	}
	onFalse, ok := content["on_false"].(string)
	if !ok {
		onTrue = ""
	}
	inputs, ok := content["input"].([]interface{})
	if !ok {
		return nil, errors.New("invalid input format")
	}
	finput, err := createFuncParams(inputs)
	if err != nil {
		return nil, errors.Errorf("invalid input format: %s", err.Error())
	}

	i := IF{
		FunctionName:  fname,
		FunctionInput: finput,
		OnFalse:       onFalse,
		OnTrue:        onTrue,
	}
	return &i, nil
}
