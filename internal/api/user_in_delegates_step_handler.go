package api

import (
	"encoding/json"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/humantasks"
)

type UserInDelegatesApproverBlockTypeStepHandler struct {
	currentUser          string
	approvementDeletages humantasks.Delegations
	userInDelegates      map[string]bool
}

func NewUserInDelegatesApproverBlockTypeStepHandler(
	currentUser string,
	delegates humantasks.Delegations,
	userInDelegates map[string]bool,
) *UserInDelegatesApproverBlockTypeStepHandler {
	return &UserInDelegatesApproverBlockTypeStepHandler{
		currentUser:          currentUser,
		approvementDeletages: delegates.FilterByType("approvement"),
		userInDelegates:      userInDelegates,
	}
}

func (h *UserInDelegatesApproverBlockTypeStepHandler) HandleStep(step *entity.Step) error {
	if step.State == nil {
		return nil
	}

	var approver approverBlock

	unmarshalErr := json.Unmarshal(step.State[step.Name], &approver)
	if unmarshalErr != nil {
		return unmarshalErr
	}

	h.userInDelegates[step.Name] = h.isDelegateAnyPersonOfStep(&approver)

	return nil
}

func (h *UserInDelegatesApproverBlockTypeStepHandler) isDelegateAnyPersonOfStep(approver *approverBlock) bool {
	if h.isApprover(approver) {
		return true
	}

	return h.isAdditionalApprover(approver)
}

func (h *UserInDelegatesApproverBlockTypeStepHandler) isApprover(approver *approverBlock) bool {
	for member := range approver.Approvers {
		if isDelegate(h.currentUser, member, &h.approvementDeletages) {
			return true
		}
	}

	return false
}

func (h *UserInDelegatesApproverBlockTypeStepHandler) isAdditionalApprover(approver *approverBlock) bool {
	for _, member := range approver.AdditionalApprovers {
		if isDelegate(h.currentUser, member.ApproverLogin, &h.approvementDeletages) {
			return true
		}
	}

	return false
}

type UserInDelegatesExecutionFromBlockTypesStepHandler struct {
	currentUser        string
	executionDelegates humantasks.Delegations
	userInDelegates    map[string]bool
}

func NewUserInDelegatesExecutionFromBlockTypesStepHandler(
	currentUser string,
	delegates humantasks.Delegations,
	userInDelegates map[string]bool,
) *UserInDelegatesExecutionFromBlockTypesStepHandler {
	return &UserInDelegatesExecutionFromBlockTypesStepHandler{
		currentUser:        currentUser,
		executionDelegates: delegates.FilterByType("execution"),
		userInDelegates:    userInDelegates,
	}
}

func (h *UserInDelegatesExecutionFromBlockTypesStepHandler) HandleStep(step *entity.Step) error {
	if step.State == nil {
		return nil
	}

	var execution executionBlock

	unmarshalErr := json.Unmarshal(step.State[step.Name], &execution)
	if unmarshalErr != nil {
		return unmarshalErr
	}

	h.userInDelegates[step.Name] = h.isDelegateAnyPersonOfStep(&execution)

	return nil
}

func (h *UserInDelegatesExecutionFromBlockTypesStepHandler) isDelegateAnyPersonOfStep(execution *executionBlock) bool {
	for member := range execution.Executors {
		if isDelegate(h.currentUser, member, &h.executionDelegates) {
			return true
		}
	}

	return false
}
