package script

import (
	"errors"
	"fmt"
)

type FormExecutorType string

func (f FormExecutorType) String() string {
	return string(f)
}

const (
	FormExecutorTypeUser         FormExecutorType = "user"
	FormExecutorTypeInitiator    FormExecutorType = "initiator"
	FormExecutorTypeFromSchema   FormExecutorType = "from_schema"
	FormExecutorTypeAutoFillUser FormExecutorType = "auto_fill"
	FormExecutorTypeGroup        FormExecutorType = "group"
)

type FormParams struct {
	SchemaId                  string               `json:"schema_id"`
	SLA                       int                  `json:"sla"`
	CheckSLA                  bool                 `json:"check_sla"`
	SchemaName                string               `json:"schema_name"`
	Executor                  string               `json:"executor"`
	FormExecutorType          FormExecutorType     `json:"form_executor_type"`
	FormGroupId               string               `json:"form_group_id"`
	FormsAccessibility        []FormAccessibility  `json:"forms_accessibility"`
	HideExecutorFromInitiator bool                 `json:"hide_executor_from_initiator"`
	Mapping                   JSONSchemaProperties `json:"mapping"`
	RepeatPrevDecision        bool                 `json:"repeat_prev_decision"`
	ReEnterSettings           *FormReEnterSettings `json:"form_re_enter_settings"`
}

func (a *FormParams) Validate() error {
	if a.SchemaId == "" || (a.FormExecutorType == FormExecutorTypeUser && a.Executor == "") {
		return errors.New("got no form name, id or executor")
	}

	if a.SLA < 1 && a.CheckSLA {
		return fmt.Errorf("invalid SLA value %d", a.SLA)
	}

	if !a.RepeatPrevDecision && a.ReEnterSettings == nil {
		return errors.New("reEnterSettings can`t be empty when RepeatPrevDecision = true")
	}

	if a.ReEnterSettings != nil {
		if a.ReEnterSettings.SLA < 1 && a.ReEnterSettings.CheckSLA {
			return fmt.Errorf("invalid reEnterSettings.SLA value %d", a.ReEnterSettings.SLA)
		}
	}
	return nil
}

type FormReEnterSettings struct {
	FormExecutorType FormExecutorType `json:"form_executor_type"`
	FormGroupId      string           `json:"form_group_id"`
	SLA              int              `json:"sla"`
	CheckSLA         bool             `json:"check_sla"`
	Executor         string           `json:"executor"`
}
