package script

import (
	"errors"
	"time"
)

type MappingParam map[string]MappingValue

type MappingValue struct {
	Description string `json:"description"`
	Type        string `json:"type"`

	Format     string         `json:"format,omitempty"`
	Items      []MappingParam `json:"items,omitempty"`
	Properties MappingParam   `json:"properties,omitempty"`

	Value string `json:"value,omitempty"`
}

type ExecutableFunctionParams struct {
	Name     string        `json:"name"`
	Version  string        `json:"version"`
	Mapping  MappingParam  `json:"mapping"`
	Function FunctionParam `json:"function"`
}

type FunctionParam struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	FunctionId  string    `json:"functionId"`
	VersionId   string    `json:"versionId"`
	Version     string    `json:"version"`
	CreatedAt   time.Time `json:"createdAt"`
	DeletedAt   time.Time `json:"deletedAt"`
	Uses        int       `json:"uses"`
	Input       string    `json:"input"`
	Output      string    `json:"output"`
	Options     string    `json:"options"`
}

type ParamMetadata struct {
	Type        string
	Description string
	Items       []ParamMetadata
	Properties  map[string]ParamMetadata
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
	for _, mappingValue := range m {
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
