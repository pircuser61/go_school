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
	Type          ApproverType `json:"type"`
	ApproverLogin string       `json:"login"`
	// TODO GroupID
}

func (a *ApproverParams) Validate() error {
	if a.ApproverLogin == "" {
		return errors.New("approver is empty")
	}

	if a.Type != ApproverTypeUser && a.Type != ApproverTypeGroup && a.Type != ApproverTypeHead {
		return fmt.Errorf("unknown approver type: %s", a.Type)
	}

	return nil
}
