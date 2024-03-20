package script

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/xeipuuv/gojsonschema"
)

const dotSeparator = "."

func RestoreMapStructure(variables map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for name, variable := range variables {
		keyParts := strings.Split(name, ".")
		current := result

		for i, keyPart := range keyParts {
			if i == len(keyParts)-1 {
				current[keyPart] = variable
			} else {
				if _, ok := current[keyPart]; !ok {
					current[keyPart] = make(map[string]interface{})
				}

				current = current[keyPart].(map[string]interface{})
			}
		}
	}

	return result
}

func MapData(
	mapping JSONSchemaProperties,
	input map[string]interface{},
	required []string,
) (map[string]interface{}, error) {
	mappingProperties := MakeMappingProperties(mapping, input, required)

	return mappingProperties.Map()
}

func getVariable(input map[string]interface{}, path []string) (interface{}, error) {
	if len(path) == 0 {
		return nil, errors.New("invalid path to variable")
	}

	param, ok := input[path[0]]
	if !ok {
		return nil, nil
	}

	if len(path) == 1 {
		return param, nil
	}

	objectProperties, ok := param.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid path to variable %s", path)
	}

	path = path[1:]

	variable, err := getVariable(objectProperties, path)
	if err != nil {
		return nil, err
	}

	return variable, nil
}

func ValidateParam(param interface{}, paramJSONSchema *JSONSchemaPropertiesValue) error {
	marshaledParam, err := json.Marshal(param)
	if err != nil {
		return err
	}

	marshaledJSONSchema, err := json.Marshal(paramJSONSchema)
	if err != nil {
		return err
	}

	err = ValidateJSONByJSONSchema(string(marshaledParam), string(marshaledJSONSchema))
	if err != nil {
		return err
	}

	return nil
}

func ValidateJSONByJSONSchema(jsonString, jsonSchema string) error {
	jsonSchema = strings.Replace(jsonSchema, `"format":"date"`, `"format":""`, -1)  // TODO: delete this string
	jsonSchema = strings.Replace(jsonSchema, `"format": "date"`, `"format":""`, -1) // TODO: delete this string

	loader := gojsonschema.NewStringLoader(jsonSchema)

	schema, err := gojsonschema.NewSchema(loader)
	if err != nil {
		return err
	}

	documentLoader := gojsonschema.NewStringLoader(jsonString)

	result, err := schema.Validate(documentLoader)
	if err != nil {
		return err
	}

	if !result.Valid() {
		var errorMsg string
		for _, resultError := range result.Errors() {
			errorMsg += resultError.String() + "; "
		}

		return errors.New(errorMsg)
	}

	return nil
}
