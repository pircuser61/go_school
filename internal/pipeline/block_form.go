package pipeline

import (
	c "context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"

	"go.opencensus.io/trace"
)

const (
	keyOutputFormExecutor = "executor"
	keyOutputFormBody     = "application_body"
)

type ChangesLogItem struct {
	Description     string                 `json:"description"`
	ApplicationBody map[string]interface{} `json:"application_body"`
	CreatedAt       time.Time              `json:"created_at"`
	Executor        string                 `json:"executor,omitempty"`
}

type FormData struct {
	FormExecutorType script.FormExecutorType `json:"form_executor_type"`
	SchemaId         string                  `json:"schema_id"`
	SchemaName       string                  `json:"schema_name"`
	Executors        map[string]struct{}     `json:"executors"`
	Description      string                  `json:"description"`
	ApplicationBody  map[string]interface{}  `json:"application_body"`
	IsFilled         bool                    `json:"is_filled"`
	ActualExecutor   *string                 `json:"actual_executor,omitempty"`
	ChangesLog       []ChangesLogItem        `json:"changes_log"`

	SLA int `json:"sla"`

	DidSLANotification bool `json:"did_sla_notification"`

	LeftToNotify                map[string]struct{} `json:"left_to_notify"`
	IsExecutorVariablesResolved bool                `json:"is_executor_variables_resolved"`
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

	if state.FormExecutorType == script.FormExecutorTypeFromSchema && !state.IsExecutorVariablesResolved {
		resolveErr := gb.resolveFormExecutors(ctx, &resolveFormExecutorsDTO{runCtx: runCtx, step: step, id: id})

		if resolveErr != nil {
			return resolveErr
		}
	}

	// nolint:dupl // not dupl?
	if gb.State.IsFilled {
		var actualExecutor string

		if state.ActualExecutor != nil {
			actualExecutor = *state.ActualExecutor
		}

		runCtx.SetValue(gb.Output[keyOutputFormExecutor], actualExecutor)
		runCtx.SetValue(gb.Output[keyOutputFormBody], gb.State.ApplicationBody)

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

// nolint:dupl // another block
func createGoFormBlock(name string, ef *entity.EriusFunc, ep *ExecutablePipeline) (*GoFormBlock, error) {
	b := &GoFormBlock{
		Name:     name,
		Title:    ef.Title,
		Input:    map[string]string{},
		Output:   map[string]string{},
		Sockets:  entity.ConvertSocket(ef.Sockets),
		Pipeline: ep,
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
		Executors: map[string]struct{}{
			params.Executor: {},
		},
		SchemaId:         params.SchemaId,
		SchemaName:       params.SchemaName,
		ChangesLog:       make([]ChangesLogItem, 0),
		FormExecutorType: params.FormExecutorType,
		ApplicationBody:  map[string]interface{}{},
	}

	if b.State.FormExecutorType == script.FormExecutorTypeUser {
		b.State.Executors = map[string]struct{}{
			params.Executor: {},
		}
	}

	if b.State.FormExecutorType == script.FormExecutorTypeInitiator {
		b.State.Executors = map[string]struct{}{
			ep.PipelineModel.Author: {},
		}
	}

	return b, nil
}

type resolveFormExecutorsDTO struct {
	runCtx *store.VariableStore
	step   *entity.Step
	id     uuid.UUID
}

func (gb *GoFormBlock) resolveFormExecutors(ctx c.Context, dto *resolveFormExecutorsDTO) (err error) {
	variableStorage, grabStorageErr := dto.runCtx.GrabStorage()
	if grabStorageErr != nil {
		return err
	}

	resolvedEntities, resolveErr := resolveValuesFromVariables(variableStorage, gb.State.Executors)
	if resolveErr != nil {
		return err
	}

	gb.State.Executors = resolvedEntities

	if len(gb.State.LeftToNotify) > 0 {
		resolvedEntitiesToNotify, resolveErrToNotify := resolveValuesFromVariables(variableStorage, gb.State.LeftToNotify)
		if resolveErrToNotify != nil {
			return err
		}

		gb.State.LeftToNotify = resolvedEntitiesToNotify
	}

	gb.State.IsExecutorVariablesResolved = true

	dto.step.State[gb.Name], err = json.Marshal(gb.State)
	if err != nil {
		return err
	}

	content, err := json.Marshal(store.NewFromStep(dto.step))
	if err != nil {
		return err
	}

	return gb.Pipeline.Storage.UpdateStepContext(ctx, &db.UpdateStepRequest{
		Id:          dto.id,
		Content:     content,
		BreakPoints: dto.step.BreakPoints,
		HasError:    false,
		Status:      string(StatusFinished),
	})
}
