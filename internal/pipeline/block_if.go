package pipeline

import (
	"context"
	"encoding/json"
	"strings"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

const (
	OrLogicalOperator  string = "or"
	AndLogicalOperator string = "and"
)

type IF struct {
	Name          string
	Title         string
	Input         map[string]string
	Output        map[string]string
	FunctionName  string
	FunctionInput map[string]string
	Result        bool
	Nexts         map[string][]string
	State         *ConditionsData

	Storage db.Database
}

type ConditionsData struct {
	Type            script.ConditionType    `json:"type"`
	ConditionGroups []script.ConditionGroup `json:"conditionGroups"`
	ChosenGroupName string                  `json:"-"`
}

func (cd *ConditionsData) GetConditionGroups() []script.ConditionGroup {
	return cd.ConditionGroups
}

func (e *IF) GetStatus() Status {
	return StatusFinished
}

func (e *IF) GetTaskHumanStatus() TaskHumanStatus {
	return ""
}

func (e *IF) GetType() string {
	return BlockGoIf
}

func (e *IF) Next(runCtx *store.VariableStore) ([]string, bool) {
	var chosenGroup = e.State.ChosenGroupName

	if chosenGroup == "" {
		nexts, ok := e.Nexts[DefaultSocket]
		if !ok {
			return nil, false
		}
		return nexts, true
	} else {
		nexts, ok := e.Nexts[chosenGroup]
		if !ok {
			return nil, false
		}
		return nexts, true
	}
}

func (e *IF) Inputs() map[string]string {
	return e.FunctionInput
}

func (e *IF) Outputs() map[string]string {
	return make(map[string]string)
}

func (e *IF) IsScenario() bool {
	return false
}

func (e *IF) DebugRun(ctx context.Context, _ *stepCtx, runCtx *store.VariableStore) error {
	_, s := trace.StartSpan(ctx, "run_if_block")
	defer s.End()

	runCtx.AddStep(e.Name)
	var chosenGroup *script.ConditionGroup

	if e.State != nil {
		conditionGroups := e.State.GetConditionGroups()

		variables, err := getVariables(runCtx)
		if err != nil {
			return err
		}

		chosenGroup = processConditionGroups(conditionGroups, variables)
	}

	var chosenGroupName string

	if chosenGroup != nil {
		chosenGroupName = chosenGroup.Name
	} else {
		chosenGroupName = ""
	}

	e.State.ChosenGroupName = chosenGroupName
	return nil
}

func (e *IF) GetState() interface{} {
	return nil
}

func (e *IF) Update(_ context.Context, _ *script.BlockUpdateData) (interface{}, error) {
	return nil, nil
}

func (e *IF) Model() script.FunctionModel {
	return script.FunctionModel{
		ID:        BlockGoIfID,
		BlockType: script.TypeIF,
		Title:     BlockGoIfTitle,
		Inputs:    nil,
		Outputs:   nil,
		Params: &script.FunctionParams{
			Type: BlockGoIfID,
			Params: &script.ConditionParams{
				Type: "",
			},
		},
		Sockets: []string{DefaultSocket},
	}
}

func createGoIfBlock(name string, ef *entity.EriusFunc) (block *IF, err error) {
	b := &IF{
		Name:   name,
		Title:  ef.Title,
		Input:  map[string]string{},
		Output: map[string]string{},
		Nexts:  ef.Next,
	}

	for _, v := range ef.Input {
		b.Input[v.Name] = v.Global
	}

	for _, v := range ef.Output {
		b.Output[v.Name] = v.Global
	}

	if ef.Params != nil {
		var params script.ConditionParams
		err = json.Unmarshal(ef.Params, &params)
		if err != nil {
			return nil, err
		}

		if err := params.Validate(); err != nil {
			return nil, err
		}

		b.State = &ConditionsData{
			Type:            params.Type,
			ConditionGroups: params.ConditionGroups,
		}
	}

	return b, nil
}

func processConditionGroups(groups []script.ConditionGroup, variables map[string]interface{}) (
	chosenGroup *script.ConditionGroup) {
	for i, conditionGroup := range groups {
		var co = groups[i]
		switch conditionGroup.LogicalOperator {
		case OrLogicalOperator:
			if processOrConditions(conditionGroup.Conditions, variables) {
				chosenGroup = &co
			}
		case AndLogicalOperator:
			if processAndConditions(conditionGroup.Conditions, variables) {
				chosenGroup = &co
			}
		}
	}

	return chosenGroup
}

func processAndConditions(conditions []script.Condition, variables map[string]interface{}) bool {
	var successCount = 0
	for _, condition := range conditions {
		setValuesToCompare(condition.LeftOperand, condition.RightOperand, variables)
		if result, _ := condition.IsTrue(); result {
			successCount++
		}
	}

	return successCount == len(conditions)
}

func processOrConditions(conditions []script.Condition, variables map[string]interface{}) bool {
	for _, condition := range conditions {
		setValuesToCompare(condition.LeftOperand, condition.RightOperand, variables)
		if result, _ := condition.IsTrue(); result {
			return true
		}
	}
	return false
}

func setValuesToCompare(leftOperand, rightOperand script.Operand, variables map[string]interface{}) {
	setOperandValueToCompare(leftOperand, variables)
	setOperandValueToCompare(rightOperand, variables)
}

func setOperandValueToCompare(operand script.Operand, variables map[string]interface{}) {
	switch op := operand.(type) {
	case *script.ValueOperand:
		op.ValueToCompare = op.Value
	case *script.VariableOperand:
		op.ValueToCompare = getVariable(variables, op.VariableRef)
	}
}

func getVariable(variables map[string]interface{}, key string) interface{} {
	return getVariableRecursive(variables, key, key)
}

func getVariableRecursive(variables map[string]interface{}, key string, currentKey string) interface{} {
	const dotSeparator = "."
	var variableMemberNames = strings.Split(currentKey, dotSeparator)

	var variableKey = variableMemberNames[0]
	var variable = variables[variableKey]

	if obj, ok := variable.(map[string]interface{}); ok {
		var firstDotIndex = strings.Index(key, dotSeparator) + 1
		var newKey = key[firstDotIndex:]
		return getVariableRecursive(obj, key, newKey)
	}

	return variable
}

func getVariables(runCtx *store.VariableStore) (result map[string]interface{}, err error) {
	variables, err := runCtx.GrabStorage()
	if err != nil {
		return nil, err
	}

	return variables, nil
}
