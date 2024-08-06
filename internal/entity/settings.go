package entity

import (
	"encoding/json"
	"fmt"
	"time"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

type UpdateApprovalListSettings struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Steps []string `json:"steps"`

	ContextMapping script.JSONSchemaProperties `json:"context_mapping"`
	FormsMapping   script.JSONSchemaProperties `json:"forms_mapping"`
}

type ApprovalListSettings struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Steps     []string  `json:"steps"`
	CreatedAt time.Time `json:"created_at"`

	ContextMapping script.JSONSchemaProperties `json:"context_mapping"`
	FormsMapping   script.JSONSchemaProperties `json:"forms_mapping"`
}

type SaveApprovalListSettings struct {
	VersionID string   `json:"version_id"`
	Name      string   `json:"name"`
	Steps     []string `json:"steps"`

	ContextMapping script.JSONSchemaProperties `json:"context_mapping"`
	FormsMapping   script.JSONSchemaProperties `json:"forms_mapping"`
}

type NodeSubscriptionEvents struct {
	NodeID string   `json:"node_id"`
	Notify bool     `json:"notify"`
	Events []string `json:"events"`
}

type ExternalSystemSubscriptionParams struct {
	SystemID           string                      `json:"system_id"`
	MicroserviceID     string                      `json:"microservice_id"`
	Path               string                      `json:"path"`
	Method             string                      `json:"method"`
	NotificationSchema script.JSONSchema           `json:"notification_schema"`
	Mapping            script.JSONSchemaProperties `json:"mapping"`
	Nodes              []NodeSubscriptionEvents    `json:"nodes"`
}

type ProcessSettingsWithExternalSystems struct {
	ExternalSystems    []ExternalSystem                   `json:"external_systems"`
	ProcessSettings    ProcessSettings                    `json:"process_settings"`
	TasksSubscriptions []ExternalSystemSubscriptionParams `json:"tasks_subscriptions"`
	ApprovalLists      []ApprovalListSettings             `json:"approval_lists"`
}

type ProcessSettings struct {
	VersionID          string             `json:"version_id"`
	StartSchema        *script.JSONSchema `json:"start_schema"`
	EndSchema          *script.JSONSchema `json:"end_schema"`
	ResubmissionPeriod int                `json:"resubmission_period"`
	Name               string             `json:"name"`
	SLA                int                `json:"sla"`
	WorkType           string             `json:"work_type"`

	StartSchemaRaw []byte `json:"-"`
	EndSchemaRaw   []byte `json:"-"`
}

func (ps *ProcessSettings) UnmarshalJSON(bytes []byte) error {
	temp := struct {
		ID                 string           `json:"version_id"`
		StartSchema        *json.RawMessage `json:"start_schema"`
		EndSchema          *json.RawMessage `json:"end_schema"`
		ResubmissionPeriod int              `json:"resubmission_period"`
		Name               string           `json:"name"`
		SLA                int              `json:"sla"`
		WorkType           string           `json:"work_type"`
	}{}

	if err := json.Unmarshal(bytes, &temp); err != nil {
		return err
	}

	ps.VersionID = temp.ID
	ps.ResubmissionPeriod = temp.ResubmissionPeriod
	ps.Name = temp.Name
	ps.SLA = temp.SLA
	ps.WorkType = temp.WorkType

	if temp.StartSchema != nil {
		ps.StartSchemaRaw = *temp.StartSchema
	}

	if temp.EndSchema != nil {
		ps.EndSchemaRaw = *temp.EndSchema
	}

	return nil
}

func (ps *ProcessSettings) ValidateSLA() bool {
	if (ps.WorkType == "8/5" || ps.WorkType == "24/7" || ps.WorkType == "12/5") && ps.SLA > 0 {
		return true
	}

	return false
}

type ExternalSystem struct {
	ID   string `json:"system_id"`
	Name string `json:"name,omitempty"`

	InputSchema   *script.JSONSchema `json:"input_schema,omitempty"`
	OutputSchema  *script.JSONSchema `json:"output_schema,omitempty"`
	InputMapping  *script.JSONSchema `json:"input_mapping,omitempty"`
	OutputMapping *script.JSONSchema `json:"output_mapping,omitempty"`

	OutputSettings *EndSystemSettings `json:"output_settings,omitempty"`

	AllowRunAsOthers bool `json:"allow_run_as_others"`
}

type EndSystemSettings struct {
	URL            string `json:"URL"`
	Method         string `json:"method"`
	MicroserviceID string `json:"microservice_id"`
}

type SLAVersionSettings struct {
	Author   string `json:"author"`
	WorkType string `json:"work_type"`
	SLA      int    `json:"sla"`
}

type EndProcessData struct {
	ID         string `json:"id"`
	VersionID  string `json:"version_id"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at"`
	Status     string `json:"status"`
}

func (ps *ProcessSettings) Validate() error {
	err := ps.StartSchema.Validate()
	if err != nil {
		return err
	}

	err = ps.EndSchema.Validate()
	if err != nil {
		return err
	}

	return nil
}

func (es *ExternalSystem) ValidateSchemas() error {
	err := es.InputSchema.Validate()
	if err != nil {
		return err
	}

	err = es.OutputSchema.Validate()
	if err != nil {
		return err
	}

	err = es.InputMapping.Validate()
	if err != nil {
		return err
	}

	err = es.OutputMapping.Validate()
	if err != nil {
		return err
	}

	err = es.ValidateInputMapping()
	if err != nil {
		return err
	}

	return nil
}

func (es *ExternalSystem) ValidateInputMapping() error {
	if es.InputMapping == nil {
		return nil
	}

	mappedSet := make(map[string]struct{})
	requireds := es.InputMapping.Required

	for k := range es.InputMapping.Properties {
		if es.InputMapping.Properties[k].Value != "" || es.InputMapping.Properties[k].Default != nil {
			mappedSet[k] = struct{}{}
		}

		if es.InputMapping.Properties[k].Type == utils.ObjectType {
			fillMappedSet(es.InputMapping.Properties[k], mappedSet, &requireds, k)
		}
	}

	for _, k := range requireds {
		if _, ok := mappedSet[k]; !ok {
			return fmt.Errorf("%w: %s", ErrMappingRequired, k)
		}
	}

	return nil
}

//nolint:gocritic //Нельзя передавать как указатель - значение находится в map
func fillMappedSet(
	obj script.JSONSchemaPropertiesValue,
	mappedSet map[string]struct{},
	requireds *[]string,
	keyPath string,
) {
	// Было: ['a', 'b'] Стало: ['obj.a', 'obj.b'] — Чтобы отличать дублирующиеся ключи
	if obj.Required != nil {
		tempRequireds := make([]string, len(obj.Required))

		for i := range obj.Required {
			tempRequireds[i] = keyPath + "." + obj.Required[i]
		}

		*requireds = append(*requireds, tempRequireds...)
	}

	_, isParentMapped := mappedSet[keyPath]

	// Счетчик для проверки, что все поля в объекте смаплены
	var mappedCounter int

	for k := range obj.Properties {
		if obj.Properties[k].Value != "" || obj.Properties[k].Default != nil || isParentMapped {
			mappedSet[keyPath+"."+k] = struct{}{}

			mappedCounter++
			if mappedCounter == len(obj.Properties) {
				mappedSet[keyPath] = struct{}{}
			}
		}

		if obj.Properties[k].Type == utils.ObjectType {
			fillMappedSet(obj.Properties[k], mappedSet, requireds, keyPath+k)
		}
	}
}
