package pipeline

import (
	"context"
	"fmt"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type ForState struct {
	Name           string
	FunctionName   string
	FunctionInput  map[string]string
	FunctionOutput map[string]string
	Sockets        []script.Socket
}

func (e *ForState) GetStatus() Status {
	return StatusFinished
}

func (e *ForState) GetTaskHumanStatus() TaskHumanStatus {
	return ""
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

func (e *ForState) DebugRun(ctx context.Context, stepCtx *stepCtx, runCtx *store.VariableStore) error {
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
		nexts, ok := script.GetNexts(e.Sockets, trueSocketID)
		if !ok {
			return nil, false
		}
		return nexts, true
	}

	i, getValue := runCtx.GetValue(e.FunctionOutput["index"])
	if !getValue {
		return []string{}, getValue
	}

	index, indexOk := indexToInt(i)
	if !indexOk {
		return []string{}, indexOk
	}

	if index >= len(arr)+1 {
		nexts, ok := script.GetNexts(e.Sockets, trueSocketID)
		if !ok {
			return nil, false
		}
		return nexts, true
	}

	nexts, ok := script.GetNexts(e.Sockets, falseSocketID)
	if !ok {
		return nil, false
	}
	return nexts, true
}

func (e *ForState) Skipped(_ *store.VariableStore) []string {
	return nil
}

func (e *ForState) GetState() interface{} {
	return nil
}

func (e *ForState) Update(_ context.Context, _ *script.BlockUpdateData) (interface{}, error) {
	return nil, nil
}
