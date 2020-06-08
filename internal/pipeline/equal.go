package pipeline

import (
	"context"
)

type IF struct {
	BlockName      string
	FunctionName  string
	FunctionInput map[string]string
	Result        bool
	OnTrue        string
	OnFalse       string
}

func (e *IF) Run(ctx context.Context, runCtx *VariableStore) error {
	r, err := runCtx.GetBoolWithInput(e.FunctionInput, "check")
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

type StringsEqual struct {
	BlockName      string
	FunctionName  string
	FunctionInput map[string]string
	Result        bool
	OnTrue        string
	OnFalse       string
}

func (e *StringsEqual) Run(ctx context.Context, runCtx *VariableStore) error {
	allparams := make([]string, 0, len(e.FunctionInput))
	for k := range e.FunctionInput {
		r, err := runCtx.GetStringWithInput(e.FunctionInput, k)
		if err != nil {
			return err
		}
		allparams = append(allparams, r)
	}
	if len(allparams) >= 2 {
		for _, v := range allparams {
			e.Result = allparams[0] == v
			if !e.Result {
				return nil
			}
		}
	}
	return nil
}

func (e *StringsEqual) Next() string {
	if e.Result {
		return e.OnTrue
	}
	return e.OnFalse
}
