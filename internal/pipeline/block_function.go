package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/kafka"
	"time"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
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
	Name    string              `json:"name"`
	Version string              `json:"version"`
	Mapping script.MappingParam `json:"mapping"`
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
	// if UpdateData is nil than block is new
	if gb.RunContext.UpdateData == nil {
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

		for k, v := range executableFunctionMapping {
			variable := getVariable(variables, v.Value)
			if variable == nil {
				return nil, fmt.Errorf("cant fill function mapping with value: k: %s", k)
			}
			functionMapping[k] = getVariable(variables, v.Value) // TODO надо будет проверять типы, а также нам нужна будет работа с обьектами
		}

		err = gb.RunContext.Kafka.Produce(ctx, kafka.RunnerOutMessage{
			TaskID:          taskStep.ID,
			FunctionMapping: functionMapping,
			RetryPolicy:     string(SimpleFunctionRetryPolicy),
			FunctionName:    gb.State.Name,
		})

		if err != nil {
			return nil, err
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
		Name:    params.Name,
		Version: params.Version,
		Mapping: params.Mapping,
	}

	return b, nil
}
