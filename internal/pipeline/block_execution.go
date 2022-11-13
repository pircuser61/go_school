package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/pkg/errors"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

const (
	keyOutputExecutionType     = "type"
	keyOutputExecutionLogin    = "login"
	keyOutputExecutionDecision = "decision"
	keyOutputExecutionComment  = "comment"

	ExecutionDecisionExecuted ExecutionDecision = "executed"
	ExecutionDecisionRejected ExecutionDecision = "rejected"

	RequestInfoQuestion RequestInfoType = "question"
	RequestInfoAnswer   RequestInfoType = "answer"
)

type GoExecutionBlock struct {
	Name    string
	Title   string
	Input   map[string]string
	Output  map[string]string
	Sockets []script.Socket
	State   *ExecutionData

	RunContext *BlockRunContext
}

func (gb *GoExecutionBlock) UpdateManual() bool {
	return true
}

// nolint:dupl // another block
func (gb *GoExecutionBlock) GetTaskHumanStatus() TaskHumanStatus {
	if gb.State != nil && gb.State.IsRevoked {
		return StatusRevoke
	}

	if gb.State != nil && gb.State.Decision != nil {
		if *gb.State.Decision == ExecutionDecisionExecuted {
			return StatusDone
		}
		return StatusExecutionRejected
	}

	if gb.State.EditingApp != nil {
		return StatusWait
	}

	if len(gb.State.RequestExecutionInfoLogs) > 0 &&
		gb.State.RequestExecutionInfoLogs[len(gb.State.RequestExecutionInfoLogs)-1].ReqType == RequestInfoQuestion {
		return StatusWait
	}

	return StatusExecution
}

// nolint:dupl // another block
func (gb *GoExecutionBlock) GetStatus() Status {
	if gb.State != nil && gb.State.IsRevoked {
		return StatusCancel
	}

	if gb.State != nil && gb.State.Decision != nil {
		if *gb.State.Decision == ExecutionDecisionExecuted {
			return StatusFinished
		}
		return StatusNoSuccess
	}

	if gb.State.EditingApp != nil {
		return StatusIdle
	}

	if len(gb.State.RequestExecutionInfoLogs) > 0 &&
		gb.State.RequestExecutionInfoLogs[len(gb.State.RequestExecutionInfoLogs)-1].ReqType == RequestInfoQuestion {
		return StatusIdle
	}

	return StatusRunning
}

func (gb *GoExecutionBlock) GetType() string {
	return BlockGoExecutionID
}

func (gb *GoExecutionBlock) Inputs() map[string]string {
	return gb.Input
}

func (gb *GoExecutionBlock) Outputs() map[string]string {
	return gb.Output
}

func (gb *GoExecutionBlock) IsScenario() bool {
	return false
}

// nolint:dupl // other block
func (gb *GoExecutionBlock) dumpCurrState(ctx context.Context, id uuid.UUID) error {
	step, err := gb.RunContext.Storage.GetTaskStepById(ctx, id)
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

	return gb.RunContext.Storage.UpdateStepContext(ctx, &db.UpdateStepRequest{
		Id:          id,
		Content:     content,
		BreakPoints: step.BreakPoints,
		HasError:    false,
		Status:      string(StatusFinished),
	})
}

//nolint:dupl // maybe later
func (gb *GoExecutionBlock) handleNotifications(ctx context.Context, id uuid.UUID, stepCtx *stepCtx) (bool, error) {
	if len(gb.State.LeftToNotify) == 0 {
		return false, nil
	}
	l := logger.GetLogger(ctx)

	emails := make([]string, 0, len(gb.State.Executors))
	for executor := range gb.State.Executors {
		email, err := gb.RunContext.People.GetUserEmail(ctx, executor)
		if err != nil {
			l.WithError(err).Error("couldn't get email")
		}
		emails = append(emails, email)
	}
	if len(emails) == 0 {
		return false, nil
	}
	//descr := gb.RunContext.currDescription TODO
	descr := ""
	additionalDescriptions, err := gb.RunContext.Storage.GetAdditionalForms(gb.RunContext.WorkNumber, gb.Name)
	if err != nil {
		return false, err
	}
	for _, item := range additionalDescriptions {
		if item == "" {
			continue
		}
		descr = fmt.Sprintf("%s\n\n%s", descr, item)
	}
	err = gb.RunContext.Sender.SendNotification(ctx, emails, nil,
		mail.NewApplicationPersonStatusNotification(
			stepCtx.workNumber,
			stepCtx.workTitle,
			statusToTaskAction[StatusExecution],
			ComputeDeadline(stepCtx.stepStart, gb.State.SLA),
			descr,
			gb.RunContext.Sender.SdAddress))
	if err != nil {
		return false, err
	}

	left := gb.State.LeftToNotify
	gb.State.LeftToNotify = map[string]struct{}{}

	if err = gb.dumpCurrState(ctx, id); err != nil {
		gb.State.LeftToNotify = left
		return false, err
	}
	return true, nil
}

func (gb *GoExecutionBlock) handleSLA(ctx context.Context, id uuid.UUID, stepCtx *stepCtx) (bool, error) {
	if gb.State.DidSLANotification {
		return false, nil
	}
	if CheckBreachSLA(stepCtx.stepStart, time.Now(), gb.State.SLA) {
		l := logger.GetLogger(ctx)

		// nolint:dupl // handle executors
		if gb.State.SLA > 8 {
			emails := make([]string, 0, len(gb.State.Executors))
			for executor := range gb.State.Executors {
				email, err := gb.RunContext.People.GetUserEmail(ctx, executor)
				if err != nil {
					l.WithError(err).Error("couldn't get email")
				}
				emails = append(emails, email)
			}
			if len(emails) == 0 {
				return false, nil
			}
			err := gb.RunContext.Sender.SendNotification(ctx, emails, nil,
				mail.NewExecutionSLATemplate(stepCtx.workNumber, stepCtx.workTitle, gb.RunContext.Sender.SdAddress))
			if err != nil {
				return false, err
			}
		}

		gb.State.DidSLANotification = true

		if err := gb.dumpCurrState(ctx, id); err != nil {
			gb.State.DidSLANotification = false
			return false, err
		}

		return true, nil
	}

	return false, nil
}

//nolint:gocyclo // later
func (gb *GoExecutionBlock) DebugRun(ctx context.Context, stepCtx *stepCtx, runCtx *store.VariableStore) (err error) {
	_, s := trace.StartSpan(ctx, "run_go_execution_block")
	defer s.End()

	// TODO: fix
	// runCtx.AddStep(gb.Name)

	l := logger.GetLogger(ctx)

	val, isOk := runCtx.GetValue(getWorkIdKey(gb.Name))
	if !isOk {
		return errors.New("can't get work id from variable store")
	}

	id, isOk := val.(uuid.UUID)
	if !isOk {
		return errors.New("can't assert type of work id")
	}

	var step *entity.Step
	step, err = gb.RunContext.Storage.GetTaskStepById(ctx, id)
	if err != nil {
		return err
	} else if step == nil {
		// still waiting
		return nil
	}

	data, ok := step.State[gb.Name]
	if !ok {
		return nil
	}

	var state ExecutionData
	if err = json.Unmarshal(data, &state); err != nil {
		return errors.Wrap(err, "invalid format of go-execution-block state")
	}

	gb.State = &state

	if step.Status != string(StatusIdle) {
		handled, handleErr := gb.handleSLA(ctx, id, stepCtx)
		if handleErr != nil {
			l.WithError(handleErr).Error("couldn't handle sla")
		}
		if handled {
			// go for another loop cause we may have updated the state at db
			return gb.DebugRun(ctx, stepCtx, runCtx)
		}
	}

	handled, err := gb.handleNotifications(ctx, id, stepCtx)
	if err != nil {
		l.WithError(err).Error("couldn't handle notifications")
	}
	if handled {
		// go for another loop cause we may have updated the state at db
		return gb.DebugRun(ctx, stepCtx, runCtx)
	}

	if state.ExecutionType == script.ExecutionTypeFromSchema && !state.IsExecutorVariablesResolved {
		if resolveErr := gb.resolveExecutors(ctx, &resolveExecutorsDTO{runCtx: runCtx, id: id}, step); resolveErr != nil {
			return resolveErr
		}
	}

	decision := gb.State.GetDecision()

	// nolint:dupl // not dupl
	if decision == nil && len(gb.State.EditingAppLog) == 0 && gb.State.GetIsEditable() {
		gb.setEditingAppLogFromPreviousBlock(ctx, &setEditingAppLogDTO{
			step:     step,
			id:       id,
			runCtx:   runCtx,
			workID:   gb.RunContext.TaskID,
			stepName: step.Name,
		})
	}

	if decision == nil && gb.State.GetRepeatPrevDecision() {
		if gb.trySetPreviousDecision(ctx, &getPreviousDecisionDTO{
			step:     step,
			id:       id,
			runCtx:   runCtx,
			workID:   gb.RunContext.TaskID,
			stepName: step.Name,
		}) {
			return nil
		}
	}

	// nolint:dupl // not dupl
	if decision != nil {
		var executor, comment string

		if state.ActualExecutor != nil {
			executor = *state.ActualExecutor
		}

		if state.DecisionComment != nil {
			comment = *state.DecisionComment
		}

		runCtx.SetValue(gb.Output[keyOutputExecutionLogin], executor)
		runCtx.SetValue(gb.Output[keyOutputExecutionDecision], decision.String())
		runCtx.SetValue(gb.Output[keyOutputExecutionComment], comment)

		var stateBytes []byte
		stateBytes, err = json.Marshal(gb.State)
		if err != nil {
			return err
		}

		runCtx.ReplaceState(gb.Name, stateBytes)
	}

	return err
}

func (gb *GoExecutionBlock) Next(_ *store.VariableStore) ([]string, bool) {
	key := notExecutedSocketID
	if gb.State != nil && gb.State.Decision != nil && *gb.State.Decision == ExecutionDecisionExecuted {
		key = executedSocketID
	}

	if gb.State != nil && gb.State.Decision == nil && gb.State.EditingApp != nil {
		key = editAppSocketID
	}

	nexts, ok := script.GetNexts(gb.Sockets, key)
	if !ok {
		return nil, false
	}
	return nexts, true
}

func (gb *GoExecutionBlock) Skipped(_ *store.VariableStore) []string {
	key := executedSocketID
	if gb.State != nil && gb.State.Decision != nil && *gb.State.Decision == ExecutionDecisionExecuted {
		key = notExecutedSocketID
	}
	var next, ok = script.GetNexts(gb.Sockets, key)
	if !ok {
		return nil
	}

	return next
}

func (gb *GoExecutionBlock) GetState() interface{} {
	return gb.State
}

func (gb *GoExecutionBlock) Model() script.FunctionModel {
	return script.FunctionModel{
		ID:        BlockGoExecutionID,
		BlockType: script.TypeGo,
		Title:     gb.Title,
		Inputs:    nil,
		Outputs: []script.FunctionValueModel{
			{
				Name:    keyOutputExecutionType,
				Type:    "string",
				Comment: "execution type (user, group)",
			},
			{
				Name:    keyOutputExecutionLogin,
				Type:    "string",
				Comment: "executor login",
			},
			{
				Name:    keyOutputExecutionDecision,
				Type:    "string",
				Comment: "execution status",
			},
			{
				Name:    keyOutputExecutionComment,
				Type:    "string",
				Comment: "execution status comment",
			},
		},
		Params: &script.FunctionParams{
			Type: BlockGoExecutionID,
			Params: &script.ExecutionParams{
				Executors:          "",
				Type:               "",
				SLA:                0,
				FormsAccessibility: []script.FormAccessibility{},
			},
		},
		Sockets: []script.Socket{
			script.ExecutedSocket,
			script.NotExecutedSocket,
			script.EditAppSocket,
		},
	}
}

// nolint:dupl // another block
func createGoExecutionBlock(ctx context.Context, name string, ef *entity.EriusFunc, runCtx *BlockRunContext) (*GoExecutionBlock, error) {
	b := &GoExecutionBlock{
		Name:    name,
		Title:   ef.Title,
		Input:   map[string]string{},
		Output:  map[string]string{},
		Sockets: entity.ConvertSocket(ef.Sockets),

		RunContext: runCtx,
	}

	for _, v := range ef.Input {
		b.Input[v.Name] = v.Global
	}

	for _, v := range ef.Output {
		b.Output[v.Name] = v.Global
	}

	var params script.ExecutionParams
	err := json.Unmarshal(ef.Params, &params)
	if err != nil {
		return nil, errors.Wrap(err, "can not get execution parameters")
	}

	if err = params.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid execution parameters, work number")
	}

	executors := map[string]struct{}{
		params.Executors: {},
	}

	executorsGroupName := ""

	if params.Type == script.ExecutionTypeGroup {
		executorsGroup, errGroup := runCtx.ServiceDesc.GetExecutorsGroup(ctx, params.ExecutorsGroupID)
		if errGroup != nil {
			return nil, errors.Wrap(errGroup, "can`t get executors group with id: "+params.ExecutorsGroupID)
		}

		if len(executorsGroup.People) == 0 {
			return nil, errors.Wrap(errGroup, "zero executors in group: "+params.ExecutorsGroupID)
		}

		executorsGroupName = executorsGroup.GroupName

		executors = make(map[string]struct{})
		for i := range executorsGroup.People {
			executors[executorsGroup.People[i].Login] = struct{}{}
		}
	}

	b.State = &ExecutionData{
		ExecutionType:      params.Type,
		Executors:          executors,
		SLA:                params.SLA,
		LeftToNotify:       executors,
		ExecutorsGroupID:   params.ExecutorsGroupID,
		ExecutorsGroupName: executorsGroupName,
		FormsAccessibility: params.FormsAccessibility,
		IsEditable:         params.IsEditable,
		RepeatPrevDecision: params.RepeatPrevDecision,
	}

	return b, nil
}

type resolveExecutorsDTO struct {
	runCtx *store.VariableStore
	id     uuid.UUID
}

func (gb *GoExecutionBlock) resolveExecutors(ctx context.Context, dto *resolveExecutorsDTO, step *entity.Step) (err error) {
	variableStorage, grabStorageErr := dto.runCtx.GrabStorage()
	if grabStorageErr != nil {
		return grabStorageErr
	}

	resolvedEntities, resolveErr := resolveValuesFromVariables(variableStorage, gb.State.Executors)
	if resolveErr != nil {
		return resolveErr
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

	step.State[gb.Name], err = json.Marshal(gb.State)
	if err != nil {
		return err
	}

	content, err := json.Marshal(store.NewFromStep(step))
	if err != nil {
		return err
	}

	return gb.RunContext.Storage.UpdateStepContext(ctx, &db.UpdateStepRequest{
		Id:          dto.id,
		Content:     content,
		BreakPoints: step.BreakPoints,
		Status:      string(StatusRunning),
	})
}
