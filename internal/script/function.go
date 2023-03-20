package script

import (
	"encoding/json"
	"errors"
	"time"
)

const (
	timeLayout  = `"2006-01-02 15:04:05.000000 -0700 MST"`
	emptyString = `""`
)

type MappingParam map[string]MappingValue

type MappingValue struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type"`

	Format     string         `json:"format,omitempty"`
	Default    interface{}    `json:"default,omitempty"`
	Required   bool           `json:"required,omitempty"`
	Items      []MappingParam `json:"items,omitempty"`
	Properties MappingParam   `json:"properties,omitempty"`

	Value string `json:"value,omitempty"`
}

func (m *MappingValue) GetType() string {
	return m.Type
}

func (m *MappingValue) GetProperties() map[string]interface{} {
	properties := make(map[string]interface{})

	for k := range m.Properties {
		properties[k] = m.Properties[k]
	}
	return properties
}

func (m *MappingValue) GetItems() []interface{} {
	items := make([]interface{}, 0)
	for _, v := range m.Items {
		items = append(items, v)
	}
	return items
}

type ExecutableFunctionParams struct {
	Name     string        `json:"name"`
	Version  string        `json:"version"`
	Mapping  MappingParam  `json:"mapping"`
	Function FunctionParam `json:"function"`
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

func (m MappingParam) Validate() error {
	for key := range m {
		mappingValue := m[key]
		if mappingValue.Type == "" || mappingValue.Description == "" {
			return errors.New("type and description are required")
		}

		err := mappingValue.Properties.Validate()
		if err != nil {
			return err
		}

		for _, item := range mappingValue.Items {
			err = item.Validate()
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
	if err != nil {
		err = json.Unmarshal(b, &parsedTime)
		if err != nil {
			return err
		}
	}

	*ft = functionTime(parsedTime)

	return nil
}

func (ft functionTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(ft))
}
