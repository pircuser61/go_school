package pipeline

import (
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
