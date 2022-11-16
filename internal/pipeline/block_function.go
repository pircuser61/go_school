package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.opencensus.io/trace"

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

func (gb *ExecutableFunctionBlock) Members() map[string]struct{} {
	return nil
}

func (gb *ExecutableFunctionBlock) CheckSLA() (bool, time.Time) {
	return false, time.Time{}
}

func (gb *ExecutableFunctionBlock) GetStatus() Status {
	return StatusRunning
}

func (gb *ExecutableFunctionBlock) GetTaskHumanStatus() TaskHumanStatus {
	return ""
}

func (gb *ExecutableFunctionBlock) GetType() string {
	return BlockExecutableFunctionID
}

func (gb *ExecutableFunctionBlock) Inputs() map[string]string {
	return gb.Input
}

func (gb *ExecutableFunctionBlock) Outputs() map[string]string {
	return gb.Output
}

func (gb *ExecutableFunctionBlock) IsScenario() bool {
	return false
}

func (gb *ExecutableFunctionBlock) DebugRun(ctx context.Context, _ *stepCtx, runCtx *store.VariableStore) error {
	_, s := trace.StartSpan(ctx, "run_function_block")
	defer s.End()

	// TODO: call the function

	return nil
}

func (gb *ExecutableFunctionBlock) Next(_ *store.VariableStore) ([]string, bool) {
	nexts, ok := script.GetNexts(gb.Sockets, DefaultSocketID)
	if !ok {
		return nil, false
	}
	return nexts, true
}

func (gb *ExecutableFunctionBlock) Skipped(_ *store.VariableStore) []string {
	return nil
}

func (gb *ExecutableFunctionBlock) RunOnly(ctx context.Context, runCtx *store.VariableStore) (interface{}, error) {
	_, s := trace.StartSpan(ctx, "run_function_block")
	defer s.End()

	values := make(map[string]interface{})

	for ikey, gkey := range gb.Input {
		val, ok := runCtx.GetValue(gkey) // if no value - empty value
		if ok {
			values[ikey] = val
		}
	}

	url := fmt.Sprintf(gb.RunURL, gb.Name)

	b, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(b))
	if err != nil {
		return nil, err
	}

	// fixme extract "X-Request-Id" to variable
	req.Header.Set("Content-Type", "application/json")

	const timeoutMinutes = 15

	client := &http.Client{
		Timeout: timeoutMinutes * time.Minute,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if len(body) != 0 {
		result := make(map[string]interface{})
		err = json.Unmarshal(body, &result)

		if err != nil {
			return string(body), nil
		}

		return result, nil
	}

	return string(body), nil
}

func (gb *ExecutableFunctionBlock) GetState() interface{} {
	return nil
}

func (gb *ExecutableFunctionBlock) Update(_ context.Context) (interface{}, error) {
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
