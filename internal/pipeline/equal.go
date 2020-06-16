package pipeline

import (
	"context"

	"go.opencensus.io/trace"
)

type IF struct {
	Name          BlockName
	FunctionName  string
	FunctionInput map[string]string
	Result        bool
	OnTrue        BlockName
	OnFalse       BlockName
}

func (e *IF) Run(ctx context.Context, runCtx *VariableStore) error {
	_, s := trace.StartSpan(ctx, "run_if_block")
	defer s.End()

	runCtx.AddStep(e.Name)

	r, err := runCtx.GetBoolWithInput(e.FunctionInput, "check")
	if err != nil {
		return err
	}

	e.Result = r

	return nil
}

func (e *IF) Next() BlockName {
	if e.Result {
		return e.OnTrue
	}

	return e.OnFalse
}

type StringsEqual struct {
	Name          BlockName
	FunctionName  string
	FunctionInput map[string]string
	Result        bool
	OnTrue        BlockName
	OnFalse       BlockName
}

func (e *StringsEqual) Run(ctx context.Context, runCtx *VariableStore) error {
	_, s := trace.StartSpan(ctx, "run_strings_equal_block")
	defer s.End()

	runCtx.AddStep(e.Name)

	allparams := make([]string, 0, len(e.FunctionInput))

	for k := range e.FunctionInput {
		r, err := runCtx.GetStringWithInput(e.FunctionInput, k)
		if err != nil {
			return err
		}

		allparams = append(allparams, r)
	}

	const minVariablesCnt = 2
	if len(allparams) >= minVariablesCnt {
		for _, v := range allparams {
			e.Result = allparams[0] == v
			if !e.Result {
				return nil
			}
		}
	}

	return nil
}

func (e *StringsEqual) Next() BlockName {
	if e.Result {
		return e.OnTrue
	}

	return e.OnFalse
}
