package script

import (
	"encoding/json"
	"strconv"

	"github.com/pkg/errors"
)

const (
	EqualCompareOperator    string = "Equal"
	NotEqualCompareOperator string = "NotEqual"

	stringOperandType  string = "string"
	booleanOperandType string = "boolean"
	// TODO: handle integer (what about float?)
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
	Type            ConditionType    `json:"type,omitempty"`
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
			var newCondition = unmarshalCondition(condition)
			conditions = append(conditions, newCondition)
		}

		var logicalOperator string
		err := json.Unmarshal(group[logicalOperatorKey], &logicalOperator)
		if err != nil {
			return err
		}

		var id string
		err = json.Unmarshal(group[idKey], &id)
		if err != nil {
			return err
		}

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

	c.ConditionGroups = conditionGroups

	if _, ok := conditionParams[typeKey]; ok {
		var paramType ConditionType
		err := json.Unmarshal(conditionParams[typeKey], &paramType)
		if err != nil {
			return err
		}
		c.Type = paramType
	}

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
		typeKey                    string = "dataType"
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
					DataType:         operandType,
					ValueToCompare:   v,
					AllowedOperators: allowedOperators,
				},
				Value: v,
			}
		case variableReferenceFieldName:
			operand = &VariableOperand{
				OperandBase: OperandBase{
					DataType:         operandType,
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
	DataType         string                     `json:"dataType"`
	AllowedOperators map[string]CompareOperator `json:"-"`
	ValueToCompare   interface{}                `json:"-"`
}

func (valOp *OperandBase) GetType() string {
	return valOp.DataType
}

func (valOp *OperandBase) GetValue() interface{} {
	return valOp.ValueToCompare
}

func (valOp *OperandBase) GetAllowedOperators() (map[string]CompareOperator, error) {
	return getAllowedOperators(valOp.DataType)
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
		var leftVal, rightVal interface{}
		if leftOperand.GetType() == rightOperand.GetType() {
			leftVal, rightVal = leftOperand.GetValue(), rightOperand.GetValue()
		} else {
			var ok bool
			ok, leftVal = convertValue(leftOperand, rightOperand)
			if !ok {
				return false
			}
			rightVal = rightOperand.GetValue()
		}
		return leftVal == rightVal
	}

	operatorFunctionsMap[NotEqualCompareOperator] = func(leftOperand, rightOperand Operand) bool {
		var leftVal, rightVal interface{}
		if leftOperand.GetType() == rightOperand.GetType() {
			leftVal, rightVal = leftOperand.GetValue(), rightOperand.GetValue()
		} else {
			var ok bool
			ok, leftVal = convertValue(leftOperand, rightOperand)
			if !ok {
				return false
			}
			rightVal = rightOperand.GetValue()
		}
		return leftVal == rightVal
	}

	return operatorFunctionsMap
}

func (c *ConditionParams) Validate() error {
	for _, conditionGroup := range c.ConditionGroups {
		for j := range conditionGroup.Conditions {
			var co = conditionGroup.Conditions[j]
			if !canCompare(&co) {
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

	for key := range allowedOperators {
		if key == operatorType {
			return true
		}
	}

	return false
}

func haveIdenticalOperandTypes(leftOperand, rightOperand Operand) bool {
	eqTypes := leftOperand.GetType() == rightOperand.GetType()
	canBeConverted, _ := convertValue(leftOperand, rightOperand)
	return eqTypes || canBeConverted
}

func convertValue(original, convertTo Operand) (canBeConverted bool, res interface{}) {
	switch original.GetType() {
	case stringOperandType:
		switch convertTo.GetType() {
		case stringOperandType:
			return true, original.GetValue()
		case booleanOperandType:
			return true, original.GetValue() == "true"
		default:
			return false, nil
		}
	case booleanOperandType:
		switch convertTo.GetType() {
		case booleanOperandType:
			return true, original.GetValue()
		case stringOperandType:
			val := original.GetValue()
			if val != nil {
				val = strconv.FormatBool(val.(bool))
			}
			return true, val
		default:
			return false, nil
		}
	}
	return false, nil
}
