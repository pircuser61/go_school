package script

import (
	"fmt"
	"strings"

	"gitlab.services.mts.ru/jocasta/human-tasks/pkg/utils/slice"
)

type MappingProperties struct {
	mapping  JSONSchemaProperties
	input    map[string]interface{}
	required []string

	mappedData map[string]interface{}
}

type param struct {
	name    string
	mapping JSONSchemaPropertiesValue
}

func MakeMappingProperties(
	mapping JSONSchemaProperties,
	input map[string]interface{},
	required []string,
) MappingProperties {
	return MappingProperties{
		mapping:  mapping,
		input:    input,
		required: required,

		mappedData: make(map[string]interface{}, len(input)),
	}
}

//nolint:gocritic //тут уже пришла коллекция без поинтеров так что МЯУ
func (p *MappingProperties) Map() (map[string]interface{}, error) {
	for paramName, paramMapping := range p.mapping {
		mapParam := param{
			name:    paramName,
			mapping: paramMapping,
		}

		err := p.mapParam(&mapParam)
		if err != nil {
			return nil, fmt.Errorf("failed map %s param, err: %w", paramName, err)
		}
	}

	return p.mappedData, nil
}

func (p *MappingProperties) mapParam(param *param) error {
	if param.mapping.Value == "" {
		err := p.mapEmptyValueParam(param)
		if err != nil {
			return err
		}

		return nil
	}

	err := p.mapNotEmptyValueParam(param)
	if err != nil {
		return err
	}

	return nil
}

func (p *MappingProperties) mapEmptyValueParam(param *param) error {
	if param.mapping.Type == object {
		err := p.mapObjectTypeParam(param)
		if err != nil {
			return err
		}

		return nil
	}

	if p.requiredContains(param.name) {
		return fmt.Errorf("%s is required, but mapping value is empty", param.name)
	}

	if param.mapping.Default != nil {
		err := validateParam(param.mapping.Default, &param.mapping)
		if err != nil {
			return err
		}

		p.mappedData[param.name] = param.mapping.Default
	}

	return nil
}

func (p *MappingProperties) mapObjectTypeParam(param *param) error {
	mappingProperties := MakeMappingProperties(param.mapping.Properties, p.input, param.mapping.Required)

	variable, err := mappingProperties.Map()
	if err != nil {
		return err
	}

	err = validateParam(variable, &param.mapping)
	if err != nil {
		return err
	}

	p.mappedData[param.name] = variable

	return nil
}

func (p *MappingProperties) mapNotEmptyValueParam(param *param) error {
	path := strings.Split(param.mapping.Value, dotSeparator)

	variable, err := getVariable(p.input, path)
	if err != nil {
		return err
	}

	if variable != nil {
		err = validateParam(variable, &param.mapping)
		if err != nil {
			return err
		}

		p.mappedData[param.name] = variable

		return nil
	}

	if p.requiredContains(param.name) {
		return fmt.Errorf("%s is required, but mapping value is empty", param.name)
	}

	if param.mapping.Default != nil {
		err = validateParam(param.mapping.Default, &param.mapping)
		if err != nil {
			return err
		}

		p.mappedData[param.name] = param.mapping.Default
	}

	return nil
}

func (p *MappingProperties) requiredContains(paramName string) bool {
	return slice.Contains(p.required, paramName)
}
