package pipeline

import (
	"context"
	"errors"
	"fmt"
	"math"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"

	"go.opencensus.io/trace"
)

var (
	ErrCantGetNextStep    = errors.New("can't get next step")
	errCantCastIndexToInt = errors.New("can't cast index to int")
)

type IF struct {
	Name          string
	FunctionName  string
	FunctionInput map[string]string
	Result        bool
	OnTrue        string
	OnFalse       string
}

func (e *IF) GetType() string {
	return BlockInternalIf
}

func (e *IF) Next(runCtx *store.VariableStore) ([]string, bool) {
	r, err := runCtx.GetBoolWithInput(e.FunctionInput, "check")
	if err != nil {
		return []string{""}, false
	}

	if r {
		return []string{e.OnTrue}, true
	}

	return []string{e.OnFalse}, true
}

func (e *IF) NextSteps() []string {
	nextSteps := []string{e.OnTrue, e.OnFalse}

	return nextSteps
}

func (e *IF) Inputs() map[string]string {
	return e.FunctionInput
}

func (e *IF) Outputs() map[string]string {
	return make(map[string]string)
}

func (e *IF) IsScenario() bool {
	return false
}

func (e *IF) Run(ctx context.Context, runCtx *store.VariableStore) error {
	return e.DebugRun(ctx, runCtx)
}

func (e *IF) DebugRun(ctx context.Context, runCtx *store.VariableStore) error {
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

type StringsEqual struct {
	Name          string
	FunctionName  string
	FunctionInput map[string]string
	Result        bool
	OnTrue        string
	OnFalse       string
}

func (se *StringsEqual) GetType() string {
	return BlockInternalStringsEqual
}

func (se *StringsEqual) IsScenario() bool {
	return false
}

func (se *StringsEqual) Inputs() map[string]string {
	return se.FunctionInput
}

func (se *StringsEqual) Outputs() map[string]string {
	return make(map[string]string)
}

func (se *StringsEqual) Run(ctx context.Context, runCtx *store.VariableStore) error {
	return se.DebugRun(ctx, runCtx)
}

func (se *StringsEqual) DebugRun(ctx context.Context, runCtx *store.VariableStore) error {
	_, s := trace.StartSpan(ctx, "run_strings_equal_block")
	defer s.End()

	runCtx.AddStep(se.Name)

	allparams := make([]string, 0, len(se.FunctionInput))

	for k := range se.FunctionInput {
		r, err := runCtx.GetStringWithInput(se.FunctionInput, k)
		if err != nil {
			return err
		}

		allparams = append(allparams, r)
	}

	const minVariablesCnt = 2
	if len(allparams) >= minVariablesCnt {
		for _, v := range allparams {
			se.Result = allparams[0] == v
			if !se.Result {
				return nil
			}
		}
	}

	return nil
}

func (se *StringsEqual) Next(runCtx *store.VariableStore) ([]string, bool) {
	if se.Result {
		return []string{se.OnTrue}, true
	}

	return []string{se.OnFalse}, true
}

func (se *StringsEqual) NextSteps() []string {
	return []string{se.OnTrue, se.OnFalse}
}

type ForState struct {
	Name           string
	FunctionName   string
	FunctionInput  map[string]string
	FunctionOutput map[string]string
	OnTrue         string
	OnFalse        string
}

func (e *ForState) GetType() string {
	return BlockInternalForState
}

func (e *ForState) Inputs() map[string]string {
	return e.FunctionInput
}

func (e *ForState) Outputs() map[string]string {
	return e.FunctionOutput
}

func (e *ForState) IsScenario() bool {
	return false
}

func (e *ForState) Run(ctx context.Context, runCtx *store.VariableStore) error {
	return e.DebugRun(ctx, runCtx)
}

func (e *ForState) DebugRun(ctx context.Context, runCtx *store.VariableStore) error {
	_, s := trace.StartSpan(ctx, "run_cyclo_block")
	defer s.End()

	runCtx.AddStep(e.Name)

	arr, _ := runCtx.GetArray(e.FunctionInput["iter"])

	index := 0

	i, ok := runCtx.GetValue(e.FunctionOutput["index"])
	if ok {
		index, ok = indexToInt(i)
		if !ok {
			return errCantCastIndexToInt
		}
	}

	if index < len(arr) {
		val := fmt.Sprintf("%v", arr[index])
		index++
		runCtx.SetValue(e.FunctionOutput["index"], index)
		runCtx.SetValue(e.FunctionOutput["now_on"], val)
	} else {
		index++
		runCtx.SetValue(e.FunctionOutput["index"], index)
	}

	return nil
}

func (e *ForState) Next(runCtx *store.VariableStore) ([]string, bool) {
	arr, _ := runCtx.GetArray(e.FunctionInput["iter"])
	if len(arr) == 0 {
		return []string{e.OnTrue}, true
	}

	i, getValue := runCtx.GetValue(e.FunctionOutput["index"])
	if !getValue {
		return []string{""}, getValue
	}

	index, indexOk := indexToInt(i)
	if !indexOk {
		return []string{""}, indexOk
	}

	if index >= len(arr)+1 {
		return []string{e.OnTrue}, true
	}

	return []string{e.OnFalse}, true
}

func (e *ForState) NextSteps() []string {
	nextSteps := []string{e.OnTrue, e.OnFalse}

	return nextSteps
}

func indexToInt(i interface{}) (int, bool) {
	switch i.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		index, ok := i.(int)

		return index, ok
	case float32, float64:
		floatIndex, ok := i.(float64)
		index := int(math.Round(floatIndex))

		return index, ok
	default:
		return 0, false
	}
}
