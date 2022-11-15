package pipeline

import (
	c "context"
	"encoding/json"
	"time"

	"golang.org/x/net/context"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
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

	IsRevoked bool `json:"is_revoked"`
}

type GoFormBlock struct {
	Name    string
	Title   string
	Input   map[string]string
	Output  map[string]string
	Sockets []script.Socket
	State   *FormData

	RunContext *BlockRunContext
}

func (gb *GoFormBlock) Members() map[string]struct{} {
	return gb.State.Executors
}

func (gb *GoFormBlock) CheckSLA() bool {
	return false
}

func (gb *GoFormBlock) UpdateManual() bool {
	return true
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
func createGoFormBlock(ctx context.Context, name string, ef *entity.EriusFunc, runCtx *BlockRunContext) (*GoFormBlock, error) {
	b := &GoFormBlock{
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
		if err := b.createState(ctx, ef); err != nil {
			return nil, err
		}
		b.RunContext.VarStore.AddStep(b.Name)
	}

	return b, nil
}

func (gb *GoFormBlock) loadState(raw json.RawMessage) error {
	return json.Unmarshal(raw, &gb.State)
}

func (gb *GoFormBlock) createState(ctx context.Context, ef *entity.EriusFunc) error {
	var params script.FormParams
	err := json.Unmarshal(ef.Params, &params)
	if err != nil {
		return errors.Wrap(err, "can not get form parameters")
	}

	if err = params.Validate(); err != nil {
		return errors.Wrap(err, "invalid form parameters")
	}

	gb.State = &FormData{
		Executors: map[string]struct{}{
			params.Executor: {},
		},
		SchemaId:         params.SchemaId,
		SchemaName:       params.SchemaName,
		ChangesLog:       make([]ChangesLogItem, 0),
		FormExecutorType: params.FormExecutorType,
		ApplicationBody:  map[string]interface{}{},
	}

	switch gb.State.FormExecutorType {
	case script.FormExecutorTypeUser:
		gb.State.Executors = map[string]struct{}{
			params.Executor: {},
		}
	case script.FormExecutorTypeInitiator:
		gb.State.Executors = map[string]struct{}{
			gb.RunContext.Initiator: {},
		}
	case script.FormExecutorTypeFromSchema:
		variableStorage, grabStorageErr := gb.RunContext.VarStore.GrabStorage()
		if grabStorageErr != nil {
			return err
		}

		resolvedEntities, resolveErr := resolveValuesFromVariables(
			variableStorage,
			map[string]struct{}{
				params.Executor: {},
			},
		)
		if resolveErr != nil {
			return err
		}

		gb.State.Executors = resolvedEntities
	}

	return gb.handleNotifications(ctx)
}

func (gb *GoFormBlock) handleNotifications(ctx context.Context) error {
	if !gb.RunContext.doNotifications {
		return nil
	}
	executors, executorsErr := gb.resolveExecutors(ctx)
	if executorsErr != nil {
		return executorsErr
	}

	var emails = make([]string, 0)

	for _, executor := range executors {
		email, emailErr := gb.RunContext.People.GetUserEmail(ctx, executor)
		if emailErr != nil {
			continue
		}
		emails = append(emails, email)
	}

	if len(emails) == 0 {
		return nil
	}

	return gb.RunContext.Sender.SendNotification(ctx, emails, nil,
		mail.NewRequestFormExecutionInfoTemplate(
			gb.RunContext.WorkNumber,
			gb.RunContext.WorkTitle,
			gb.RunContext.Sender.SdAddress))
}

//nolint:gocyclo //ok
func (gb *GoFormBlock) resolveExecutors(ctx c.Context) (users []string, err error) {
	const funcName = "pipeliner.block_form.resolveExecutors"
	users = make([]string, 0)

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

	executorsWithAccess, err := gb.RunContext.Storage.GetUsersWithReadWriteFormAccess(ctx, gb.RunContext.WorkNumber, gb.Name)
	if err != nil {
		return nil, errors.Wrap(err, funcName)
	}

	for _, executor := range executorsWithAccess {
		switch executor.ExecutionType {
		case entity.GroupExecution:
			if executor.BlockType == entity.ExecutionBlockType {
				sdUsers, sdErr := gb.RunContext.ServiceDesc.GetExecutorsGroup(ctx, *executor.GroupId)
				if sdErr != nil {
					return nil, errors.Wrap(sdErr, funcName)
				}
				appendUnique(executorsToString(sdUsers.People))
			}
			if executor.BlockType == entity.ApprovementBlockType {
				sdUsers, sdErr := gb.RunContext.ServiceDesc.GetApproversGroup(ctx, *executor.GroupId)
				if sdErr != nil {
					return nil, errors.Wrap(sdErr, funcName)
				}
				appendUnique(approversToString(sdUsers.People))
			}
		case entity.FromSchemaExecution:
			variables, varErr := gb.RunContext.VarStore.GrabStorage()
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
