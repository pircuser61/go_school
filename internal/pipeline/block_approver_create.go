package pipeline

import (
	c "context"
	"encoding/json"
	"fmt"
	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"golang.org/x/net/context"
	"time"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
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

	for _, v := range ef.Output {
		b.Output[v.Name] = v.Global
	}

	rawState, ok := runCtx.VarStore.State[name]
	if !ok {
		if err := b.loadState(rawState); err != nil {
			return nil, err
		}
	} else {
		if err := b.createState(ctx, ef, runCtx); err != nil {
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

func (gb *GoApproverBlock) createState(ctx context.Context, ef *entity.EriusFunc, runCtx *BlockRunContext) error {
	var params script.ApproverParams
	err := json.Unmarshal(ef.Params, &params)
	if err != nil {
		return errors.Wrap(err, "can not get approver parameters")
	}

	if err = params.Validate(); err != nil {
		return errors.Wrap(err, "invalid approver parameters")
	}

	gb.State = &ApproverData{
		Type:               params.Type,
		SLA:                params.SLA,
		AutoAction:         params.AutoAction,
		IsEditable:         params.IsEditable,
		RepeatPrevDecision: params.RepeatPrevDecision,
		ApproverLog:        make([]ApproverLogEntry, 0),
		FormsAccessibility: params.FormsAccessibility,
		ApprovementRule:    params.ApprovementRule,
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
		approversGroup, errGroup := runCtx.ServiceDesc.GetApproversGroup(ctx, params.ApproversGroupID)
		if errGroup != nil {
			return errors.Wrap(errGroup, "can`t get approvers group with id: "+params.ApproversGroupID)
		}

		if len(approversGroup.People) == 0 {
			return errors.Wrap(errGroup, "zero approvers in group: "+params.ApproversGroupID)
		}

		gb.State.Approvers = make(map[string]struct{})
		for i := range approversGroup.People {
			gb.State.Approvers[approversGroup.People[i].Login] = struct{}{}
		}
		gb.State.ApproversGroupID = params.ApproversGroupID
		gb.State.ApproversGroupName = approversGroup.GroupName
	case script.ApproverTypeFromSchema:
		variableStorage, grabStorageErr := runCtx.VarStore.GrabStorage()
		if grabStorageErr != nil {
			return err
		}

		resolvedEntities, resolveErr := resolveValuesFromVariables(
			variableStorage,
			map[string]struct{}{
				params.Approver: {},
			},
		)
		if resolveErr != nil {
			return err
		}

		gb.State.Approvers = resolvedEntities
	}

	gb.RunContext.VarStore.AddStep(gb.Name)

	return gb.handleNotifications(ctx)
}

//nolint:dupl // maybe later
func (gb *GoApproverBlock) handleNotifications(ctx c.Context) error {
	l := logger.GetLogger(ctx)

	emails := make([]string, 0, len(gb.State.Approvers))
	for approver := range gb.State.Approvers {
		email, err := gb.RunContext.People.GetUserEmail(ctx, approver)
		if err != nil {
			l.WithError(err).Error("couldn't get email")
		}
		emails = append(emails, email)
	}
	if len(emails) == 0 {
		return nil
	}
	data, err := gb.RunContext.Storage.GetApplicationData(gb.RunContext.WorkNumber)
	if err != nil {
		return err
	}
	var descr string
	dataDescr, ok := data.Get("description")
	if ok {
		convDescr, convOk := dataDescr.(string)
		if convOk {
			descr = convDescr
		}
	}
	additionalDescriptions, err := gb.RunContext.Storage.GetAdditionalForms(gb.RunContext.WorkNumber, gb.Name)
	if err != nil {
		return err
	}
	for _, item := range additionalDescriptions {
		if item == "" {
			continue
		}
		descr = fmt.Sprintf("%s\n\n%s", descr, item)
	}
	err = gb.RunContext.Sender.SendNotification(ctx, emails, nil,
		mail.NewApplicationPersonStatusNotification(
			gb.RunContext.WorkNumber,
			gb.RunContext.WorkTitle,
			statusToTaskAction[StatusApprovement],
			ComputeDeadline(time.Now(), gb.State.SLA),
			descr,
			gb.RunContext.Sender.SdAddress))
	if err != nil {
		return err
	}
	return nil
}

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

	parentStep, err = gb.RunContext.Storage.GetParentTaskStepByName(ctx,  gb.RunContext.TaskID, gb.Name)
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
