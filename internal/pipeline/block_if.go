package pipeline

import (
	"context"
	"encoding/json"

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
	Sockets       []script.Socket
	State         *ConditionsData

	RunContext *BlockRunContext
}

type ConditionsData struct {
	Type            script.ConditionType    `json:"type"`
	ConditionGroups []script.ConditionGroup `json:"conditionGroups"`
	ChosenGroupID   string                  `json:"-"`
}

func (cd *ConditionsData) GetConditionGroups() []script.ConditionGroup {
	return cd.ConditionGroups
}

func (e *IF) UpdateManual() bool {
	return false
}

func (e *IF) GetStatus() Status {
	return StatusFinished
}

func (e *IF) GetTaskHumanStatus() TaskHumanStatus {
	return ""
}

func (e *IF) GetType() string {
	return BlockGoIfID
}

func (e *IF) Next(_ *store.VariableStore) ([]string, bool) {
	if e.State.ChosenGroupID == "" {
		nexts, ok := script.GetNexts(e.Sockets, DefaultSocketID)
		if !ok {
			return nil, false
		}
		return nexts, true
	} else {
		nexts, ok := script.GetNexts(e.Sockets, e.State.ChosenGroupID)
		if !ok {
			return nil, false
		}
		return nexts, true
	}
}

func (e *IF) Skipped(_ *store.VariableStore) []string {
	chosenGroupSocket := script.Socket{Id: e.State.ChosenGroupID}
	if chosenGroupSocket.Id == "" {
		chosenGroupSocket = script.DefaultSocket
	}

	skipped := make([]string, 0)
	for i := range e.Sockets {
		var socket = e.Sockets[i]
		if socket.Id != chosenGroupSocket.Id {
			skipped = append(skipped, socket.NextBlockIds...)
		}
	}
	return skipped
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

func (e *IF) DebugRun(_ context.Context, _ *stepCtx, _ *store.VariableStore) error {
	return nil
}

func (e *IF) GetState() interface{} {
	return nil
}

func (e *IF) Update(_ context.Context) (interface{}, error) {
	e.RunContext.VarStore.AddStep(e.Name)
	var chosenGroup *script.ConditionGroup

	if e.State != nil {
		conditionGroups := e.State.GetConditionGroups()

		variables, err := getVariables(e.RunContext.VarStore)
		if err != nil {
			return nil, err
		}

		chosenGroup = processConditionGroups(conditionGroups, variables)
	}

	var chosenGroupID string

	if chosenGroup != nil {
		chosenGroupID = chosenGroup.Id
	} else {
		chosenGroupID = ""
	}

	e.State.ChosenGroupID = chosenGroupID
	return nil, nil
}

func (e *IF) Model() script.FunctionModel {
	return script.FunctionModel{
		ID:        BlockGoIfID,
		BlockType: script.TypeGo,
		Title:     BlockGoIfTitle,
		Inputs:    nil,
		Outputs:   nil,
		Params: &script.FunctionParams{
			Type: BlockGoIfID,
			Params: &script.ConditionParams{
				Type: "",
			},
		},
		Sockets: []script.Socket{script.DefaultSocket},
	}
}

func createGoIfBlock(name string, ef *entity.EriusFunc, runCtx *BlockRunContext) (block *IF, err error) {
	b := &IF{
		Name:       name,
		Title:      ef.Title,
		Input:      map[string]string{},
		Output:     map[string]string{},
		Sockets:    entity.ConvertSocket(ef.Sockets),
		RunContext: runCtx,
	}

	for _, v := range ef.Input {
		b.Input[v.Name] = v.Global
	}

	for _, v := range ef.Output {
		b.Output[v.Name] = v.Global
	}

	b.State = &ConditionsData{
		ChosenGroupID: "",
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

		b.State.Type = params.Type
		b.State.ConditionGroups = params.ConditionGroups
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
		default:
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

func getVariables(runCtx *store.VariableStore) (result map[string]interface{}, err error) {
	variables, err := runCtx.GrabStorage()
	if err != nil {
		return nil, err
	}

	return variables, nil
}
