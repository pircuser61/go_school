package pipeline

import (
	c "context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"

	e "gitlab.services.mts.ru/abp/mail/pkg/email"
	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

// nolint:dupl // another block
func createGoApproverBlock(ctx c.Context, name string, ef *entity.EriusFunc, runCtx *BlockRunContext) (*GoApproverBlock, bool, error) {
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

	rawState, blockExists := runCtx.VarStore.State[name]
	reEntry := false
	if blockExists {
		if err := b.loadState(rawState); err != nil {
			return nil, false, err
		}

		reEntry = runCtx.UpdateData == nil

		if reEntry {
			if err := b.reEntry(ctx, ef); err != nil {
				return nil, false, err
			}
			b.RunContext.VarStore.AddStep(b.Name)
		}
	} else {
		if err := b.createState(ctx, ef); err != nil {
			return nil, false, err
		}
		b.RunContext.VarStore.AddStep(b.Name)

		// TODO: выпилить когда сделаем циклы
		// это для возврата на доработку при которой мы создаем новый процесс
		// и пытаемся взять решение из прошлого процесса
		if err := b.setPrevDecision(ctx); err != nil {
			return nil, false, err
		}
	}

	return b, reEntry, nil
}

func (gb *GoApproverBlock) reEntry(ctx c.Context, ef *entity.EriusFunc) error {
	if gb.State.GetRepeatPrevDecision() {
		return nil
	}

	gb.State.Decision = nil
	gb.State.Comment = nil
	gb.State.DecisionAttachments = nil
	gb.State.ActualApprover = nil

	var params script.ApproverParams
	err := json.Unmarshal(ef.Params, &params)
	if err != nil {
		return errors.Wrap(err, "can not get approver parameters for block: "+gb.Name)
	}

	err = gb.setApproversByParams(ctx, &setApproversByParamsDTO{
		Type:     params.Type,
		GroupID:  params.ApproversGroupID,
		Approver: params.Approver,
		WorkType: params.WorkType,
	})
	if err != nil {
		return err
	}

	return gb.handleNotifications(ctx)
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

	setErr := gb.setApproversByParams(ctx, &setApproversByParamsDTO{
		Type:     params.Type,
		GroupID:  params.ApproversGroupID,
		Approver: params.Approver,
		WorkType: params.WorkType,
	})
	if setErr != nil {
		return setErr
	}

	if params.WorkType != nil {
		gb.State.WorkType = *params.WorkType
	} else {
		task, getVersionErr := gb.RunContext.Storage.GetVersionByWorkNumber(ctx, gb.RunContext.WorkNumber)
		if getVersionErr != nil {
			return getVersionErr
		}

		processSLASettings, getVersionErr := gb.RunContext.Storage.GetSlaVersionSettings(ctx, task.VersionID.String())
		if getVersionErr != nil {
			return getVersionErr
		}
		gb.State.WorkType = processSLASettings.WorkType
	}

	return gb.handleNotifications(ctx)
}

type setApproversByParamsDTO struct {
	Type     script.ApproverType
	GroupID  string
	Approver string
	WorkType *string
}

func (gb *GoApproverBlock) setApproversByParams(ctx c.Context, dto *setApproversByParamsDTO) error {
	fmt.Printf("%+v\n", dto)
	switch dto.Type {
	case script.ApproverTypeUser:
		gb.State.Approvers = map[string]struct{}{
			dto.Approver: {},
		}
	case script.ApproverTypeHead:
	case script.ApproverTypeGroup:
		workGroup, errGroup := gb.RunContext.ServiceDesc.GetWorkGroup(ctx, dto.GroupID)
		if errGroup != nil {
			return errors.Wrap(errGroup, "can`t get approvers group with id: "+dto.GroupID)
		}

		if len(workGroup.People) == 0 {
			return errors.New("zero approvers in group: " + dto.GroupID)
		}

		gb.State.Approvers = make(map[string]struct{})
		for i := range workGroup.People {
			gb.State.Approvers[workGroup.People[i].Login] = struct{}{}
		}
		gb.State.ApproversGroupID = dto.GroupID
		gb.State.ApproversGroupName = workGroup.GroupName
	case script.ApproverTypeFromSchema:
		variableStorage, grabStorageErr := gb.RunContext.VarStore.GrabStorage()
		if grabStorageErr != nil {
			return grabStorageErr
		}

		approversFromSchema := make(map[string]struct{})

		approversVars := strings.Split(dto.Approver, ";")
		for i := range approversVars {
			resolvedEntities, resolveErr := resolveValuesFromVariables(
				variableStorage,
				map[string]struct{}{
					approversVars[i]: {},
				},
			)
			if resolveErr != nil {
				return resolveErr
			}

			for approverLogin := range resolvedEntities {
				approversFromSchema[approverLogin] = struct{}{}
			}
		}

		gb.State.Approvers = approversFromSchema

		delegations, htErr := gb.RunContext.HumanTasks.GetDelegationsByLogins(ctx, getSliceFromMapOfStrings(gb.State.Approvers))
		if htErr != nil {
			return htErr
		}
		gb.RunContext.Delegations = delegations.FilterByType("approvement")
	}

	return nil
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

	approvers := getSliceFromMapOfStrings(gb.State.Approvers)
	loginsToNotify := delegates.GetUserInArrayWithDelegations(approvers)

	var emailAttachment []e.Attachment

	description, err := gb.RunContext.makeNotificationDescription(gb.Name)
	if err != nil {
		return err
	}

	actionsList := make([]mail.Action, 0, len(gb.State.ActionList))
	for i := range gb.State.ActionList {
		actionsList = append(actionsList, mail.Action{
			InternalActionName: gb.State.ActionList[i].Id,
			Title:              gb.State.ActionList[i].Title,
		})
	}

	task, getVersionErr := gb.RunContext.Storage.GetVersionByWorkNumber(ctx, gb.RunContext.WorkNumber)
	if getVersionErr != nil {
		return getVersionErr
	}

	processSettings, getVersionErr := gb.RunContext.Storage.GetVersionSettings(ctx, task.VersionID.String())
	if getVersionErr != nil {
		return getVersionErr
	}

	taskRunContext, getDataErr := gb.RunContext.Storage.GetTaskRunContext(ctx, gb.RunContext.WorkNumber)
	if getDataErr != nil {
		return getDataErr
	}

	login := task.Author

	recipient := getRecipientFromState(&taskRunContext.InitialApplication.ApplicationBody)

	if recipient != "" {
		login = recipient
	}

	lastWorksForUser := make([]*entity.EriusTask, 0)

	if processSettings.ResubmissionPeriod > 0 {
		var getWorksErr error
		lastWorksForUser, getWorksErr = gb.RunContext.Storage.GetWorksForUserWithGivenTimeRange(
			ctx,
			processSettings.ResubmissionPeriod,
			login,
			task.VersionID.String(),
			gb.RunContext.WorkNumber,
		)
		if getWorksErr != nil {
			return getWorksErr
		}
	}

	emails := make(map[string]mail.Template, 0)
	slaInfoPtr, getSlaInfoErr := GetSLAInfoPtr(ctx, GetSLAInfoDTOStruct{
		Service: gb.RunContext.HrGate,
		TaskCompletionIntervals: []entity.TaskCompletionInterval{{StartedAt: gb.RunContext.currBlockStartTime,
			FinishedAt: gb.RunContext.currBlockStartTime.Add(time.Hour * 24 * 100)}},
		WorkType: WorkHourType(gb.State.WorkType),
	})

	if getSlaInfoErr != nil {
		return getSlaInfoErr
	}
	for _, login := range loginsToNotify {
		email, getEmailErr := gb.RunContext.People.GetUserEmail(ctx, login)
		if getEmailErr != nil {
			l.WithField("login", login).WithError(getEmailErr).Warning("couldn't get email")
			continue
		}

		emails[email] = mail.NewAppPersonStatusNotificationTpl(
			&mail.NewAppPersonStatusTpl{
				WorkNumber:                gb.RunContext.WorkNumber,
				Name:                      gb.RunContext.NotifName,
				Status:                    gb.State.ApproveStatusName,
				Action:                    statusToTaskAction[StatusApprovement],
				DeadLine:                  ComputeDeadline(time.Now(), gb.State.SLA, slaInfoPtr),
				SdUrl:                     gb.RunContext.Sender.SdAddress,
				Mailto:                    gb.RunContext.Sender.FetchEmail,
				Login:                     login,
				IsEditable:                gb.State.GetIsEditable(),
				ApproverActions:           actionsList,
				Description:               description,
				BlockID:                   BlockGoApproverID,
				ExecutionDecisionExecuted: string(ExecutionDecisionExecuted),
				ExecutionDecisionRejected: string(ExecutionDecisionRejected),
				LastWorks:                 lastWorksForUser,
			})
	}

	for i := range emails {
		if sendErr := gb.RunContext.Sender.SendNotification(ctx, []string{i}, emailAttachment, emails[i]); sendErr != nil {
			return sendErr
		}
	}

	return nil
}

//nolint:unparam // Need here
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
