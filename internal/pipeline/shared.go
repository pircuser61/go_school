package pipeline

import (
	"encoding/json"

	"strings"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	human_tasks "gitlab.services.mts.ru/jocasta/pipeliner/internal/human-tasks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
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

func getDelegates(st *store.VariableStore) human_tasks.Delegations {
	var delegations = make(human_tasks.Delegations, 0)

	if delegationsArr, ok := st.GetArray(script.DelegationsCollection); ok {
		t, err := json.Marshal(delegationsArr)
		if err != nil {
			return nil
		}

		unmarshalErr := json.Unmarshal(t, &delegations)
		if unmarshalErr != nil {
			return nil
		}
	}

	return delegations
}
