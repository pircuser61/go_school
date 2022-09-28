package pipeline

import (
	c "context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"golang.org/x/net/context"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"

	"go.opencensus.io/trace"
)

const (
	keyOutputFormExecutor = "executor"
	keyOutputFormBody     = "form_body"
)

type FormData struct {
	FormId         string                 `json:"form_id"`
	FormName       string                 `json:"form_name"`
	Type           script.ApproverType    `json:"type"`
	Executors      map[string]struct{}    `json:"executors"`
	FormBody       map[string]interface{} `json:"form_body"`
	IsFilled       bool                   `json:"is_filled"`
	ActualExecutor *string                `json:"actual_executor,omitempty"`

	SLA int `json:"sla"`

	DidSLANotification bool `json:"did_sla_notification"`

	LeftToNotify map[string]struct{} `json:"left_to_notify"`
}

type GoFormBlock struct {
	Name    string
	Title   string
	Input   map[string]string
	Output  map[string]string
	Sockets []script.Socket
	State   *FormData

	Pipeline *ExecutablePipeline
}

func (gb *GoFormBlock) GetStatus() Status {
	if gb.State != nil && gb.State.IsFilled {
		return StatusFinished
	}

	return StatusRunning
}

func (gb *GoFormBlock) GetTaskHumanStatus() TaskHumanStatus {
	if gb.State != nil && gb.State.IsFilled {
		return StatusDone
	}

	return StatusExecution
}

func (gb *GoFormBlock) GetType() string {
	return BlockGoFormID
}

func (gb *GoFormBlock) Inputs() map[string]string {
	return gb.Input
}

func (gb *GoFormBlock) Outputs() map[string]string {
	return gb.Output
}

func (gb *GoFormBlock) IsScenario() bool {
	return false
}

func (gb *GoFormBlock) GetState() interface{} {
	return gb.State
}

func (gb *GoFormBlock) Skipped(_ *store.VariableStore) []string {
	return nil
}

func (gb *GoFormBlock) Next(_ *store.VariableStore) ([]string, bool) {
	nexts, ok := script.GetNexts(gb.Sockets, DefaultSocketID)
	if !ok {
		return nil, false
	}
	return nexts, true
}

//nolint:gocyclo //ok
func (gb *GoFormBlock) DebugRun(ctx c.Context, stepCtx *stepCtx, runCtx *store.VariableStore) (err error) {
	ctx, s := trace.StartSpan(ctx, "run_go_form_block")
	defer s.End()

	l := logger.GetLogger(ctx)

	val, isOk := runCtx.GetValue(getWorkIdKey(gb.Name))
	if !isOk {
		return errors.New("can't get work id from variable store")
	}

	id, isOk := val.(uuid.UUID)
	if !isOk {
		return errors.New("can't assert type of work id")
	}

	// check state from database
	var step *entity.Step
	step, err = gb.Pipeline.Storage.GetTaskStepById(ctx, id)
	if err != nil {
		return err
	} else if step == nil {
		l.Error(err)
		return nil
	}

	// get state from step.State
	data, ok := step.State[gb.Name]
	if !ok {
		return fmt.Errorf("key %s is not found in state of go-form-block", gb.Name)
	}

	var state FormData
	err = json.Unmarshal(data, &state)
	if err != nil {
		return errors.Wrap(err, "invalid format of go-form-block state")
	}

	gb.State = &state

	// nolint:dupl // not dupl?
	if gb.State.IsFilled {
		var actualExecutor string

		if state.ActualExecutor != nil {
			actualExecutor = *state.ActualExecutor
		}

		runCtx.SetValue(gb.Output[keyOutputFormExecutor], actualExecutor)
		runCtx.SetValue(gb.Output[keyOutputFormBody], gb.State.FormBody)

		var stateBytes []byte
		stateBytes, err = json.Marshal(gb.State)
		if err != nil {
			return err
		}

		runCtx.ReplaceState(gb.Name, stateBytes)
	}

	return nil
}

func (gb *GoFormBlock) Model() script.FunctionModel {
	return script.FunctionModel{
		ID:        BlockGoFormID,
		BlockType: script.TypeGo,
		Title:     gb.Title,
		Inputs:    nil,
		Outputs: []script.FunctionValueModel{
			{
				Name:    keyOutputFormExecutor,
				Type:    "string",
				Comment: "form executor login",
			},
			{
				Name:    keyOutputFormBody,
				Type:    "string",
				Comment: "form body",
			},
		},
		Params: &script.FunctionParams{
			Type:   BlockGoFormID,
			Params: &script.FormParams{},
		},
		Sockets: []script.Socket{script.DefaultSocket},
	}
}

func (gb *GoFormBlock) Update(_ context.Context, _ *script.BlockUpdateData) (interface{}, error) {
	return nil, nil
}

// nolint:dupl // another block
func createGoFormBlock(name string, ef *entity.EriusFunc) (*GoFormBlock, error) {
	b := &GoFormBlock{
		Name:    name,
		Title:   ef.Title,
		Input:   map[string]string{},
		Output:  map[string]string{},
		Sockets: entity.ConvertSocket(ef.Sockets),
	}

	for _, v := range ef.Input {
		b.Input[v.Name] = v.Global
	}

	for _, v := range ef.Output {
		b.Output[v.Name] = v.Global
	}

	var params script.FormParams
	err := json.Unmarshal(ef.Params, &params)
	if err != nil {
		return nil, errors.Wrap(err, "can not get form parameters")
	}

	if err = params.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid form parameters")
	}

	b.State = &FormData{
		FormId:   params.FormId,
		FormName: params.FormName,
	}

	return b, nil
}
