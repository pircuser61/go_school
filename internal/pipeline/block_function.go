package pipeline

import (
	"context"
	"encoding/json"
	"time"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

const (
	ErrorKey     = "error"
	KeyDelimiter = "."
)

type ExecutableFunction struct {
	Name             string              `json:"name"`
	Version          string              `json:"version"`
	Mapping          script.MappingParam `json:"mapping"`
	Async            bool                `json:"async"`
	ResponseReceived bool                `json:"response_received"`
}

type ExecutableFunctionBlock struct {
	Name    string
	Title   string
	Input   map[string]string
	Output  map[string]string
	Sockets []script.Socket
	State   *ExecutableFunction
	RunURL  string

	RunContext *BlockRunContext
}

func (gb *ExecutableFunctionBlock) Members() []Member {
	return nil
}

func (gb *ExecutableFunctionBlock) CheckSLA() (bool, bool, time.Time) {
	return false, false, time.Time{}
}

func (gb *ExecutableFunctionBlock) GetStatus() Status {
	if gb.State.ResponseReceived {
		return StatusFinished
	}

	if !gb.State.ResponseReceived && gb.State.Async {
		return StatusIdle
	}

	return StatusRunning
}

func (gb *ExecutableFunctionBlock) GetTaskHumanStatus() TaskHumanStatus {
	return ""
}

func (gb *ExecutableFunctionBlock) Next(_ *store.VariableStore) ([]string, bool) {
	nexts, ok := script.GetNexts(gb.Sockets, DefaultSocketID)
	if !ok {
		return nil, false
	}
	return nexts, true
}

func (gb *ExecutableFunctionBlock) GetState() interface{} {
	return nil
}

func (gb *ExecutableFunctionBlock) Update(ctx context.Context) (interface{}, error) {
	function, err := gb.RunContext.FunctionStore.GetFunction(ctx, gb.Name)
	if err != nil {
		return nil, err
	}

	var valNotExistsErr error
	gb.State.Async, valNotExistsErr = function.GetOptionAsBool("async")
	if valNotExistsErr != nil {
		return nil, nil
	}

	// 0.
	// 1. consume
	// 2. if async
	// 2.1 response is intermediate -> responseReceived = false
	// 2.2. response is final -> responseReceived = true
	// 3. if sync
	// 3. response is final -> responseReceived = true
	// 4. check map from consumer message
	// 5. compare with expected fields
	// 6. set outputs if ok

	if gb.State.Async {
		// check if response is intermediate
		gb.State.ResponseReceived = false
		// if not
		//gb.State.ResponseReceived = true
	} else {
		gb.State.ResponseReceived = true
	}

	var expectedOutput = make(map[string]interface{})
	var outputFromFunction = make(map[string]interface{})

	var keyExist = func(entry string, m map[string]interface{}) bool {
		for k := range m {
			if k == entry {
				return true
			}
		}
		return false
	}

	var resultOutput = make(map[string]interface{})

	for k, v := range expectedOutput {
		if keyExist(k, outputFromFunction) {
			resultOutput[k] = v
		}
	}

	if len(resultOutput) != len(outputFromFunction) {
		return nil, errors.New("function returned not all of expected results")
	}

	for k, v := range resultOutput {
		gb.RunContext.VarStore.SetValue(gb.Output[k], v)
	}

	// todo: validate for certain type (JAP-903)
	return nil, nil
}

func (gb *ExecutableFunctionBlock) Model() script.FunctionModel {
	return script.FunctionModel{
		ID:        BlockExecutableFunctionID,
		BlockType: script.TypeExternal,
		Title:     BlockExecutableFunctionTitle,
		Inputs:    nil,
		Outputs:   nil,
		Params: &script.FunctionParams{
			Type: BlockExecutableFunctionID,
			Params: &script.ExecutableFunctionParams{
				Name:    "",
				Version: "",
				Mapping: script.MappingParam{},
			},
		},
		Sockets: []script.Socket{script.DefaultSocket},
	}
}

func (gb *ExecutableFunctionBlock) UpdateManual() bool {
	return false
}

// nolint:dupl // another block
func createExecutableFunctionBlock(name string, ef *entity.EriusFunc, runCtx *BlockRunContext) (*ExecutableFunctionBlock, error) {
	b := &ExecutableFunctionBlock{
		Name:       name,
		Title:      ef.Title,
		Input:      map[string]string{},
		Output:     map[string]string{},
		Sockets:    entity.ConvertSocket(ef.Sockets),
		RunContext: runCtx,
	}

	for _, v := range ef.Input {
		b.Input[v.Name] = v.Global
	}

	for _, v := range ef.Output {
		b.Output[v.Name] = v.Global
	}

	var params script.ExecutableFunctionParams
	err := json.Unmarshal(ef.Params, &params)
	if err != nil {
		return nil, errors.Wrap(err, "can not get executable function parameters")
	}

	if err = params.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid executable function parameters")
	}

	b.State = &ExecutableFunction{
		Name:    params.Name,
		Version: params.Version,
		Mapping: params.Mapping,
	}

	return b, nil
}
