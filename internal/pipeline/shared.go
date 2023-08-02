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

	variable, ok := variables[strings.Join(variableMemberNames, dotSeparator)]
	if ok {
		if _, ok = variable.([]interface{}); ok {
			return variable
		}
	}

	variable, ok = variables[strings.Join(variableMemberNames[:2], dotSeparator)]
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

func getUsersFromVars(varStore map[string]interface{}, toResolve map[string]struct{}) (map[string]struct{}, error) {
	res := make(map[string]struct{})
	for varName := range toResolve {
		if len(strings.Split(varName, dotSeparator)) == 1 {
			continue
		}
		varValue := getVariable(varStore, varName)

		if varValue == nil {
			return nil, errors.New("unable to find value by varName: " + varName)
		}

		if login, castOK := varValue.(string); castOK {
			res[login] = toResolve[varName]
		}

		if person, castOk := varValue.(map[string]interface{}); castOk {
			if login, exists := person["username"]; exists {
				if loginString, castOK := login.(string); castOK {
					res[loginString] = toResolve[varName]
				}
			}
		}

		if people, castOk := varValue.([]interface{}); castOk {
			for _, castedPerson := range people {
				if person, ok := castedPerson.(map[string]interface{}); ok {
					if login, exists := person["username"]; exists {
						if loginString, castOK := login.(string); castOK {
							res[loginString] = toResolve[varName]
						}
					}
				}
			}
		}

		return res, nil
	}

	return nil, errors.New("unexpected behavior")
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
