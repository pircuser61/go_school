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

	ExecutorsGroupID     string  `json:"executors_group_id"`
	ExecutorsGroupName   string  `json:"executors_group_name"`
	ExecutorsGroupIDPath *string `json:"executors_group_id_path"`

	FormsAccessibility []FormAccessibility `json:"forms_accessibility"`

	SLA            int  `json:"sla"`
	CheckSLA       bool `json:"check_sla"`
	ReworkSLA      int  `json:"rework_sla"`
	CheckReworkSLA bool `json:"check_rework_sla"`

	IsEditable           bool    `json:"is_editable"`
	RepeatPrevDecision   bool    `json:"repeat_prev_decision"`
	WorkType             *string `json:"work_type"`
	UseActualExecutor    bool    `json:"use_actual_executor"`
	HideExecutor         bool    `json:"hide_executor"`
	ChildWorkBlueprintID *string `json:"child_work_blueprint_id"`
}

func (a *ExecutionParams) Validate() error {
	if a.ExecutorsGroupID == "" && a.ExecutorsGroupIDPath == nil && a.Type == ExecutionTypeGroup {
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

	if typeExecution == ExecutionTypeFromSchema && len(strings.Split(a.Executors, ";")) < 1 {
		return errors.New("execution from schema is empty")
	}

	if a.IsEditable && a.CheckReworkSLA && a.ReworkSLA < 16 {
		return fmt.Errorf("invalid Rework SLA: %d", a.SLA)
	}

	if a.CheckSLA && (a.SLA <= 0 || a.WorkType == nil) {
		return fmt.Errorf("invalid SLA or empty WorkType: %d %v", a.SLA, a.WorkType)
	}

	return nil
}
