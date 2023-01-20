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
	ExecutionTypeUser       ExecutionType = "user"
	ExecutionTypeGroup      ExecutionType = "group"
	ExecutionTypeFromSchema ExecutionType = "from_schema"
)

type ExecutionParams struct {
	Type      ExecutionType `json:"type"`
	Executors string        `json:"executors"`

	ExecutorsGroupID   string `json:"executors_group_id"`
	ExecutorsGroupName string `json:"executors_group_name"`

	FormsAccessibility []FormAccessibility `json:"forms_accessibility"`

	SLA            int  `json:"sla"`
	CheckSLA       bool `json:"check_sla"`
	ReworkSLA      int  `json:"rework_sla"`
	CheckReworkSLA bool `json:"check_rework_sla"`

	IsEditable         bool `json:"is_editable"`
	RepeatPrevDecision bool `json:"repeat_prev_decision"`
}

func (a *ExecutionParams) Validate() error {
	if a.ExecutorsGroupID == "" && a.Type == ExecutionTypeGroup {
		return errors.New("executors group id is empty")
	}

	if a.Executors == "" && a.Type == ExecutionTypeUser {
		return errors.New("executor is empty")
	}

	typeExecution := ExecutionType(strings.ToLower(a.Type.String()))

	if typeExecution != ExecutionTypeUser &&
		typeExecution != ExecutionTypeGroup &&
		typeExecution != ExecutionTypeFromSchema {
		return fmt.Errorf("unknown executor type: %s", a.Type)
	}

	if a.IsEditable && a.ReworkSLA < 16 {
		return fmt.Errorf("invalid Rework SLA: %d", a.SLA)
	}

	return nil
}
