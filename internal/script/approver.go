package script

import (
	"errors"
	"fmt"
)

type ApproverType string

func (a ApproverType) String() string {
	return string(a)
}

type AutoAction string

const (
	ApproverTypeUser       ApproverType = "user"
	ApproverTypeGroup      ApproverType = "group"
	ApproverTypeHead       ApproverType = "head"
	ApproverTypeFromSchema ApproverType = "fromSchema"

	AutoActionApprove AutoAction = "approve"
	AutoActionReject  AutoAction = "reject"
)

type ApproverParams struct {
	Type     ApproverType `json:"type"`
	Approver string       `json:"approver"`

	SLA                int                 `json:"sla"`
	AutoAction         *AutoAction         `json:"auto_action,omitempty"`
	FormsAccessibility []FormAccessibility `json:"formsAccessibility"`

	IsEditable         bool `json:"is_editable"`
	RepeatPrevDecision bool `json:"repeat_prev_decision"`

	ApproversGroupID   string `json:"approvers_group_id"`
	ApproversGroupName string `json:"approvers_group_name"`
}

func (a *ApproverParams) Validate() error {
	if a.Approver == "" && a.Type == ApproverTypeUser {
		return errors.New("approver is empty")
	}

	if a.ApproversGroupID == "" && a.Type == ApproverTypeGroup {
		return errors.New("approvers group id is empty")
	}

	typeApprove := ApproverType(a.Type.String())

	if typeApprove != ApproverTypeUser &&
		typeApprove != ApproverTypeGroup &&
		typeApprove != ApproverTypeHead &&
		typeApprove != ApproverTypeFromSchema {
		return fmt.Errorf("unknown approver type: %s", a.Type)
	}

	if a.SLA < 1 {
		return fmt.Errorf("bad SLA value: %d", a.SLA)
	}

	if a.AutoAction != nil && *a.AutoAction != AutoActionApprove && *a.AutoAction != AutoActionReject {
		return fmt.Errorf("unknown auto action type: %s", *a.AutoAction)
	}

	if a.Type == ApproverTypeGroup && a.ApproversGroupID == "" {
		return fmt.Errorf("empty ApproversGroupID")
	}

	return nil
}
