package pipeline

import (
	"github.com/pkg/errors"
	"strings"
)

// todo: move to separate files

// Constants! //

const (
	MoreThanCompareOperator        string = "moreThan"
	LessThanCompareOperator        string = "lessThan"
	MoreThanOrEqualCompareOperator string = "moreThanOrEqual"
	LessThanOrEqualCompareOperator string = "lessThanOrEqual"
	EqualCompareOperator           string = "equal"
	NotEqualCompareOperator        string = "notEqual"

	stringOperandType  string = "string"
	booleanOperandType string = "boolean"
	integerOperandType string = "integer"
)

// Errors! //

var (
	ErrOperandTypesNotEqual = errors.New("Operand types are not equal.")
	ErrForbiddenOperator    = errors.New("Forbidden operator for operand.")
)

// Types! //

type CompareOperator struct {
	Type            string                                       `json:"type"`
	CompareFunction func(leftOperand, rightOperand Operand) bool `json:"-"`
}

type Condition struct {
	LeftOperand  Operand         `json:"leftOperand"`
	RightOperand Operand         `json:"rightOperand"`
	Operator     CompareOperator `json:"operator"`
}

type ConditionGroup struct {
	AllOf []Condition `json:"allOf"`
	AnyOf []Condition `json:"anyOf"`
}

type Operand struct {
	Value            interface{}
	Type             string
	AllowedOperators []CompareOperator
}

type StringOperand struct{}
type BooleanOperand struct{}
type IntegerOperand struct{}

func (strOperand *StringOperand) Create() Operand {
	var newOperand = Operand{
		Type:             stringOperandType,
		AllowedOperators: genericOperators(),
	}

	return newOperand
}

func (boolOperand *BooleanOperand) Create() Operand {
	var newOperand = Operand{
		Type:             booleanOperandType,
		AllowedOperators: genericOperators(),
	}

	return newOperand
}

func (intOperand *IntegerOperand) Create() Operand {
	var newOperand = Operand{
		Type: integerOperandType,
		AllowedOperators: []CompareOperator{
			{
				Type: EqualCompareOperator,
				CompareFunction: func(leftOperand, rightOperand Operand) bool {
					return leftOperand.Value == rightOperand.Value
				},
			},
			{
				Type: NotEqualCompareOperator,
				CompareFunction: func(leftOperand, rightOperand Operand) bool {
					return leftOperand.Value != rightOperand.Value
				},
			},
			{
				Type: MoreThanCompareOperator,
				CompareFunction: func(leftOperand, rightOperand Operand) bool {
					return leftOperand.Value.(int) > rightOperand.Value.(int)
				},
			},
			{
				Type: MoreThanOrEqualCompareOperator,
				CompareFunction: func(leftOperand, rightOperand Operand) bool {
					return leftOperand.Value.(int) >= rightOperand.Value.(int)
				},
			},
			{
				Type: LessThanCompareOperator,
				CompareFunction: func(leftOperand, rightOperand Operand) bool {
					return leftOperand.Value.(int) < rightOperand.Value.(int)
				},
			},
			{
				Type: LessThanOrEqualCompareOperator,
				CompareFunction: func(leftOperand, rightOperand Operand) bool {
					return leftOperand.Value.(int) <= rightOperand.Value.(int)
				},
			},
		},
	}

	return newOperand
}

func (condition *Condition) Check() bool {
	if canCompare(condition) {
		var compareFunction = condition.Operator.CompareFunction
		var result = compareFunction(condition.LeftOperand, condition.RightOperand)

		return result
	}

	return false
}

// Functions! //

func determineNodeExit() {
	// iterate through condition groups (one in case of MVP)
}

func retrieveConditions() {
	// unmarshal json params to structs
}

func makeOutputs(groups []ConditionGroup) {

}

func getValue() {
	// todo: get variable value or const value from json
}

func tryGetVariableReference(value interface{}) string {
	const referencePrefix = "ref#"
	const empty = ""

	if val, ok := value.(string); ok {
		if strings.HasPrefix(val, referencePrefix) {
			var variableName = strings.Replace(val, referencePrefix, empty, 1)
			return variableName
		}
	}

	return empty
}

func genericOperators() []CompareOperator {
	return []CompareOperator{
		{
			Type: EqualCompareOperator,
			CompareFunction: func(leftOperand, rightOperand Operand) bool {
				return leftOperand.Value == rightOperand.Value
			},
		},
		{
			Type: NotEqualCompareOperator,
			CompareFunction: func(leftOperand, rightOperand Operand) bool {
				return leftOperand.Value != rightOperand.Value
			},
		},
	}
}

func canCompare(condition *Condition) bool {
	return haveAllowedOperator(condition) &&
		haveIdenticalOperandTypes(condition.LeftOperand, condition.RightOperand)
}

func haveAllowedOperator(condition *Condition) bool {
	var operator = condition.Operator

	return operandHaveAllowedOperator(&condition.LeftOperand, operator.Type) &&
		operandHaveAllowedOperator(&condition.RightOperand, operator.Type)
}

func operandHaveAllowedOperator(operand *Operand, operatorType string) bool {
	for i := range operand.AllowedOperators {
		if operand.AllowedOperators[i].Type == operatorType {
			return true
		}
	}
	// todo: handle err - forbidden operator for operand
	return false
}

func haveIdenticalOperandTypes(leftOperand, rightOperand Operand) bool {
	// todo: handle err - operand types not equal
	return leftOperand.Type == rightOperand.Type
}
