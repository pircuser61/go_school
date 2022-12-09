package pipeline

import (
	"context"
	"encoding/json"
	"strconv"
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
	Name           string               `json:"name"`
	Version        string               `json:"version"`
	Mapping        script.MappingParam  `json:"mapping"`
	Function       script.FunctionParam `json:"function"`
	Async          bool                 `json:"async"`
	FunctionStatus FunctionStatus       `json:"function_status"`
}

type FunctionStatus string

const (
	WaitingForAnyResponse        FunctionStatus = "waiting_for_any_response"
	IntermediateResponseReceived FunctionStatus = "intermediate_response_received"
	FinalResponseReceived        FunctionStatus = "final_response_received"
)

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
	switch gb.State.FunctionStatus {
	case IntermediateResponseReceived:
		return StatusIdle
	case FinalResponseReceived:
		return StatusFinished
	default:
		return StatusRunning
	}
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
	err := gb.computeCurrentFunctionStatus(ctx)
	if err != nil {
		return nil, err
	}

	if gb.State.FunctionStatus == FinalResponseReceived {
		var expectedOutput map[string]script.ParamMetadata
		unescapedOutputStr, unquoteErr := strconv.Unquote(gb.State.Function.Output)
		if unquoteErr != nil {
			return nil, unquoteErr
		}
		err = json.Unmarshal([]byte(unescapedOutputStr), &expectedOutput)

		var outputData map[string]interface{}
		err = json.Unmarshal(gb.RunContext.UpdateData.Parameters, &outputData)
		if err != nil {
			return nil, err
		}

		var keyExist = func(entry string, m map[string]interface{}) bool {
			for k := range m {
				if k == entry {
					return true
				}
			}
			return false
		}

		var resultOutput = make(map[string]interface{})

		for k := range expectedOutput {
			if keyExist(k, outputData) {
				var val = outputData[k]
				// todo: конвертируем в нужный тип на основе метаданных (JAP-903)
				resultOutput[k] = val
			}
		}

		if len(resultOutput) != len(expectedOutput) {
			return nil, errors.New("function returned not all of expected results")
		}

		for k, v := range resultOutput {
			gb.RunContext.VarStore.SetValue(gb.Output[k], v)
		}
	}

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
		Name:           params.Name,
		Version:        params.Version,
		Mapping:        params.Mapping,
		Function:       params.Function,
		FunctionStatus: WaitingForAnyResponse,
	}

	return b, nil
}

func (gb *ExecutableFunctionBlock) computeCurrentFunctionStatus(ctx context.Context) error {
	function, err := gb.RunContext.FunctionStore.GetFunction(ctx, gb.Name)
	if err != nil {
		return err
	}

	var invalidOptionTypeErr error
	gb.State.Async, invalidOptionTypeErr = function.IsAsync()
	if invalidOptionTypeErr != nil {
		return invalidOptionTypeErr
	}

	if gb.State.Async {
		if gb.State.FunctionStatus == IntermediateResponseReceived {
			gb.State.FunctionStatus = FinalResponseReceived
		}

		if gb.State.FunctionStatus == WaitingForAnyResponse {
			gb.State.FunctionStatus = IntermediateResponseReceived
		}
	} else {
		gb.State.FunctionStatus = FinalResponseReceived
	}

	return nil
}
