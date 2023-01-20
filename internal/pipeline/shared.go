package pipeline

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/google/uuid"
	"github.com/pkg/errors"
)

type UpdateData struct {
	Id   uuid.UUID
	Data interface{}
}

const dotSeparator = "."

func getVariable(variables map[string]interface{}, key string) interface{} {
	variableMemberNames := strings.Split(key, dotSeparator)
	if len(variableMemberNames) <= 2 {
		return variables[key]
	}

	variable, ok := variables[strings.Join(variableMemberNames[:2], dotSeparator)]
	if !ok {
		return nil
	}

	newVariables, ok := variable.(map[string]interface{})
	if !ok {
		return nil
	}

	currK := variableMemberNames[2]
	for i := 2; i < len(variableMemberNames)-1; i++ {
		newVariables, ok = newVariables[currK].(map[string]interface{})
		if !ok {
			return nil
		}
		currK = variableMemberNames[i+1]
	}
	return newVariables[currK]
}

func resolveValuesFromVariables(variableStorage map[string]interface{}, toResolve map[string]struct{}) (
	entitiesToResolve map[string]struct{}, err error) {
	entitiesToResolve = make(map[string]struct{})
	for entityVariableRef := range toResolve {
		if len(strings.Split(entityVariableRef, dotSeparator)) == 1 {
			continue
		}
		entityVar := getVariable(variableStorage, entityVariableRef)

		if entityVar == nil {
			return nil, errors.Wrap(err, "Unable to find entity by variable reference")
		}

		if actualFormExecutorUsername, castOK := entityVar.(string); castOK {
			entitiesToResolve[actualFormExecutorUsername] = toResolve[entityVariableRef]
		}

		return entitiesToResolve, err
	}

	return nil, errors.Wrap(err, "Unexpected behavior")
}

func getSliceFromMapOfStrings(source map[string]struct{}) []string {
	var result = make([]string, 0)

	for key := range source {
		result = append(result, key)
	}

	return result
}

func getStringAddress(s string) *string {
	return &s
}

var typesMapping = map[string]reflect.Kind{
	"array":   reflect.Slice,
	"integer": reflect.Int,
	"string":  reflect.String,
	"number":  reflect.Float64,
	"boolean": reflect.Bool,
}

func checkVariableType(variable interface{}, expectedType string) error {
	goType, ok := typesMapping[expectedType]
	if !ok {
		return fmt.Errorf("unexpected type %v", expectedType)
	}

	if reflect.TypeOf(variable).Kind() != goType {
		return fmt.Errorf("unexpected type of variable %v %T", variable, variable)
	}

	return nil
}
