package pipeline

import (
	c "context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
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
	gb.State.DecisionAttachments = make([]string, 0)
	gb.State.ActualApprover = nil
	gb.State.ApproverLog = make([]ApproverLogEntry, 0)

	var params script.ApproverParams
	err := json.Unmarshal(ef.Params, &params)
	if err != nil {
		return errors.Wrap(err, "can not get approver parameters for block: "+gb.Name)
	}

	if params.ApproversGroupIDPath != nil {
		variableStorage, grabStorageErr := gb.RunContext.VarStore.GrabStorage()
		if grabStorageErr != nil {
			return grabStorageErr
		}

		groupId := getVariable(variableStorage, *params.ApproversGroupIDPath)
		if groupId == nil {
			return errors.New("can't find group id in variables")
		}
		params.ApproversGroupID = fmt.Sprintf("%v", groupId)
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

	if params.ApproversGroupIDPath != nil && *params.ApproversGroupIDPath != "" {
		variableStorage, grabStorageErr := gb.RunContext.VarStore.GrabStorage()
		if grabStorageErr != nil {
			return grabStorageErr
		}

		groupId := getVariable(variableStorage, *params.ApproversGroupIDPath)
		if groupId == nil {
			return errors.New("can't find group id in variables")
		}
		params.ApproversGroupID = fmt.Sprintf("%v", groupId)
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
