package pipeline

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/kafka"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type FunctionRetryPolicy string

const (
	ErrorKey     = "error"
	KeyDelimiter = "."

	SimpleFunctionRetryPolicy FunctionRetryPolicy = "simple"
)

type ExecutableFunction struct {
	Name           string               `json:"name"`
	Version        string               `json:"version"`
	Mapping        script.MappingParam  `json:"mapping"`
	Function       script.FunctionParam `json:"function"`
	Async          bool                 `json:"async"`
	HasAck         bool                 `json:"has_ack"`
	HasResponse    bool                 `json:"has_response"`
	Contracts      string               `json:"contracts"`
	WaitCorrectRes int                  `json:"waitCorrectRes"`
}

type FunctionStatus string

type FunctionUpdateParams struct {
	Mapping map[string]interface{} `json:"mapping"`
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

func (gb *ExecutableFunctionBlock) Deadlines() []Deadline {
	return []Deadline{}
}

func (gb *ExecutableFunctionBlock) GetStatus() Status {
	if gb.State.Async && gb.State.HasAck && !gb.State.HasResponse {
		return StatusIdle
	}

	if gb.State.HasResponse {
		return StatusFinished
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
	return gb.State
}

//nolint:gocyclo //its ok here
func (gb *ExecutableFunctionBlock) Update(ctx context.Context) (interface{}, error) {
	if gb.RunContext.UpdateData != nil {
		var updateDataParams FunctionUpdateParams
		updateDataUnmarshalErr := json.Unmarshal(gb.RunContext.UpdateData.Parameters, &updateDataParams)
		if updateDataUnmarshalErr != nil {
			return nil, updateDataUnmarshalErr
		}

		gb.changeCurrentState()

		if gb.State.HasResponse {
			var expectedOutput map[string]script.ParamMetadata
			outputUnmarshalErr := json.Unmarshal([]byte(gb.State.Function.Output), &expectedOutput)
			if outputUnmarshalErr != nil {
				return nil, outputUnmarshalErr
			}

			var resultOutput = make(map[string]interface{})

			for k := range expectedOutput {
				param, ok := updateDataParams.Mapping[k]
				if !ok {
					return nil, errors.New("function returned not all of expected results")
				}

				if err := utils.CheckVariableType(param, expectedOutput[k]); err != nil {
					return nil, err
				}

				resultOutput[k] = param
			}

			if len(resultOutput) != len(expectedOutput) {
				return nil, errors.New("function returned not all of expected results")
			}

			for k, v := range resultOutput {
				gb.RunContext.VarStore.SetValue(gb.Output[k], v)
			}
		}
	} else {
		taskStep, err := gb.RunContext.Storage.GetTaskStepByName(ctx, gb.RunContext.TaskID, gb.Name)
		if err != nil {
			return nil, err
		}

		executableFunctionMapping := gb.State.Mapping

		variables, err := getVariables(gb.RunContext.VarStore)
		if err != nil {
			return nil, err
		}

		functionMapping := make(map[string]interface{})

		for k := range executableFunctionMapping {
			v := executableFunctionMapping[k]
			variable := getVariable(variables, v.Value)
			if variable == nil {
				return nil, fmt.Errorf("cant fill function mapping with value: %s = %v", k, v.Value)
			}

			if checkErr := utils.CheckVariableType(variable, &v); checkErr != nil {
				return nil, checkErr
			}

			functionMapping[k] = variable
		}

		if !gb.RunContext.skipProduce {
			err = gb.RunContext.Kafka.Produce(ctx, kafka.RunnerOutMessage{
				TaskID:          taskStep.ID,
				FunctionMapping: functionMapping,
				Contracts:       gb.State.Contracts,
				RetryPolicy:     string(SimpleFunctionRetryPolicy),
				FunctionName:    gb.State.Name,
			})

			if err != nil {
				return nil, err
			}
		}
	}

	var stateBytes []byte
	stateBytes, err := json.Marshal(gb.State)
	if err != nil {
		return nil, err
	}

	gb.RunContext.VarStore.ReplaceState(gb.Name, stateBytes)

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

	rawState, ok := runCtx.VarStore.State[name]
	if ok {
		if err := b.loadState(rawState); err != nil {
			return nil, err
		}
	} else {
		if err := b.createState(ef); err != nil {
			return nil, err
		}
		b.RunContext.VarStore.AddStep(b.Name)
	}

	b.RunContext.VarStore.AddStep(b.Name)

	return b, nil
}

func (gb *ExecutableFunctionBlock) loadState(raw json.RawMessage) error {
	return json.Unmarshal(raw, &gb.State)
}

//nolint:dupl,gocyclo //its not duplicate
func (gb *ExecutableFunctionBlock) createState(ef *entity.EriusFunc) error {
	var params script.ExecutableFunctionParams
	err := json.Unmarshal(ef.Params, &params)
	if err != nil {
		return errors.Wrap(err, "can not get executable function parameters")
	}

	if err = params.Validate(); err != nil {
		return errors.Wrap(err, "invalid executable function parameters")
	}

	function, err := gb.RunContext.FunctionStore.GetFunction(context.Background(), params.Function.FunctionId)
	if err != nil {
		return err
	}

	isAsync, invalidOptionTypeErr := function.IsAsync()
	if invalidOptionTypeErr != nil {
		return invalidOptionTypeErr
	}

	gb.State = &ExecutableFunction{
		Name:           params.Name,
		Version:        params.Version,
		Mapping:        params.Mapping,
		Function:       params.Function,
		HasAck:         false,
		HasResponse:    false,
		Async:          isAsync,
		Contracts:      function.Contracts,
		WaitCorrectRes: params.WaitCorrectRes,
	}

	return nil
}

func (gb *ExecutableFunctionBlock) changeCurrentState() {
	if gb.State.Async && !gb.State.HasAck {
		gb.State.HasAck = true
		return
	}
	gb.State.HasResponse = true
}
