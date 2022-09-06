package pipeline

import (
	"context"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type StringsEqual struct {
	Name          string
	FunctionName  string
	FunctionInput map[string]string
	Result        bool
	Sockets       []script.Socket
}

func (se *StringsEqual) GetStatus() Status {
	return StatusFinished
}

func (se *StringsEqual) GetTaskHumanStatus() TaskHumanStatus {
	return ""
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

func (se *StringsEqual) DebugRun(ctx context.Context, _ *stepCtx, runCtx *store.VariableStore) error {
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
		nexts, ok := script.GetNexts(se.Sockets, trueSocketID)
		if !ok {
			return nil, false
		}
		return nexts, true
	}

	nexts, ok := script.GetNexts(se.Sockets, falseSocketID)
	if !ok {
		return nil, false
	}
	return nexts, true
}

func (se *StringsEqual) Skipped(_ *store.VariableStore) []string {
	if se.Result {
		var next, ok = script.GetNexts(se.Sockets, falseSocketID)
		if !ok {
			return nil
		}

		return next
	}
	var next, ok = script.GetNexts(se.Sockets, trueSocketID)
	if !ok {
		return nil
	}

	return next
}

func (se *StringsEqual) GetState() interface{} {
	return nil
}

func (se *StringsEqual) Update(_ context.Context, _ *script.BlockUpdateData) (interface{}, error) {
	return nil, nil
}
