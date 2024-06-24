package stephandlers

import (
	"encoding/json"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/humantasks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
)

const (
	TypeAccessFormNone = "None"
)

type AccessibleFormsApproverBlockStepHandler struct {
	currentUser            string
	accessibleForms        map[string]struct{}
	approvementDelegations humantasks.Delegations
}

func NewAccessibleFormsApproverBlockStepHandler(
	currentUser string,
	accessibleForms map[string]struct{},
	delegates humantasks.Delegations,
) *AccessibleFormsApproverBlockStepHandler {
	return &AccessibleFormsApproverBlockStepHandler{
		accessibleForms:        accessibleForms,
		approvementDelegations: delegates.FilterByType("approvement"),
		currentUser:            currentUser,
	}
}

func (h *AccessibleFormsApproverBlockStepHandler) HandleStep(step *entity.Step) error {
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

func (h *AccessibleFormsApproverBlockStepHandler) userHasAccess(approver *pipeline.ApproverData) bool {
	if h.isApprover(approver) {
		return true
	}

	return h.isAdditionalApprover(approver)
}

func (h *AccessibleFormsApproverBlockStepHandler) isApprover(approver *pipeline.ApproverData) bool {
	for member := range approver.Approvers {
		if h.currentUser == member || isDelegate(h.currentUser, member, &h.approvementDelegations) {
			return true
		}
	}

	return false
}

func (h *AccessibleFormsApproverBlockStepHandler) isAdditionalApprover(approver *pipeline.ApproverData) bool {
	for _, member := range approver.AdditionalApprovers {
		if h.currentUser == member.ApproverLogin || isDelegate(h.currentUser, member.ApproverLogin, &h.approvementDelegations) {
			return true
		}
	}

	return false
}

type AccessibleFormsFormBlockStepHandler struct {
	currentUser     string
	accessibleForms map[string]struct{}
}

func NewAccessibleFormsFormBlockStepHandler(
	currentUser string,
	accessibleForms map[string]struct{},
) *AccessibleFormsFormBlockStepHandler {
	return &AccessibleFormsFormBlockStepHandler{
		currentUser:     currentUser,
		accessibleForms: accessibleForms,
	}
}

func (h *AccessibleFormsFormBlockStepHandler) HandleStep(step *entity.Step) error {
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

func (h *AccessibleFormsFormBlockStepHandler) userHasAccess(form *pipeline.FormData) bool {
	for member := range form.Executors {
		if h.currentUser == member {
			return true
		}
	}

	for member := range form.InitialExecutors {
		if h.currentUser == member {
			return true
		}
	}

	return false
}

type AccessibleFormsExecutionBlockStepHandler struct {
	currentUser          string
	accessibleForms      map[string]struct{}
	executionDelegations humantasks.Delegations
}

func NewAccessibleFormsExecutionBlockStepHandler(
	currentUser string,
	accessibleForms map[string]struct{},
	delegates humantasks.Delegations,
) *AccessibleFormsExecutionBlockStepHandler {
	return &AccessibleFormsExecutionBlockStepHandler{
		currentUser:          currentUser,
		accessibleForms:      accessibleForms,
		executionDelegations: delegates.FilterByType("execution"),
	}
}

func (h *AccessibleFormsExecutionBlockStepHandler) HandleStep(step *entity.Step) error {
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

func (h *AccessibleFormsExecutionBlockStepHandler) userHasAccess(execution *pipeline.ExecutionData) bool {
	for member := range execution.Executors {
		if member == h.currentUser || isDelegate(h.currentUser, member, &h.executionDelegations) {
			return true
		}
	}

	for member := range execution.InitialExecutors {
		if member == h.currentUser || isDelegate(h.currentUser, member, &h.executionDelegations) {
			return true
		}
	}

	for i := range execution.ChangedExecutorsLogs {
		member := execution.ChangedExecutorsLogs[i].OldLogin
		if member == h.currentUser || isDelegate(h.currentUser, member, &h.executionDelegations) {
			return true
		}
	}

	return false
}

type AccessibleFormsSignBlockStepHandler struct {
	currentUser     string
	accessibleForms map[string]struct{}
}

func NewAccessibleFormsSignBlockStepHandler(currentUser string, accessibleForms map[string]struct{}) *AccessibleFormsSignBlockStepHandler {
	return &AccessibleFormsSignBlockStepHandler{
		currentUser:     currentUser,
		accessibleForms: accessibleForms,
	}
}

func (h *AccessibleFormsSignBlockStepHandler) HandleStep(s *entity.Step) error {
	var sign pipeline.SignData

	unmarshalErr := json.Unmarshal(s.State[s.Name], &sign)
	if unmarshalErr != nil {
		return unmarshalErr
	}

	if !h.userHasAccess(&sign) {
		return nil
	}

	for _, form := range sign.FormsAccessibility {
		if form.AccessType != TypeAccessFormNone {
			h.accessibleForms[form.NodeID] = struct{}{}
		}
	}

	return nil
}

func (h *AccessibleFormsSignBlockStepHandler) userHasAccess(sign *pipeline.SignData) bool {
	if h.isSigner(sign) {
		return true
	}

	return h.isAdditionalApprover(sign)
}

func (h *AccessibleFormsSignBlockStepHandler) isSigner(sign *pipeline.SignData) bool {
	for member := range sign.Signers {
		if member == h.currentUser {
			return true
		}
	}

	return false
}

func (h *AccessibleFormsSignBlockStepHandler) isAdditionalApprover(sign *pipeline.SignData) bool {
	for addMember := range sign.AdditionalApprovers {
		if sign.AdditionalApprovers[addMember].ApproverLogin == h.currentUser {
			return true
		}
	}

	return false
}
