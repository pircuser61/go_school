package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

//nolint:dupl,goconst //another block // не нужно здесь чекать константы
func createGoApproverBlock(ctx context.Context, name string, ef *entity.EriusFunc, runCtx *BlockRunContext,
	expectedEvents map[string]struct{},
) (*GoApproverBlock, bool, error) {
	if ef.ShortTitle == "" {
		return nil, false, errors.New(ef.Title + " block short title is empty")
	}

	b := &GoApproverBlock{
		Name:       name,
		ShortName:  ef.ShortTitle,
		Title:      ef.Title,
		Input:      map[string]string{},
		Output:     map[string]string{},
		Sockets:    entity.ConvertSocket(ef.Sockets),
		RunContext: runCtx,

		expectedEvents: expectedEvents,
		happenedEvents: make([]entity.NodeEvent, 0),
	}

	for _, v := range ef.Input {
		b.Input[v.Name] = v.Global
	}

	// TODO: check existence of keyApproverDecision in Output
	if ef.Output != nil {
		//nolint:gocritic //в этом проекте не принято использовать поинтеры в коллекциях
		for propertyName, v := range ef.Output.Properties {
			if v.Global == "" {
				continue
			}

			b.Output[propertyName] = v.Global
		}
	}

	reEntry := false

	rawState, blockExists := runCtx.VarStore.State[name]
	if blockExists {
		loadReEntry, err := b.load(ctx, rawState, runCtx, name, ef)
		if err != nil {
			return nil, false, err
		}

		reEntry = loadReEntry
	} else {
		err := b.init(ctx, runCtx, name, ef)
		if err != nil {
			return nil, false, err
		}
	}

	return b, reEntry, nil
}

func (gb *GoApproverBlock) load(
	ctx context.Context,
	rawState json.RawMessage,
	runCtx *BlockRunContext,
	name string,
	ef *entity.EriusFunc,
) (reEntry bool, err error) {
	if errLoad := gb.loadState(rawState); errLoad != nil {
		return false, errLoad
	}

	reEntry = runCtx.UpdateData == nil

	if reEntry {
		err = gb.reentryMakeExpectedEvents(ctx, runCtx, name, ef)
		if err != nil {
			return false, err
		}
	}

	return reEntry, nil
}

func (gb *GoApproverBlock) init(ctx context.Context, runCtx *BlockRunContext, name string, ef *entity.EriusFunc) error {
	if err := gb.createState(ctx, ef); err != nil {
		return err
	}

	gb.RunContext.VarStore.AddStep(gb.Name)

	err := gb.makeExpectedEvents(ctx, runCtx, name, ef)
	if err != nil {
		return err
	}

	gb.setPrevDecision(ctx)

	return nil
}

func (gb *GoApproverBlock) reentryMakeExpectedEvents(
	ctx context.Context,
	runCtx *BlockRunContext,
	name string,
	ef *entity.EriusFunc,
) error {
	if err := gb.reEntry(ctx, ef); err != nil {
		return err
	}

	gb.RunContext.VarStore.AddStep(gb.Name)

	err := gb.makeExpectedEvents(ctx, runCtx, name, ef)
	if err != nil {
		return err
	}

	return nil
}

func (gb *GoApproverBlock) makeExpectedEvents(ctx context.Context, runCtx *BlockRunContext, name string, ef *entity.EriusFunc) error {
	status, _, _ := gb.GetTaskHumanStatus()

	if _, ok := gb.expectedEvents[eventStart]; ok {
		event, err := runCtx.MakeNodeStartEvent(ctx, MakeNodeStartEventArgs{
			NodeName:      name,
			NodeShortName: ef.ShortTitle,
			HumanStatus:   status,
			NodeStatus:    gb.GetStatus(),
		})
		if err != nil {
			return err
		}

		gb.happenedEvents = append(gb.happenedEvents, event)
	}

	toAddLogins := getSliceFromMap(gb.State.Approvers)

	sort.Strings(toAddLogins)

	kafkaEvent, err := runCtx.MakeNodeKafkaEvent(ctx, &MakeNodeKafkaEvent{
		EventName:     eventStart,
		NodeName:      name,
		NodeShortName: ef.ShortTitle,
		HumanStatus:   status,
		NodeStatus:    gb.GetStatus(),
		NodeType:      BlockGoApproverID,
		SLA:           gb.State.Deadline.Unix(),
		Rule:          gb.State.ApprovementRule.String(),
		ToAddLogins:   toAddLogins,
	})
	if err != nil {
		return err
	}

	gb.happenedKafkaEvents = append(gb.happenedKafkaEvents, kafkaEvent)

	return nil
}

func (gb *GoApproverBlock) reEntry(ctx context.Context, ef *entity.EriusFunc) error {
	if gb.State.GetRepeatPrevDecision() {
		return nil
	}

	gb.State.Decision = nil
	gb.State.Comment = nil
	gb.State.DecisionAttachments = make([]entity.Attachment, 0)
	gb.State.ActualApprover = nil
	gb.State.ApproverLog = make([]ApproverLogEntry, 0)
	gb.State.IsExpired = false

	var params script.ApproverParams

	err := json.Unmarshal(ef.Params, &params)
	if err != nil {
		return errors.Wrap(err, "can not get approver parameters for block: "+gb.Name)
	}

	if params.ApproversGroupIDPath != nil && *params.ApproversGroupIDPath != "" {
		variableStorage, grabStorageErr := gb.RunContext.VarStore.GrabStorage()
		if grabStorageErr != nil {
			return grabStorageErr
		}

		groupID := getVariable(variableStorage, *params.ApproversGroupIDPath)
		if groupID == nil {
			return errors.New("can't find group id in variables")
		}

		params.ApproversGroupID = fmt.Sprintf("%v", groupID)
	}

	if deadlineErr := gb.setWorkTypeAndDeadline(ctx, &params); deadlineErr != nil {
		return deadlineErr
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

//nolint:dupl //its not duplicate
func (gb *GoApproverBlock) createState(ctx context.Context, ef *entity.EriusFunc) error {
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
			ID:    socket.ID,
			Title: socket.Title,
			Type:  socket.ActionType,
		})
	}

	gb.State = &ApproverData{
		Type:               params.Type,
		CheckSLA:           params.CheckSLA,
		SLA:                params.SLA,
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
		WaitAllDecisions:   params.WaitAllDecisions,
	}

	if gb.State.AutoAction != nil {
		autoActionValid := false

		for _, a := range actions {
			if a.ID == string(*gb.State.AutoAction) {
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

	if params.ApproversGroupIDPath != nil && *params.ApproversGroupIDPath != "" {
		variableStorage, grabStorageErr := gb.RunContext.VarStore.GrabStorage()
		if grabStorageErr != nil {
			return grabStorageErr
		}

		groupID := getVariable(variableStorage, *params.ApproversGroupIDPath)
		if groupID == nil {
			return errors.New("can't find group id in variables")
		}

		params.ApproversGroupID = fmt.Sprintf("%v", groupID)
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

	if deadlineErr := gb.setWorkTypeAndDeadline(ctx, &params); deadlineErr != nil {
		return deadlineErr
	}

	return gb.handleNotifications(ctx)
}

func (gb *GoApproverBlock) setWorkTypeAndDeadline(ctx context.Context, params *script.ApproverParams) error {
	if params.WorkType != nil {
		gb.State.WorkType = *params.WorkType
	} else {
		task, getVersionErr := gb.RunContext.Services.Storage.GetVersionByWorkNumber(ctx, gb.RunContext.WorkNumber)
		if getVersionErr != nil {
			return getVersionErr
		}

		processSLASettings, getVersionErr := gb.RunContext.Services.Storage.GetSLAVersionSettings(
			ctx, task.VersionID.String())
		if getVersionErr != nil {
			return getVersionErr
		}

		gb.State.WorkType = processSLASettings.WorkType
	}

	deadline, err := gb.getDeadline(ctx, gb.State.WorkType)
	if err != nil {
		return err
	}

	gb.State.Deadline = deadline

	return nil
}

type setApproversByParamsDTO struct {
	Type     script.ApproverType
	GroupID  string
	Approver string
	WorkType *string
}

func (gb *GoApproverBlock) setApproversByParams(ctx context.Context, dto *setApproversByParamsDTO) error {
	switch dto.Type {
	case script.ApproverTypeUser:
		gb.State.Approvers = map[string]struct{}{
			dto.Approver: {},
		}
	case script.ApproverTypeHead:
	case script.ApproverTypeGroup:
		workGroup, errGroup := gb.RunContext.Services.ServiceDesc.GetWorkGroup(ctx, dto.GroupID)
		if errGroup != nil {
			return errors.Wrap(errGroup, "can`t get approvers group with id: "+dto.GroupID)
		}

		if len(workGroup.People) == 0 {
			return errors.New("zero approvers in group: " + dto.GroupID)
		}

		gb.State.Approvers = make(map[string]struct{}, len(workGroup.People))

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
			resolvedEntities, resolveErr := getUsersFromVars(
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
	}

	return nil
}

func (gb *GoApproverBlock) setPrevDecision(ctx context.Context) {
	decision := gb.State.GetDecision()

	if decision == nil && len(gb.State.EditingAppLog) == 0 && gb.State.GetIsEditable() {
		gb.setEditingAppLogFromPreviousBlock(ctx)
	}

	if !gb.State.RepeatPrevDecision {
		return
	}

	gb.setPreviousApprovers(ctx)

	if decision == nil {
		gb.trySetPreviousDecision(ctx)
	}
}

//nolint:dupl //its not duplicate
func (gb *GoApproverBlock) setEditingAppLogFromPreviousBlock(ctx context.Context) {
	const funcName = "setEditingAppLogFromPreviousBlock"

	l := logger.GetLogger(ctx)

	var (
		parentStep *entity.Step
		err        error
	)

	parentStep, err = gb.RunContext.Services.Storage.GetParentTaskStepByName(ctx, gb.RunContext.TaskID, gb.Name)
	if err != nil || parentStep == nil {
		return
	}

	// get state from step.State
	data, ok := parentStep.State[gb.Name]
	if !ok {
		//nolint:goconst //не хочу внедрять миллион констант под каждую строку в проекте
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

func (gb *GoApproverBlock) trySetPreviousDecision(ctx context.Context) (isPrevDecisionAssigned bool) {
	const funcName = "pipeline.approver.trySetPreviousDecision"

	l := logger.GetLogger(ctx)

	var (
		parentStep *entity.Step
		err        error
	)

	parentStep, err = gb.RunContext.Services.Storage.GetParentTaskStepByName(ctx, gb.RunContext.TaskID, gb.Name)
	if err != nil || parentStep == nil {
		l.Error(err)

		return false
	}

	data, exists := parentStep.State[gb.Name]
	if !exists {
		//nolint:goconst // не нужно здесь константы чекать
		l.Error(funcName, "parent step state is not found: "+gb.Name)

		return false
	}

	var parentState ApproverData
	if err = json.Unmarshal(data, &parentState); err != nil {
		l.Error(funcName, "invalid format of go-approver-block state")

		return false
	}

	if parentState.Decision == nil {
		return true
	}

	var actualApprover, comment string

	if parentState.ActualApprover != nil {
		actualApprover = *parentState.ActualApprover
	}

	if parentState.Comment != nil {
		comment = *parentState.Comment
	}

	person, personErr := gb.RunContext.Services.ServiceDesc.GetSsoPerson(ctx, actualApprover)
	if personErr != nil {
		//nolint:goconst //не хочу внедрять миллион констант под каждую строку в проекте
		l.Error(funcName, "service couldn't get person by login: "+actualApprover)

		return false
	}

	if valOutputApprover, ok := gb.Output[keyOutputApprover]; ok {
		gb.RunContext.VarStore.SetValue(valOutputApprover, person)
	}

	if valOutputDecision, ok := gb.Output[keyOutputDecision]; ok {
		gb.RunContext.VarStore.SetValue(valOutputDecision, parentState.Decision.String())
	}

	if valOutputComment, ok := gb.Output[keyOutputComment]; ok {
		gb.RunContext.VarStore.SetValue(valOutputComment, comment)
	}

	gb.State.ActualApprover = &actualApprover
	gb.State.Comment = &comment
	gb.State.Decision = parentState.Decision

	gb.State.ApproverLog = parentState.ApproverLog

	status, _, _ := gb.GetTaskHumanStatus()

	if _, exists = gb.expectedEvents[eventEnd]; exists {
		event, eventErr := gb.RunContext.MakeNodeEndEvent(ctx, MakeNodeEndEventArgs{
			NodeName:      gb.Name,
			NodeShortName: gb.ShortName,
			HumanStatus:   status,
			NodeStatus:    gb.GetStatus(),
		})

		if eventErr != nil {
			return false
		}

		gb.happenedEvents = append(gb.happenedEvents, event)
	}

	deadline, errDeadline := gb.getDeadline(ctx, gb.State.WorkType)
	if errDeadline != nil {
		return false
	}

	kafkaEvent, eventErr := gb.RunContext.MakeNodeKafkaEvent(ctx, &MakeNodeKafkaEvent{
		EventName:      eventEnd,
		NodeName:       gb.Name,
		NodeShortName:  gb.ShortName,
		HumanStatus:    status,
		NodeStatus:     gb.GetStatus(),
		NodeType:       BlockGoApproverID,
		SLA:            deadline.Unix(),
		ToRemoveLogins: []string{},
	})

	if eventErr != nil {
		return false
	}

	gb.happenedKafkaEvents = append(gb.happenedKafkaEvents, kafkaEvent)

	return true
}

// nolint:dupl // not dupl
func (gb *GoApproverBlock) setPreviousApprovers(ctx context.Context) {
	const funcName = "pipeline.approver.setPreviousApprovers"

	l := logger.GetLogger(ctx)

	var (
		parentStep *entity.Step
		err        error
	)

	parentStep, err = gb.RunContext.Services.Storage.GetParentTaskStepByName(ctx, gb.RunContext.TaskID, gb.Name)
	if err != nil || parentStep == nil {
		l.Error(err)

		return
	}

	data, ok := parentStep.State[gb.Name]
	if !ok {
		l.Error(funcName, "parent step state is not found: "+gb.Name)

		return
	}

	var parentState ApproverData
	if err = json.Unmarshal(data, &parentState); err != nil {
		l.Error(funcName, "invalid format of go-approver-block state")

		return
	}

	if parentState.Approvers != nil {
		gb.State.Approvers = map[string]struct{}{}
		for login := range parentState.Approvers {
			gb.State.Approvers[login] = struct{}{}
		}
	}
}
