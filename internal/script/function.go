package script

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/araddon/dateparse"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/functions"
)

const (
	timeLayout  = `"2006-01-02 15:04:05.000000 -0700 MST"`
	timeLayout2 = `"2006-01-02 15:04:05.000 -0700 MST"`
	timeLayout3 = `"2006-01-02 15:04:05.00000 -0700 MST"`
	timeLayout4 = `"2006-01-02 15:04:05.0000 -0700 MST"`
	emptyString = `""`
	object      = "object"
)

type JSONSchema struct {
	Type       string               `json:"type"`
	Properties JSONSchemaProperties `json:"properties"`
	Required   []string             `json:"required,omitempty"`
}

type JSONSchemaProperties map[string]JSONSchemaPropertiesValue

type JSONSchemaPropertiesValue struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type"`
	Global      string `json:"global,omitempty"`

	Format      string               `json:"format,omitempty"`
	Default     interface{}          `json:"default,omitempty"`
	Required    []string             `json:"required,omitempty"`
	Items       *ArrayItems          `json:"items,omitempty"`
	Properties  JSONSchemaProperties `json:"properties,omitempty"`
	FieldHidden bool                 `json:"fieldHidden,omitempty"`

	Value string `json:"value,omitempty"`
}

// nolint:gocritic // need for json marshaling only struct
func (jspv JSONSchemaPropertiesValue) MarshalJSON() ([]byte, error) {
	dataToMarshal := make(map[string]interface{})

	for i := 0; i < reflect.ValueOf(jspv).NumField(); i++ {
		field := reflect.TypeOf(jspv).Field(i)
		value := reflect.ValueOf(jspv).Field(i)
		// handle Properties being not null but an empty struct (omitempty omits both cases)
		if strings.HasSuffix(field.Tag.Get("json"), "omitempty") && value.IsZero() {
			continue
		}

		dataToMarshal[strings.Replace(field.Tag.Get("json"), ",omitempty", "", 1)] = value.Interface()
	}

	return json.Marshal(dataToMarshal)
}

type ArrayItems struct {
	Items      *ArrayItems          `json:"items,omitempty"`
	Properties JSONSchemaProperties `json:"properties,omitempty"`
	Type       string               `json:"type,omitempty"`
	Format     string               `json:"format,omitempty"`
}

func (jspv *JSONSchemaPropertiesValue) GetType() string {
	return jspv.Type
}

func (jspv *JSONSchemaPropertiesValue) GetProperties() map[string]interface{} {
	properties := make(map[string]interface{})

	for k := range jspv.Properties {
		properties[k] = jspv.Properties[k]
	}

	return properties
}

type ExecutableFunctionParams struct {
	Name           string                 `json:"name"`
	Version        string                 `json:"version"`
	Mapping        JSONSchemaProperties   `json:"mapping"`
	Function       FunctionParam          `json:"function"`
	WaitCorrectRes int                    `json:"waitCorrectRes"`
	Constants      map[string]interface{} `json:"constants"`
	CheckSLA       bool                   `json:"check_sla"`
	SLA            int                    `json:"sla"` // seconds
}

type FunctionParam struct {
	Name          string       `json:"name"`
	Description   string       `json:"description"`
	FunctionID    string       `json:"functionId"`
	VersionID     string       `json:"versionId"`
	Version       string       `json:"version"`
	CreatedAt     functionTime `json:"createdAt"`
	DeletedAt     functionTime `json:"deletedAt"`
	Uses          int          `json:"uses"`
	Input         string       `json:"input"`
	RequiredInput []string     `json:"requiredInput"`
	Output        string       `json:"output"`
	Options       string       `json:"options"`
}

type functionTime time.Time

type ParamMetadata struct {
	Type        string
	Description string
	Items       *ParamMetadata
	Properties  map[string]ParamMetadata
}

func (p ParamMetadata) GetType() string {
	return p.Type
}

func (p ParamMetadata) GetProperties() map[string]interface{} {
	properties := make(map[string]interface{}, len(p.Properties))
	for k, v := range p.Properties {
		properties[k] = v
	}

	return properties
}

func (a *ExecutableFunctionParams) Validate() error {
	if a.Name == "" || a.Version == "" {
		return errors.New("got no function name or version")
	}

	err := a.Mapping.Validate()
	if err != nil {
		return err
	}

	if slaErr := a.validateSLA(); slaErr != nil {
		return slaErr
	}

	return nil
}

func (a *ExecutableFunctionParams) validateSLA() error {
	funcType, err := a.getFuncType()
	if err != nil {
		return err
	}

	switch funcType {
	case functions.SyncFlag:
		if a.SLA > int(60*time.Minute.Seconds()+59*time.Second.Seconds()) {
			return errors.New("sync function SLA is too long")
		}
	case functions.AsyncFlag:
		if a.SLA > int(365*24*time.Hour.Seconds()+23*time.Hour.Seconds()+59*time.Minute.Seconds()) {
			return errors.New("async function SLA is too long")
		}
	}

	return nil
}

func (a *ExecutableFunctionParams) getFuncType() (string, error) {
	validBody := strings.Replace(a.Function.Options, "\\", "", -1)

	options := struct {
		Type string `json:"type"`
	}{}

	if err := json.Unmarshal([]byte(validBody), &options); err != nil {
		return "", fmt.Errorf("cannot unmarshal function options: %w", err)
	}

	if options.Type != functions.SyncFlag && options.Type != functions.AsyncFlag {
		return "", errors.New("invalid function type")
	}

	return options.Type, nil
}

func (js *JSONSchema) Validate() error {
	if js == nil {
		return nil
	}

	if js.Type != object {
		return errors.New(`schema type must be "object"`)
	}

	for _, s := range js.Required {
		_, ok := js.Properties[s]
		if !ok {
			return fmt.Errorf("%s is required", s)
		}
	}

	err := js.Properties.Validate()
	if err != nil {
		return err
	}

	return nil
}

func (properties JSONSchemaProperties) Validate() error {
	for name := range properties {
		property := properties[name]

		if property.Type == "" {
			return errors.New("type is required")
		}

		err := property.Properties.Validate()
		if err != nil {
			return err
		}

		if property.Items != nil {
			err = property.Items.Validate()
			if err != nil {
				return err
			}
		}

		for _, requiredName := range property.Required {
			_, ok := property.Properties[requiredName]
			if !ok {
				return fmt.Errorf("%s is required", requiredName)
			}
		}

		if property.Value != "" {
			hasInnerMapping := property.Properties.checkInnerFieldsHasMapping()
			if hasInnerMapping {
				return fmt.Errorf("object %s must either be mapped or must have inner field mappings, but not both", name)
			}
		}
	}

	return nil
}

//nolint:gocritic //в этом проекте не принято использовать поинтеры в коллекциях
func (properties JSONSchemaProperties) checkInnerFieldsHasMapping() bool {
	for _, property := range properties {
		if property.Value != "" {
			return true
		}

		hasInnerMapping := property.Properties.checkInnerFieldsHasMapping()
		if hasInnerMapping {
			return true
		}
	}

	return false
}

func (ai ArrayItems) Validate() error {
	if ai.Type == "" {
		return errors.New("type is required")
	}

	if ai.Type == "array" {
		if ai.Items == nil {
			return errors.New("items is required")
		}

		err := ai.Items.Validate()
		if err != nil {
			return err
		}
	}

	if ai.Type == object {
		if ai.Properties == nil {
			return errors.New("properties is required")
		}

		err := ai.Properties.Validate()
		if err != nil {
			return err
		}
	}

	return nil
}

func (ft *functionTime) UnmarshalJSON(b []byte) error {
	if len(b) == len(emptyString) {
		return nil
	}

	parsedTime, err := dateparse.ParseLocal(string(b))
	if err == nil {
		*ft = functionTime(parsedTime)

		return nil
	}

	parsedTime, err = time.Parse(timeLayout, string(b))
	if err == nil {
		*ft = functionTime(parsedTime)

		return nil
	}

	parsedTime, err = time.Parse(timeLayout2, string(b))
	if err == nil {
		*ft = functionTime(parsedTime)

		return nil
	}

	parsedTime, err = time.Parse(timeLayout3, string(b))
	if err == nil {
		*ft = functionTime(parsedTime)

		return nil
	}

	parsedTime, err = time.Parse(timeLayout4, string(b))
	if err == nil {
		*ft = functionTime(parsedTime)

		return nil
	}

	err = json.Unmarshal(b, &parsedTime)
	if err != nil {
		return err
	}

	*ft = functionTime(parsedTime)

	return nil
}

func (ft functionTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(ft))
}

type VersionsByFunction struct {
	Name   string
	Link   string
	Status int
}
