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
	Type     ExecutionType `json:"type"`
	Executors string        `json:"executors"`
}

func (a *ExecutionParams) Validate() error {
	if a.Executors == "" {
		return errors.New("executor is empty")
	}

	if a.Type != ExecutionTypeUser && a.Type != ExecutionTypeGroup {
		return fmt.Errorf("unknown executor type: %s", a.Type)
	}

	return nil
}
