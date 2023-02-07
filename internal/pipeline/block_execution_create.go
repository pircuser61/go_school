package pipeline

import (
	c "context"
	"encoding/json"
	"time"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

// nolint:dupl // another block
func createGoExecutionBlock(ctx c.Context, name string, ef *entity.EriusFunc, runCtx *BlockRunContext) (*GoExecutionBlock, error) {
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

	if err := b.setPrevDecision(ctx); err != nil {
		return nil, err
	}
	return b, nil
}

func (gb *GoExecutionBlock) loadState(raw json.RawMessage) error {
	return json.Unmarshal(raw, &gb.State)
}

//nolint:dupl,gocyclo //its not duplicate
func (gb *GoExecutionBlock) createState(ctx c.Context, ef *entity.EriusFunc) error {
	var params script.ExecutionParams
	err := json.Unmarshal(ef.Params, &params)
	if err != nil {
		return errors.Wrap(err, "can not get execution parameters")
	}

	if err = params.Validate(); err != nil {
		return errors.Wrap(err, "invalid execution parameters, work number")
	}

	gb.State = &ExecutionData{
		ExecutionType:      params.Type,
		SLA:                params.SLA,
		CheckSLA:           params.CheckSLA,
		ReworkSLA:          params.ReworkSLA,
		CheckReworkSLA:     params.CheckReworkSLA,
		FormsAccessibility: params.FormsAccessibility,
		IsEditable:         params.IsEditable,
		RepeatPrevDecision: params.RepeatPrevDecision,
	}

	switch params.Type {
	case script.ExecutionTypeUser:
		gb.State.Executors = map[string]struct{}{
			params.Executors: {},
		}
	case script.ExecutionTypeFromSchema:
		variableStorage, grabStorageErr := gb.RunContext.VarStore.GrabStorage()
		if grabStorageErr != nil {
			return grabStorageErr
		}

		resolvedEntities, resolveErr := resolveValuesFromVariables(
			variableStorage,
			map[string]struct{}{
				params.Executors: {},
			},
		)
		if resolveErr != nil {
			return resolveErr
		}

		gb.State.Executors = resolvedEntities

		delegations, htErr := gb.RunContext.HumanTasks.GetDelegationsByLogins(ctx, getSliceFromMapOfStrings(gb.State.Executors))
		if htErr != nil {
			return htErr
		}
		delegations = delegations.FilterByType("execution")

		gb.RunContext.Delegations = delegations
	case script.ExecutionTypeGroup:
		executorsGroup, errGroup := gb.RunContext.ServiceDesc.GetExecutorsGroup(ctx, params.ExecutorsGroupID)
		if errGroup != nil {
			return errors.Wrap(errGroup, "can`t get executors group with id: "+params.ExecutorsGroupID)
		}

		if len(executorsGroup.People) == 0 {
			//nolint:goimports // bugged golint
			return errors.New("zero executors in group: " + params.ExecutorsGroupID)
		}

		gb.State.Executors = make(map[string]struct{})
		for i := range executorsGroup.People {
			gb.State.Executors[executorsGroup.People[i].Login] = struct{}{}
		}
		gb.State.ExecutorsGroupID = params.ExecutorsGroupID
		gb.State.ExecutorsGroupName = executorsGroup.GroupName
	}

	// maybe we should notify the executor
	if notifErr := gb.RunContext.handleInitiatorNotification(ctx, gb.Name, ef.TypeID, gb.GetTaskHumanStatus()); notifErr != nil {
		return notifErr
	}
	return gb.handleNotifications(ctx)
}

//nolint:dupl // maybe later
func (gb *GoExecutionBlock) handleNotifications(ctx c.Context) error {
	if gb.RunContext.skipNotifications {
		return nil
	}

	l := logger.GetLogger(ctx)
	delegates, getDelegationsErr := gb.RunContext.HumanTasks.GetDelegationsByLogins(ctx, getSliceFromMapOfStrings(gb.State.Executors))
	if getDelegationsErr != nil {
		return getDelegationsErr
	}
	delegates = delegates.FilterByType("execution")

	loginsToNotify := delegates.GetUserInArrayWithDelegations(getSliceFromMapOfStrings(gb.State.Executors))

	emails := make([]string, 0, len(loginsToNotify))
	for _, login := range loginsToNotify {
		email, getUserEmailErr := gb.RunContext.People.GetUserEmail(ctx, login)
		if getUserEmailErr != nil {
			l.WithField("login", login).WithError(getUserEmailErr).Warning("couldn't get email")
			continue
		}

		emails = append(emails, email)
	}

	if len(emails) == 0 {
		return nil
	}

	description, makeNotificationErr := gb.RunContext.makeNotificationDescription(gb.Name)
	if makeNotificationErr != nil {
		return makeNotificationErr
	}

	emails = utils.UniqueStrings(emails)

	for i := range emails {
		tpl := mail.NewAppPersonStatusNotificationTpl(
			&mail.NewAppPersonStatusTpl{
				WorkNumber:  gb.RunContext.WorkNumber,
				Name:        gb.RunContext.WorkTitle,
				Status:      string(StatusExecution),
				Action:      statusToTaskAction[StatusExecution],
				DeadLine:    ComputeDeadline(time.Now(), gb.State.SLA),
				Description: description,
				SdUrl:       gb.RunContext.Sender.SdAddress,
				Mailto:      gb.RunContext.Sender.FetchEmail,
				IsEditable:  gb.State.GetIsEditable(),

				BlockID:                   BlockGoExecutionID,
				ExecutionDecisionExecuted: string(ExecutionDecisionExecuted),
				ExecutionDecisionRejected: string(ExecutionDecisionRejected),
			})

		if sendNotificationErr := gb.RunContext.Sender.SendNotification(ctx, []string{emails[i]}, nil, tpl); sendNotificationErr != nil {
			return sendNotificationErr
		}
	}

	return nil
}

//nolint:unparam // ok here
func (gb *GoExecutionBlock) setPrevDecision(ctx c.Context) error {
	decision := gb.State.GetDecision()

	if decision == nil && len(gb.State.EditingAppLog) == 0 && gb.State.GetIsEditable() {
		gb.setEditingAppLogFromPreviousBlock(ctx)
	}

	if decision == nil && gb.State.GetRepeatPrevDecision() {
		if gb.trySetPreviousDecision(ctx) {
			return nil
		}
	}
	return nil
}

//nolint:dupl //its not duplicate
func (gb *GoExecutionBlock) setEditingAppLogFromPreviousBlock(ctx c.Context) {
	const funcName = "setEditingAppLogFromPreviousBlock"
	l := logger.GetLogger(ctx)

	var parentStep *entity.Step
	var err error

	parentStep, err = gb.RunContext.Storage.GetParentTaskStepByName(ctx, gb.RunContext.TaskID, gb.Name)
	if err != nil || parentStep == nil {
		return
	}

	// get state from step.State
	data, ok := parentStep.State[gb.Name]
	if !ok {
		l.Error(funcName, "step state is not found: "+gb.Name)
		return
	}

	var parentState ExecutionData
	if err = json.Unmarshal(data, &parentState); err != nil {
		l.Error(funcName, "invalid format of go-execution-block state")
		return
	}

	if len(parentState.EditingAppLog) > 0 {
		gb.State.EditingAppLog = parentState.EditingAppLog
	}
}

// nolint:dupl // not dupl
func (gb *GoExecutionBlock) trySetPreviousDecision(ctx c.Context) (isPrevDecisionAssigned bool) {
	const funcName = "pipeline.execution.trySetPreviousDecision"
	l := logger.GetLogger(ctx)

	var parentStep *entity.Step
	var err error

	parentStep, err = gb.RunContext.Storage.GetParentTaskStepByName(ctx, gb.RunContext.TaskID, gb.Name)
	if err != nil || parentStep == nil {
		l.Error(err)
		return false
	}

	data, ok := parentStep.State[gb.Name]
	if !ok {
		l.Error(funcName, "parent step state is not found: "+gb.Name)
		return false
	}

	var parentState ExecutionData
	if err = json.Unmarshal(data, &parentState); err != nil {
		l.Error(funcName, "invalid format of go-execution-block state")
		return false
	}

	if parentState.Decision != nil {
		var actualExecutor, comment string

		if parentState.ActualExecutor != nil {
			actualExecutor = *parentState.ActualExecutor
		}

		if parentState.DecisionComment != nil {
			comment = *parentState.DecisionComment
		}

		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputExecutionLogin], actualExecutor)
		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputDecision], &parentState.Decision)
		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputComment], comment)

		gb.State.ActualExecutor = &actualExecutor
		gb.State.DecisionComment = &comment
		gb.State.Decision = parentState.Decision
	}

	return true
}
