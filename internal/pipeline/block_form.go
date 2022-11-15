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
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc"
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

	DidSLANotification  bool `json:"did_sla_notification"`
	DidFillNotification bool `json:"did_fill_notification"`

	LeftToNotify                map[string]struct{} `json:"left_to_notify"`
	IsExecutorVariablesResolved bool                `json:"is_executor_variables_resolved"`

	IsRevoked bool `json:"is_revoked"`
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
	if gb.State != nil && gb.State.IsRevoked {
		return StatusCancel
	}
	if gb.State != nil && gb.State.IsFilled {
		return StatusFinished
	}

	return StatusRunning
}

func (gb *GoFormBlock) GetTaskHumanStatus() TaskHumanStatus {
	if gb.State != nil && gb.State.IsRevoked {
		return StatusRevoke
	}
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

	_, err = gb.handleNotifications(ctx, stepCtx, runCtx, id)
	if err != nil {
		l.WithError(err).Error("couldn't handle notifications")
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
				Type:    "object",
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
		resolvedEntitiesToNotify, resolveErrToNotify :=
			resolveValuesFromVariables(variableStorage, gb.State.LeftToNotify)
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
		Members:     gb.State.Executors,
	})
}

func (gb *GoFormBlock) handleNotifications(
	ctx c.Context,
	stepCtx *stepCtx,
	runCtx *store.VariableStore,
	id uuid.UUID) (ok bool, err error) {
	if !gb.State.DidFillNotification {
		l := logger.GetLogger(ctx)

		executors, executorsErr := gb.resolveExecutors(ctx, runCtx, stepCtx.workNumber)
		if executorsErr != nil {
			return false, executorsErr
		}

		var emails = make([]string, 0)

		for _, executor := range executors {
			email, emailErr := gb.Pipeline.People.GetUserEmail(ctx, executor)
			if emailErr != nil {
				l.WithError(emailErr).Error("couldn't get email")
			}
			emails = append(emails, email)
		}

		if len(emails) == 0 {
			return false, nil
		}

		err = gb.Pipeline.Sender.SendNotification(ctx, emails, nil,
			mail.NewRequestFormExecutionInfoTemplate(
				stepCtx.workNumber,
				stepCtx.workTitle,
				gb.Pipeline.Sender.SdAddress))
		if err != nil {
			return false, err
		}
	}

	gb.State.DidFillNotification = true

	err = gb.dumpCurrState(ctx, id)
	if err != nil {
		return false, err
	}

	return true, nil
}

//nolint:gocyclo //ok
func (gb *GoFormBlock) resolveExecutors(ctx c.Context, runCtx *store.VariableStore, workNumber string) ([]string, error) {
	const funcName = "pipeliner.block_form.resolveExecutors"
	users := make([]string, 0)

	var exists = func(entry string) bool {
		for _, user := range users {
			if user == entry {
				return true
			}
		}
		return false
	}

	var appendUnique = func(usersToAppend []string) {
		for _, user := range usersToAppend {
			if !exists(user) && user != "" {
				users = append(users, user)
			}
		}
	}

	appendUnique(mapToString(gb.State.Executors))

	executorsWithAccess, err := gb.Pipeline.Storage.GetUsersWithReadWriteFormAccess(ctx, workNumber, gb.Name)
	if err != nil {
		return nil, errors.Wrap(err, funcName)
	}

	for _, executor := range executorsWithAccess {
		switch executor.ExecutionType {
		case entity.GroupExecution:
			if executor.BlockType == entity.ExecutionBlockType && executor.GroupId != nil {
				sdUsers, sdErr := gb.Pipeline.ServiceDesc.GetExecutorsGroup(ctx, *executor.GroupId)
				if sdErr != nil {
					return nil, errors.Wrap(sdErr, funcName)
				}
				appendUnique(executorsToString(sdUsers.People))
			}
			if executor.BlockType == entity.ApprovementBlockType && executor.GroupId != nil {
				sdUsers, sdErr := gb.Pipeline.ServiceDesc.GetApproversGroup(ctx, *executor.GroupId)
				if sdErr != nil {
					return nil, errors.Wrap(sdErr, funcName)
				}
				appendUnique(approversToString(sdUsers.People))
			}
		case entity.FromSchemaExecution:
			variables, varErr := runCtx.GrabStorage()
			if varErr != nil {
				return nil, errors.Wrap(varErr, funcName)
			}

			var toResolve = map[string]struct{}{
				executor.Executor: {},
			}

			schemaUsers, resolveErr := resolveValuesFromVariables(variables, toResolve)
			if resolveErr != nil {
				return nil, errors.Wrap(resolveErr, funcName)
			}
			appendUnique(mapToString(schemaUsers))
		case entity.UserExecution:
			appendUnique([]string{executor.Executor})
		default:
			return nil, errors.New("invalid execution type from database")
		}
	}

	return users, nil
}

func executorsToString(executors []servicedesc.Executor) []string {
	var res = make([]string, len(executors))
	for _, executor := range executors {
		res = append(res, executor.Login)
	}
	return res
}

func approversToString(approvers []servicedesc.Approver) []string {
	var res = make([]string, len(approvers))
	for _, approver := range approvers {
		res = append(res, approver.Login)
	}
	return res
}

func mapToString(schemaUsers map[string]struct{}) []string {
	var res = make([]string, len(schemaUsers))
	for userKey := range schemaUsers {
		res = append(res, userKey)
	}
	return res
}

//nolint:dupl // different block
func (gb *GoFormBlock) dumpCurrState(ctx c.Context, id uuid.UUID) error {
	step, err := gb.Pipeline.Storage.GetTaskStepById(ctx, id)
	if err != nil {
		return err
	}

	step.State[gb.Name], err = json.Marshal(gb.State)
	if err != nil {
		return err
	}

	content, err := json.Marshal(store.NewFromStep(step))
	if err != nil {
		return err
	}

	return gb.Pipeline.Storage.UpdateStepContext(ctx, &db.UpdateStepRequest{
		Id:          id,
		Content:     content,
		BreakPoints: step.BreakPoints,
		HasError:    false,
		Status:      string(StatusFinished),
		Members:     gb.State.Executors,
	})
}
