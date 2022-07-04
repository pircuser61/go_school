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
	ApproverTypeUser  ApproverType = "user"
	ApproverTypeGroup ApproverType = "group"
	ApproverTypeHead  ApproverType = "head"

	AutoActionApprove AutoAction = "approve"
	AutoActionReject  AutoAction = "reject"
)

type ApproverParams struct {
	Type     ApproverType `json:"type"`
	Approver string       `json:"approver"`

	SLA        int         `json:"sla"`
	AutoAction *AutoAction `json:"auto_action"`
}

func (a *ApproverParams) Validate() error {
	if a.Approver == "" {
		return errors.New("approver is empty")
	}

	if a.Type != ApproverTypeUser && a.Type != ApproverTypeGroup && a.Type != ApproverTypeHead {
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
