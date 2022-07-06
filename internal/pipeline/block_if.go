package pipeline

import (
	"context"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"go.opencensus.io/trace"
	"strings"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

const (
	keyIf string = "check"
)

type IF struct {
	Name          string
	FunctionName  string
	FunctionInput map[string]string
	Result        bool
	Nexts         map[string][]string
	State         *ConditionsData

	Storage db.Database
}

type ConditionsData struct {
	Type            script.ConditionType     `json:"type"`
	ConditionGroups *[]script.ConditionGroup `json:"conditionGroups"`
}

func (cd *ConditionsData) GetConditionGroups() *[]script.ConditionGroup {
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

	if chosenGroup, ok := runCtx.GetValue("chosenGroup"); ok {

	}

	r, err := runCtx.GetBoolWithInput(e.FunctionInput, keyIf)
	if err != nil {
		return []string{}, false
	}

	if r {
		nexts, ok := e.Nexts[trueSocket]
		if !ok {
			return nil, false
		}
		return nexts, true
	}

	nexts, ok := e.Nexts[falseSocket]
	if !ok {
		return nil, false
	}
	return nexts, true
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

	r, err := runCtx.GetBoolWithInput(e.FunctionInput, keyIf)
	if err != nil {
		return err
	}

	e.Result = r

	variables, err := runCtx.GrabStorage()
	conditionGroups := *e.State.GetConditionGroups()

	var chosenGroup = processConditions(conditionGroups, variables)
	runCtx.SetValue("chosenGroup", chosenGroup)

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
		NextFuncs: []string{script.Next},
	}
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

func checkForReferenceVariables(leftOperand, rightOperand script.Operand, variables map[string]interface{}) (leftOperandValue, rightOperandValue interface{}) {
	var leftOperandVariableReference = tryGetVariableReference(leftOperand.Value)

	if leftOperandVariableReference != "" {
		var leftOperandVariable = variables[leftOperandVariableReference]
		leftOperand.Value = leftOperandVariable
	}

	var rightOperandVariableReference = tryGetVariableReference(rightOperand.Value)

	if rightOperandVariableReference != "" {
		var rightOperandVariable = variables[rightOperandVariableReference]
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
