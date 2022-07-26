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

	SLA        int         `json:"sla"`
	AutoAction *AutoAction `json:"auto_action,omitempty"`

	IsEditable         bool `json:"is_editable"`
	RepeatPrevDecision bool `json:"repeat_prev_decision"`
}

func (a *ApproverParams) Validate() error {
	if a.Approver == "" {
		return errors.New("approver is empty")
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

	return nil
}
