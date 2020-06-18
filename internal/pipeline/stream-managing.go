package pipeline

import (
	"context"
	"gitlab.services.mts.ru/erius/pipeliner/internal/store"

	"go.opencensus.io/trace"
)

type IF struct {
	Name          string
	FunctionName  string
	FunctionInput map[string]string
	Result        bool
	OnTrue        string
	OnFalse       string
}

func (e *IF) Run(ctx context.Context, runCtx *store.VariableStore) error {
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

func (e *IF) Next() string {
	if e.Result {
		return e.OnTrue
	}

	return e.OnFalse
}

type StringsEqual struct {
	Name          string
	FunctionName  string
	FunctionInput map[string]string
	Result        bool
	OnTrue        string
	OnFalse       string
}

func (e *StringsEqual) Run(ctx context.Context, runCtx *store.VariableStore) error {
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

func (e *StringsEqual) Next() string {
	if e.Result {
		return e.OnTrue
	}

	return e.OnFalse
}


type ForState struct {
	Name          string
	FunctionName  string
	FunctionInput map[string]string
	FunctionOutput map[string]string
	Result        bool
	OnTrue        string
	OnFalse       string
}

func (e *ForState) Run(ctx context.Context, runCtx *store.VariableStore) error {
	_, s := trace.StartSpan(ctx, "run_strings_equal_block")
	defer s.End()
	runCtx.AddStep(e.Name)
	// get input array, index
	// check index
	// return if index >= len
	// create new index if empty
	// set iter, set index
	return nil
}

func (e *ForState) Next() string {
	if e.Result {
		return e.OnTrue
	}

	return e.OnFalse
}
