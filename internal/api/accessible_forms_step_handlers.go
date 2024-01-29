package api

import (
	"encoding/json"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/humantasks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
)

const (
	ApproverBlockType  = "approver"
	ExecutionBlockType = "execution"
	FormBlockType      = "form"

	RunningStatus = "running"
	IdleStatus    = "idle"
)

type AccessibleFormsApproverBlockTypeStepHandler struct {
	currentUser            string
	accessibleForms        map[string]struct{}
	approvementDelegations humantasks.Delegations
}

func NewAccessibleFormsApproverBlockTypeStepHandler(
	currentUser string,
	accessibleForms map[string]struct{},
	delegates humantasks.Delegations,
) *AccessibleFormsApproverBlockTypeStepHandler {
	return &AccessibleFormsApproverBlockTypeStepHandler{
		accessibleForms:        accessibleForms,
		approvementDelegations: delegates.FilterByType("approvement"),
		currentUser:            currentUser,
	}
}

func (h *AccessibleFormsApproverBlockTypeStepHandler) HandleStep(step *entity.Step) error {
	if step.State == nil || (step.Status != RunningStatus && step.Status != IdleStatus) {
		return nil
	}

	var approver pipeline.ApproverData

	err := json.Unmarshal(step.State[step.Name], &approver)
	if err != nil {
		return err
	}

	if !h.userHasAccess(&approver) {
		return nil
	}

	for _, form := range approver.FormsAccessibility {
		if form.AccessType != TypeAccessFormNone {
			h.accessibleForms[form.NodeID] = struct{}{}
		}
	}

	return nil
}

func (h *AccessibleFormsApproverBlockTypeStepHandler) userHasAccess(approver *pipeline.ApproverData) bool {
	if h.isApprover(approver) {
		return true
	}

	return h.isAdditionalApprover(approver)
}

func (h *AccessibleFormsApproverBlockTypeStepHandler) isApprover(approver *pipeline.ApproverData) bool {
	for member := range approver.Approvers {
		if h.currentUser == member || isDelegate(h.currentUser, member, &h.approvementDelegations) {
			return true
		}
	}

	return false
}

func (h *AccessibleFormsApproverBlockTypeStepHandler) isAdditionalApprover(approver *pipeline.ApproverData) bool {
	for _, member := range approver.AdditionalApprovers {
		if h.currentUser == member.ApproverLogin || isDelegate(h.currentUser, member.ApproverLogin, &h.approvementDelegations) {
			return true
		}
	}

	return false
}

type AccessibleFormsFormBlockTypeStepHandler struct {
	currentUser     string
	accessibleForms map[string]struct{}
}

func NewAccessibleFormsFormBlockTypeStepHandler(
	currentUser string,
	accessibleForms map[string]struct{},
) *AccessibleFormsFormBlockTypeStepHandler {
	return &AccessibleFormsFormBlockTypeStepHandler{
		currentUser:     currentUser,
		accessibleForms: accessibleForms,
	}
}

func (h *AccessibleFormsFormBlockTypeStepHandler) HandleStep(step *entity.Step) error {
	if step.State == nil || (step.Status != RunningStatus && step.Status != IdleStatus) {
		return nil
	}

	var form pipeline.FormData

	unmarshalErr := json.Unmarshal(step.State[step.Name], &form)
	if unmarshalErr != nil {
		return unmarshalErr
	}

	if !h.userHasAccess(&form) {
		return nil
	}

	h.accessibleForms[step.Name] = struct{}{}

	for _, form := range form.FormsAccessibility {
		if form.AccessType != TypeAccessFormNone {
			h.accessibleForms[form.NodeID] = struct{}{}
		}
	}

	return nil
}

func (h *AccessibleFormsFormBlockTypeStepHandler) userHasAccess(form *pipeline.FormData) bool {
	for member := range form.Executors {
		if h.currentUser == member {
			return true
		}
	}

	return false
}

type AccessibleFormsExecutionBlockTypeHandler struct {
	currentUser          string
	accessibleForms      map[string]struct{}
	executionDelegations humantasks.Delegations
}

func NewAccessibleFormsExecutionBlockTypeHandler(
	currentUser string,
	accessibleForms map[string]struct{},
	delegates humantasks.Delegations,
) *AccessibleFormsExecutionBlockTypeHandler {
	return &AccessibleFormsExecutionBlockTypeHandler{
		currentUser:          currentUser,
		accessibleForms:      accessibleForms,
		executionDelegations: delegates.FilterByType("execution"),
	}
}

func (h *AccessibleFormsExecutionBlockTypeHandler) HandleStep(step *entity.Step) error {
	if step.State == nil || (step.Status != RunningStatus && step.Status != IdleStatus) {
		return nil
	}

	var execution pipeline.ExecutionData

	unmarshalErr := json.Unmarshal(step.State[step.Name], &execution)
	if unmarshalErr != nil {
		return unmarshalErr
	}

	if !h.userHasAccess(&execution) {
		return nil
	}

	for _, form := range execution.FormsAccessibility {
		if form.AccessType != TypeAccessFormNone {
			h.accessibleForms[form.NodeID] = struct{}{}
		}
	}

	return nil
}

func (h *AccessibleFormsExecutionBlockTypeHandler) userHasAccess(execution *pipeline.ExecutionData) bool {
	for member := range execution.Executors {
		if member == h.currentUser || isDelegate(h.currentUser, member, &h.executionDelegations) {
			return true
		}
	}

	return false
}
