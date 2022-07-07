package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"go.opencensus.io/trace"
	"strings"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

const (
	keyIf            string = "check"
	groupDefaultName string = "group"
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
}

func (cd *ConditionsData) GetConditionGroups() []script.ConditionGroup {
	for i := range cd.ConditionGroups {
		cd.ConditionGroups[i].Alias = fmt.Sprintf("%s-%v", groupDefaultName, i)
	}
	return cd.ConditionGroups
}

func (e *IF) GetStatus() Status {
	return StatusFinished
}

func (e *IF) GetTaskHumanStatus() TaskHumanStatus {
	return ""
}

func (e *IF) GetType() string {
	return BlockInternalIf
}

func (e *IF) Next(runCtx *store.VariableStore) ([]string, bool) {
	cg, ok := runCtx.GetValue("chosenGroup")
	if ok {
		var chosenGroup = cg.(string)

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

	return nil, false
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

func (e *IF) Run(ctx context.Context, runCtx *store.VariableStore) error {
	return e.DebugRun(ctx, runCtx)
}

func (e *IF) DebugRun(ctx context.Context, runCtx *store.VariableStore) error {
	_, s := trace.StartSpan(ctx, "run_if_block")
	defer s.End()

	runCtx.AddStep(e.Name)

	variables, err := runCtx.GrabStorage()
	if err != nil {
		return nil
	}

	conditionGroups := e.State.GetConditionGroups()

	var chosenGroup = processConditions(conditionGroups, variables)
	runCtx.SetValue("chosenGroup", chosenGroup.Alias)

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

func createGoIfBlock(name string, ef *entity.EriusFunc) *IF {
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

	var params script.ConditionParams
	err := json.Unmarshal(ef.Params, &params)
	if err != nil {
		return nil
	}

	if err = params.Validate(); err != nil {
		return nil
	}

	b.State = &ConditionsData{
		Type:            params.Type,
		ConditionGroups: params.ConditionGroups,
	}

	for _, cg := range b.State.ConditionGroups {
		cg.PrepareOperands()
	}

	return b
}

func processConditions(groups []script.ConditionGroup, variables map[string]interface{}) (
	chosenGroup *script.ConditionGroup) {
	for _, conditionGroup := range groups {
		//todo: handle all of also
		if processAnyOf(conditionGroup.AnyOf, variables) {
			chosenGroup = &conditionGroup
		}
	}

	return chosenGroup
}

func processAnyOf(conditions []script.Condition, variables map[string]interface{}) bool {
	for _, condition := range conditions {
		checkForReferenceVariables(condition.LeftOperand, condition.RightOperand, variables)
		if condition.IsTrue() {
			return true
		}
	}
	return false
}

func processAllOf(allOfConditions []script.Condition, variables map[string]interface{}) bool {
	var validConditionsCount = 0
	for _, condition := range allOfConditions {
		checkForReferenceVariables(condition.LeftOperand, condition.RightOperand, variables)
		if condition.IsTrue() {
			validConditionsCount++
		}
	}

	return validConditionsCount == len(allOfConditions)
}

func checkForReferenceVariables(leftOperand, rightOperand script.Operand, variables map[string]interface{}) (
	leftOperandValue, rightOperandValue interface{}) {
	var leftVariableReference = tryGetVariableReference(leftOperand.Value)

	if leftVariableReference != "" {
		var leftOperandVariable = variables[leftVariableReference]
		leftOperand.Value = leftOperandVariable
	}

	var rightVariableReference = tryGetVariableReference(rightOperand.Value)

	if rightVariableReference != "" {
		var rightOperandVariable = variables[rightVariableReference]
		rightOperand.Value = rightOperandVariable
	}

	return leftOperand.Value, rightOperand.Value
}

func tryGetVariableReference(value interface{}) string {
	const (
		referencePrefix = "ref#"
		empty           = ""
	)

	if val, ok := value.(string); ok {
		if strings.HasPrefix(val, referencePrefix) {
			var variableName = strings.Replace(val, referencePrefix, empty, 1)
			return variableName
		}
	}

	return empty
}
