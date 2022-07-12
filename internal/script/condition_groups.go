package script

import (
	"encoding/json"

	"github.com/pkg/errors"
)

const (
	EqualCompareOperator    string = "equal"
	NotEqualCompareOperator string = "notEqual"

	stringOperandType  string = "string"
	booleanOperandType string = "boolean"
)

var (
	ErrNotComparableOperands = errors.New("Invalid condition. Check for operand types equality and used operator is allowed for type.")
	ErrNoAllowedOperators    = errors.New("Unable to find allowed operators for this type.")
)

type ConditionType string

func (ct ConditionType) String() string {
	return string(ct)
}

type ConditionParams struct {
	Type            ConditionType    `json:"type"`
	ConditionGroups []ConditionGroup `json:"conditionGroups"`
}

func (c *ConditionParams) UnmarshalJSON(b []byte) error {
	const (
		conditionGroupsKey string = "conditionGroups"
		conditionsKey      string = "conditions"
		logicalOperatorKey string = "logicalOperator"
		idKey              string = "id"
		nameKey            string = "name"
		typeKey            string = "type"
	)

	var err error

	var conditionParams map[string]json.RawMessage
	if err := json.Unmarshal(b, &conditionParams); err != nil {
		return err
	}

	var conditionGroupMaps []map[string]json.RawMessage
	if err := json.Unmarshal(conditionParams[conditionGroupsKey], &conditionGroupMaps); err != nil {
		return err
	}

	var conditionGroups = make([]ConditionGroup, 0)
	for _, group := range conditionGroupMaps {

		var conditionsMap []map[string]interface{}

		if err := json.Unmarshal(group[conditionsKey], &conditionsMap); err != nil {
			return err
		}

		var conditions = make([]Condition, 0)
		for _, condition := range conditionsMap {
			var newCondition Condition
			newCondition = unmarshalCondition(condition)
			conditions = append(conditions, newCondition)
		}

		var logicalOperator string
		err = json.Unmarshal(group[logicalOperatorKey], &logicalOperator)

		var id string
		err = json.Unmarshal(group[idKey], &id)

		var name string
		err = json.Unmarshal(group[nameKey], &name)

		if err != nil {
			return err
		}

		var newConditionGroup = ConditionGroup{
			Id:              id,
			Name:            name,
			LogicalOperator: logicalOperator,
			Conditions:      conditions,
		}
		conditionGroups = append(conditionGroups, newConditionGroup)
	}

	var paramType ConditionType
	err = json.Unmarshal(conditionParams[typeKey], &paramType)

	c.ConditionGroups = conditionGroups
	c.Type = paramType

	return nil
}

func unmarshalCondition(conditionRaw map[string]interface{}) Condition {
	const (
		operatorKey     string = "operator"
		leftOperandKey  string = "leftOperand"
		rightOperandKey string = "rightOperand"
	)
	var newCondition = Condition{
		Operator:     conditionRaw[operatorKey].(string),
		LeftOperand:  unmarshalOperand(conditionRaw[leftOperandKey]),
		RightOperand: unmarshalOperand(conditionRaw[rightOperandKey]),
	}

	return newCondition
}

func unmarshalOperand(operandRaw interface{}) Operand {
	const (
		typeKey                    string = "type"
		valueFieldName             string = "value"
		variableReferenceFieldName string = "variableRef"
	)

	var operandMap = operandRaw.(map[string]interface{})
	var operandType = operandMap[typeKey].(string)

	var operand Operand
	allowedOperators, err := getAllowedOperators(operandType)
	if err != nil {
		return nil
	}

	for k, v := range operandMap {
		switch k {
		case valueFieldName:
			operand = &ValueOperand{
				OperandBase: OperandBase{
					Type:             operandType,
					ValueToCompare:   v,
					AllowedOperators: allowedOperators,
				},
				Value: v,
			}
		case variableReferenceFieldName:
			operand = &VariableOperand{
				OperandBase: OperandBase{
					Type:             operandType,
					AllowedOperators: allowedOperators,
				},
				VariableRef: v.(string),
			}
		}
	}

	return operand
}

type CompareOperator func(leftOperand, rightOperand Operand) bool

type Condition struct {
	LeftOperand  Operand `json:"leftOperand"`
	RightOperand Operand `json:"rightOperand"`
	Operator     string  `json:"operator" example:"equal,notEqual"`
}

type ConditionGroup struct {
	Id              string      `json:"id"`
	Name            string      `json:"name"`
	LogicalOperator string      `json:"logicalOperator" example:"or,and"`
	Conditions      []Condition `json:"conditions"`
}

type Operand interface {
	GetValue() interface{}
	GetType() string
	GetAllowedOperators() (map[string]CompareOperator, error)
}

type OperandBase struct {
	Type             string                     `json:"type"`
	AllowedOperators map[string]CompareOperator `json:"-"`
	ValueToCompare   interface{}                `json:"-"`
}

func (valOp *OperandBase) GetType() string {
	return valOp.Type
}

func (valOp *OperandBase) GetValue() interface{} {
	return valOp.ValueToCompare
}

func (valOp *OperandBase) GetAllowedOperators() (map[string]CompareOperator, error) {
	return getAllowedOperators(valOp.Type)
}

type ValueOperand struct {
	Value interface{} `json:"value"`
	OperandBase
}

type VariableOperand struct {
	VariableRef string `json:"variableRef"`
	OperandBase
}

func (condition *Condition) IsTrue() (bool, error) {
	if canCompare(condition) {
		allowedOperatorFunctions, err := condition.LeftOperand.GetAllowedOperators()
		if err != nil {
			return false, err
		}
		var compareFunction = allowedOperatorFunctions[condition.Operator]
		var result = compareFunction(condition.LeftOperand, condition.RightOperand)

		return result, nil
	}

	return false, nil
}

func getAllowedOperators(operatorType string) (map[string]CompareOperator, error) {
	switch operatorType {
	case stringOperandType, booleanOperandType:
		return genericOperators(), nil
	}

	return nil, ErrNoAllowedOperators
}

func genericOperators() map[string]CompareOperator {
	var operatorFunctionsMap = make(map[string]CompareOperator)

	operatorFunctionsMap[EqualCompareOperator] = func(leftOperand, rightOperand Operand) bool {
		return leftOperand.GetValue() == rightOperand.GetValue()
	}

	operatorFunctionsMap[NotEqualCompareOperator] = func(leftOperand, rightOperand Operand) bool {
		return leftOperand.GetValue() != rightOperand.GetValue()
	}

	return operatorFunctionsMap
}

func (c *ConditionParams) Validate() error {

	for _, conditionGroup := range c.ConditionGroups {
		for _, condition := range conditionGroup.Conditions {
			if canCompare(&condition) == false {
				return ErrNotComparableOperands
			}
		}
	}

	return nil
}

func canCompare(condition *Condition) bool {
	return haveAllowedOperator(condition) &&
		haveIdenticalOperandTypes(condition.LeftOperand, condition.RightOperand)
}

func haveAllowedOperator(condition *Condition) bool {
	var operator = condition.Operator

	return operandHaveAllowedOperator(condition.LeftOperand, operator) &&
		operandHaveAllowedOperator(condition.RightOperand, operator)
}

func operandHaveAllowedOperator(operand Operand, operatorType string) bool {
	allowedOperators, err := operand.GetAllowedOperators()
	if err != nil {
		return false
	}

	for key, _ := range allowedOperators {
		if key == operatorType {
			return true
		}
	}

	return false
}

func haveIdenticalOperandTypes(leftOperand, rightOperand Operand) bool {
	return leftOperand.GetType() == rightOperand.GetType()
}
