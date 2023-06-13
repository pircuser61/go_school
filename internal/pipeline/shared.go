package pipeline

import (
	"encoding/json"
	"reflect"
	"strings"

	"github.com/google/uuid"
	"github.com/iancoleman/orderedmap"
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
		newVariables = structToMap(variable)
		if newVariables == nil {
			return nil
		}
	}

	currK := variableMemberNames[2]
	for i := 2; i < len(variableMemberNames)-1; i++ {
		newVariables, ok = newVariables[currK].(map[string]interface{})
		if !ok {
			newVariables = structToMap(variable)
			if newVariables == nil {
				return nil
			}
		}
		currK = variableMemberNames[i+1]
	}
	return newVariables[currK]
}

func resolveValuesFromVariables(variableStorage map[string]interface{}, toResolve map[string]struct{}) (
	map[string]struct{}, error) {
	entitiesToResolve := make(map[string]struct{})
	for entityVariableRef := range toResolve {
		if len(strings.Split(entityVariableRef, dotSeparator)) == 1 {
			continue
		}
		entityVar := getVariable(variableStorage, entityVariableRef)

		if entityVar == nil {
			return nil, errors.New("Unable to find entity by variable reference")
		}

		if actualFormExecutorUsername, castOK := entityVar.(string); castOK {
			entitiesToResolve[actualFormExecutorUsername] = toResolve[entityVariableRef]
		}

		return entitiesToResolve, nil
	}

	return nil, errors.New("Unexpected behavior")
}

func getSliceFromMapOfStrings(source map[string]struct{}) []string {
	var result = make([]string, 0)

	for key := range source {
		result = append(result, key)
	}

	return result
}

// nolint:deadcode,unused //used in tests
func getStringAddress(s string) *string {
	return &s
}

func getRecipientFromState(applicationBody *orderedmap.OrderedMap) string {
	if applicationBody == nil {
		return ""
	}

	var login string
	if recipientValue, ok := applicationBody.Get("recipient"); ok {
		if recipient, ok := recipientValue.(orderedmap.OrderedMap); ok {
			if usernameValue, ok := recipient.Get("username"); ok {
				if username, ok := usernameValue.(string); ok {
					login = username
				}
			}
		}
	}

	return login
}

func structToMap(variable interface{}) map[string]interface{} {
	variableType := reflect.TypeOf(variable)
	if !(variableType.Kind() == reflect.Struct ||
		(variableType.Kind() == reflect.Pointer && variableType.Elem().Kind() == reflect.Struct)) {
		return nil
	}

	bytes, err := json.Marshal(variable)
	if err != nil {
		return nil
	}

	res := make(map[string]interface{})
	if unmErr := json.Unmarshal(bytes, &res); unmErr != nil {
		return nil
	}

	return res
}
