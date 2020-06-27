package pipeline

import (
	"context"
	"errors"
	"fmt"
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


func (e IF) Inputs() map[string]string {
	return e.FunctionInput
}

func (e IF) Outputs() map[string]string {
	return make(map[string]string)
}

func (e IF) IsScenario() bool {
	return false
}

func (e *IF) Run(ctx context.Context, runCtx *store.VariableStore, deep int) error {
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

func (fb StringsEqual) IsScenario() bool {
	return false
}

func  (fb StringsEqual)  Inputs() map[string]string {
	return fb.FunctionInput
}

func (fb StringsEqual)  Outputs() map[string]string {
	return make(map[string]string)
}



func (e *StringsEqual) Run(ctx context.Context, runCtx *store.VariableStore, deep int) error {
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
	Name           string
	FunctionName   string
	FunctionInput  map[string]string
	FunctionOutput map[string]string
	LastElem       bool
	OnTrue         string
	OnFalse        string
}


func (e ForState) Inputs() map[string]string {
	return e.FunctionInput
}

func (e ForState) Outputs() map[string]string {
	return e.FunctionOutput
}

func (e ForState) IsScenario() bool {
	return false
}

func (e *ForState) Run(ctx context.Context, runCtx *store.VariableStore, deep int) error {
	_, s := trace.StartSpan(ctx, "run_cyclo_block")
	defer s.End()
	runCtx.AddStep(e.Name)
	arr, ok := runCtx.GetArray(e.FunctionInput["iter"])

	index := 0
	i, ok := runCtx.GetValue(e.FunctionOutput["index"])
	if ok {
		index, ok = i.(int)
		if !ok {
			return errors.New("can't get index")
		}
	}
	fmt.Println(len(arr), index, len(arr) > index, len(arr) < index, len(arr) == index)
	if index < len(arr) {
		fmt.Println(arr[index])
		val := fmt.Sprintf("%v", arr[index])
		index++
		runCtx.SetValue(e.FunctionOutput["index"], index)
		runCtx.SetValue(e.FunctionOutput["now_on"], val)
	} else {
		e.LastElem = true
	}
	return nil
}

func (e *ForState) Next() string {
	if e.LastElem {
		return e.OnTrue
	}
	return e.OnFalse
}
