package script

import "errors"

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
	Name    string       `json:"name"`
	Version string       `json:"version"`
	Mapping MappingParam `json:"mapping"`
}

func (a *ExecutableFunctionParams) Validate() error {
	if a.Name == "" || a.Version == "" {
		return errors.New("got no function name or version")
	}

	err := a.validateMapping(a.Mapping)
	if err != nil {
		return err
	}

	return nil
}

func (a *ExecutableFunctionParams) validateMapping(mappingParam MappingParam) error {
	if mappingParam != nil {
		for _, mappingValue := range mappingParam {
			if mappingValue.Type == "" || mappingValue.Description == "" {
				return errors.New("type and description are required")
			}

			err := a.validateMapping(mappingValue.Properties)
			if err != nil {
				return err
			}

			for _, item := range mappingValue.Items {
				err := a.validateMapping(item)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}
