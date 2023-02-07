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
func createGoApproverBlock(ctx c.Context, name string, ef *entity.EriusFunc, runCtx *BlockRunContext) (*GoApproverBlock, error) {
	b := &GoApproverBlock{
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

	// TODO: check existence of keyApproverDecision in Output

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

func (gb *GoApproverBlock) loadState(raw json.RawMessage) error {
	return json.Unmarshal(raw, &gb.State)
}

//nolint:dupl,gocyclo //its not duplicate
func (gb *GoApproverBlock) createState(ctx c.Context, ef *entity.EriusFunc) error {
	var params script.ApproverParams
	err := json.Unmarshal(ef.Params, &params)
	if err != nil {
		return errors.Wrap(err, "can not get approver parameters")
	}

	if err = params.Validate(); err != nil {
		return errors.Wrap(err, "invalid approver parameters")
	}

	actions := make([]Action, 0, len(ef.Sockets))

	for _, socket := range ef.Sockets {
		actions = append(actions, Action{
			Id:    socket.Id,
			Title: socket.Title,
			Type:  socket.ActionType,
		})
	}

	gb.State = &ApproverData{
		Type:               params.Type,
		SLA:                params.SLA,
		CheckSLA:           params.CheckSLA,
		ReworkSLA:          params.ReworkSLA,
		CheckReworkSLA:     params.CheckReworkSLA,
		AutoAction:         ApproverActionFromString(params.AutoAction),
		IsEditable:         params.IsEditable,
		RepeatPrevDecision: params.RepeatPrevDecision,
		ApproverLog:        make([]ApproverLogEntry, 0),
		FormsAccessibility: params.FormsAccessibility,
		ApprovementRule:    params.ApprovementRule,
		ApproveStatusName:  params.ApproveStatusName,
		ActionList:         actions,
	}

	if gb.State.AutoAction != nil {
		autoActionValid := false
		for _, a := range actions {
			if a.Id == string(*gb.State.AutoAction) {
				autoActionValid = true
				break
			}
		}
		if !autoActionValid {
			return errors.New("bad auto action")
		}
	}

	if gb.State.ApprovementRule == "" {
		gb.State.ApprovementRule = script.AnyOfApprovementRequired
	}

	switch params.Type {
	case script.ApproverTypeUser:
		gb.State.Approvers = map[string]struct{}{
			params.Approver: {},
		}
	case script.ApproverTypeHead:
	case script.ApproverTypeGroup:
		approversGroup, errGroup := gb.RunContext.ServiceDesc.GetApproversGroup(ctx, params.ApproversGroupID)
		if errGroup != nil {
			return errors.Wrap(errGroup, "can`t get approvers group with id: "+params.ApproversGroupID)
		}

		if len(approversGroup.People) == 0 {
			return errors.New("zero approvers in group: "+params.ApproversGroupID)
		}

		gb.State.Approvers = make(map[string]struct{})
		for i := range approversGroup.People {
			gb.State.Approvers[approversGroup.People[i].Login] = struct{}{}
		}
		gb.State.ApproversGroupID = params.ApproversGroupID
		gb.State.ApproversGroupName = approversGroup.GroupName
	case script.ApproverTypeFromSchema:
		variableStorage, grabStorageErr := gb.RunContext.VarStore.GrabStorage()
		if grabStorageErr != nil {
			return grabStorageErr
		}

		resolvedEntities, resolveErr := resolveValuesFromVariables(
			variableStorage,
			map[string]struct{}{
				params.Approver: {},
			},
		)
		if resolveErr != nil {
			return resolveErr
		}

		gb.State.Approvers = resolvedEntities

		delegations, htErr := gb.RunContext.HumanTasks.GetDelegationsByLogins(ctx, getSliceFromMapOfStrings(gb.State.Approvers))
		if htErr != nil {
			return htErr
		}
		delegations = delegations.FilterByType("approvement")

		gb.RunContext.Delegations = delegations
	}

	gb.RunContext.VarStore.AddStep(gb.Name)

	return gb.handleNotifications(ctx)
}

//nolint:dupl // maybe later
func (gb *GoApproverBlock) handleNotifications(ctx c.Context) error {
	if gb.RunContext.skipNotifications {
		return nil
	}

	l := logger.GetLogger(ctx)

	delegates, getDelegationsErr := gb.RunContext.HumanTasks.GetDelegationsByLogins(ctx, getSliceFromMapOfStrings(gb.State.Approvers))
	if getDelegationsErr != nil {
		return getDelegationsErr
	}
	delegates = delegates.FilterByType("approvement")

	loginsToNotify := delegates.GetUserInArrayWithDelegations(getSliceFromMapOfStrings(gb.State.Approvers))

	emails := make([]string, 0, len(loginsToNotify))
	for _, login := range loginsToNotify {
		email, getEmailErr := gb.RunContext.People.GetUserEmail(ctx, login)
		if getEmailErr != nil {
			l.WithField("login", login).WithError(getEmailErr).Warning("couldn't get email")
			continue
		}

		emails = append(emails, email)
	}

	if len(emails) == 0 {
		return nil
	}

	description, makeNotifErr := gb.RunContext.makeNotificationDescription(gb.Name)
	if makeNotifErr != nil {
		return makeNotifErr
	}

	actionsList := make([]mail.Action, 0, len(gb.State.ActionList))
	for i := range gb.State.ActionList {
		actionsList = append(actionsList, mail.Action{
			InternalActionName: gb.State.ActionList[i].Id,
			Title:              gb.State.ActionList[i].Title,
		})
	}

	emails = utils.UniqueStrings(emails)

	for i := range emails {
		tpl := mail.NewAppPersonStatusNotificationTpl(
			&mail.NewAppPersonStatusTpl{
				WorkNumber:      gb.RunContext.WorkNumber,
				Name:            gb.RunContext.WorkTitle,
				Status:          gb.State.ApproveStatusName,
				Action:          statusToTaskAction[StatusApprovement],
				DeadLine:        ComputeDeadline(time.Now(), gb.State.SLA),
				Description:     description,
				SdUrl:           gb.RunContext.Sender.SdAddress,
				Mailto:          gb.RunContext.Sender.FetchEmail,
				IsEditable:      gb.State.GetIsEditable(),
				ApproverActions: actionsList,

				BlockID:                   BlockGoApproverID,
				ExecutionDecisionExecuted: string(ExecutionDecisionExecuted),
				ExecutionDecisionRejected: string(ExecutionDecisionRejected),
			})

		if sendErr := gb.RunContext.Sender.SendNotification(ctx, []string{emails[i]}, nil, tpl); sendErr != nil {
			return sendErr
		}
	}

	return nil
}

//nolint:unparam
func (gb *GoApproverBlock) setPrevDecision(ctx c.Context) error {
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
func (gb *GoApproverBlock) setEditingAppLogFromPreviousBlock(ctx c.Context) {
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

	var parentState ApproverData

	if err = json.Unmarshal(data, &parentState); err != nil {
		l.Error(funcName, "invalid format of go-approver-block state")
		return
	}

	if len(parentState.EditingAppLog) > 0 {
		gb.State.EditingAppLog = parentState.EditingAppLog
	}
}

func (gb *GoApproverBlock) trySetPreviousDecision(ctx c.Context) (isPrevDecisionAssigned bool) {
	const funcName = "pipeline.approver.trySetPreviousDecision"
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

	var parentState ApproverData
	if err = json.Unmarshal(data, &parentState); err != nil {
		l.Error(funcName, "invalid format of go-approver-block state")
		return false
	}

	if parentState.Decision != nil {
		var actualApprover, comment string

		if parentState.ActualApprover != nil {
			actualApprover = *parentState.ActualApprover
		}

		if parentState.Comment != nil {
			comment = *parentState.Comment
		}

		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputApprover], actualApprover)
		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputDecision], parentState.Decision.String())
		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputComment], comment)

		gb.State.ActualApprover = &actualApprover
		gb.State.Comment = &comment
		gb.State.Decision = parentState.Decision
	}

	return true
}
