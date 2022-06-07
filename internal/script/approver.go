package script

import (
	"errors"
	"fmt"
)

type ApproverType string

func (a ApproverType) String() string {
	return string(a)
}

const (
	ApproverTypeUser  ApproverType = "user"
	ApproverTypeGroup ApproverType = "group"
	ApproverTypeHead  ApproverType = "head"
)

type ApproverParams struct {
	Type     ApproverType `json:"type"`
	Approver string       `json:"approver"`
}

func (a *ApproverParams) Validate() error {
	if a.Approver == "" {
		return errors.New("approver is empty")
	}

	if a.Type != ApproverTypeUser && a.Type != ApproverTypeGroup && a.Type != ApproverTypeHead {
		return fmt.Errorf("unknown approver type: %s", a.Type)
	}

	return nil
}
