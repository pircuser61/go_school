package script

import (
	"errors"
	"fmt"
)

type ExecutionType string

func (a ExecutionType) String() string {
	return string(a)
}

const (
	ExecutionTypeUser  ExecutionType = "user"
	ExecutionTypeGroup ExecutionType = "group"
)

type ExecutionParams struct {
	ApplicationID string `json:"application_id"`

	Type      ExecutionType `json:"type"`
	Executors []string      `json:"executors"`
}

func (a *ExecutionParams) Validate() error {
	if a.ApplicationID == "" {
		return errors.New("application_id is empty")
	}

	if len(a.Executors) == 0 {
		return errors.New("executor is empty")
	}

	if a.Type != ExecutionTypeUser && a.Type != ExecutionTypeGroup {
		return fmt.Errorf("unknown executor type: %s", a.Type)
	}

	return nil
}
