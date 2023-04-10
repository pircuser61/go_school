package script

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const (
	timeLayout  = `"2006-01-02 15:04:05.000000 -0700 MST"`
	timeLayout2 = `"2006-01-02 15:04:05.000 -0700 MST"`
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

	Format     string               `json:"format,omitempty"`
	Default    interface{}          `json:"default,omitempty"`
	Required   []string             `json:"required,omitempty"`
	Items      *ArrayItems          `json:"items,omitempty"`
	Properties JSONSchemaProperties `json:"properties,omitempty"`

	Value string `json:"value,omitempty"`
}

type ArrayItems struct {
	Items      *ArrayItems          `json:"items,omitempty"`
	Properties JSONSchemaProperties `json:"properties,omitempty"`
	Type       string               `json:"type,omitempty"`
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
	Name           string               `json:"name"`
	Version        string               `json:"version"`
	Mapping        JSONSchemaProperties `json:"mapping"`
	Function       FunctionParam        `json:"function"`
	WaitCorrectRes int                  `json:"waitCorrectRes"`
}

type FunctionParam struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	FunctionId  string       `json:"functionId"`
	VersionId   string       `json:"versionId"`
	Version     string       `json:"version"`
	CreatedAt   functionTime `json:"createdAt"`
	DeletedAt   functionTime `json:"deletedAt"`
	Uses        int          `json:"uses"`
	Input       string       `json:"input"`
	Output      string       `json:"output"`
	Options     string       `json:"options"`
}

type functionTime time.Time

type ParamMetadata struct {
	Type        string
	Description string
	Items       []ParamMetadata
	Properties  map[string]ParamMetadata
}

func (p ParamMetadata) GetType() string {
	return p.Type
}

func (p ParamMetadata) GetProperties() map[string]interface{} {
	properties := make(map[string]interface{})
	for k, v := range p.Properties {
		properties[k] = v
	}
	return properties
}

func (p ParamMetadata) GetItems() []interface{} {
	items := make([]interface{}, 0)
	for _, v := range p.Items {
		items = append(items, v)
	}
	return items
}

type Options struct {
	Type   string
	Input  map[string]interface{}
	Output map[string]ParamMetadata
}

func (a *ExecutableFunctionParams) Validate() error {
	if a.Name == "" || a.Version == "" {
		return errors.New("got no function name or version")
	}

	err := a.Mapping.Validate()
	if err != nil {
		return err
	}

	return nil
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
	for name, property := range properties {
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

		for _, name := range property.Required {
			_, ok := property.Properties[name]
			if !ok {
				return fmt.Errorf("%s is required", name)
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
		} else {
			err := ai.Items.Validate()
			if err != nil {
				return err
			}
		}
	}

	if ai.Type == object {
		if ai.Properties == nil {
			return errors.New("properties is required")
		} else {
			err := ai.Properties.Validate()
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (ft *functionTime) UnmarshalJSON(b []byte) error {
	if len(b) == len(emptyString) {
		return nil
	}

	parsedTime, err := time.Parse(timeLayout, string(b))
	if err == nil {
		*ft = functionTime(parsedTime)
		return nil
	}

	parsedTime, err = time.Parse(timeLayout2, string(b))
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
