package script

import (
	"errors"
	"fmt"
	"strings"
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
	Type      ExecutionType `json:"type"`
	Executors string        `json:"executors"`

	ExecutorsGroupID   string `json:"executors_group_id"`
	ExecutorsGroupName string `json:"executors_group_name"`

	FormsAccessibility []FormAccessibility `json:"formsAccessibility"`

	SLA int `json:"sla"`
}

func (a *ExecutionParams) Validate() error {
	if a.ExecutorsGroupID == "" && a.Type == ExecutionTypeGroup {
		return errors.New("executors group id is empty")
	}

	if a.Executors == "" && a.Type == ExecutionTypeUser {
		return errors.New("executor is empty")
	}

	typeExecution := ExecutionType(strings.ToLower(a.Type.String()))

	if typeExecution != ExecutionTypeUser && typeExecution != ExecutionTypeGroup {
		return fmt.Errorf("unknown executor type: %s", a.Type)
	}

	if a.SLA < 1 {
		return fmt.Errorf("bad SLA value: %d", a.SLA)
	}

	return nil
}
