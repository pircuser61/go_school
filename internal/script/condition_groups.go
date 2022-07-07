package script

import "github.com/pkg/errors"

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

var (
	ErrNotComparableOperands = errors.New("Invalid condition. Check for operand types equality and used operator is allowed for type.")
	ErrCombinedUsage         = errors.New("Unable to use 'allOf' and 'anyOf' together in single group.")
)

type ConditionType string

func (ct ConditionType) String() string {
	return string(ct)
}

type ConditionParams struct {
	Type            ConditionType    `json:"type"`
	ConditionGroups []ConditionGroup `json:"conditionGroups"`
}

type CompareOperator func(leftOperand, rightOperand Operand) bool

type Condition struct {
	LeftOperand  Operand `json:"leftOperand"`
	RightOperand Operand `json:"rightOperand"`
	Operator     string  `json:"operator"`
}

type ConditionGroup struct {
	Alias string
	AllOf []Condition `json:"allOf"`
	AnyOf []Condition `json:"anyOf"`
}

type Operand struct {
	Value            interface{}                `json:"value" example:"\"ref#testVariableName\", \"abc123\", 0, false"`
	Type             string                     `json:"type"`
	AllowedOperators map[string]CompareOperator `json:"-"`
}

func (op *Operand) AsString() {
	op.Type = stringOperandType
	op.AllowedOperators = genericOperators()
}

func (op *Operand) AsBoolean() {
	op.Type = booleanOperandType
	op.AllowedOperators = genericOperators()
}

func (op *Operand) AsInteger() {
	var operatorFunctionsMap = map[string]CompareOperator{
		EqualCompareOperator: func(leftOperand, rightOperand Operand) bool {
			return leftOperand.Value == rightOperand.Value
		},
		NotEqualCompareOperator: func(leftOperand, rightOperand Operand) bool {
			return leftOperand.Value != rightOperand.Value
		},
		MoreThanCompareOperator: func(leftOperand, rightOperand Operand) bool {
			return leftOperand.Value.(int) > rightOperand.Value.(int)
		},
		MoreThanOrEqualCompareOperator: func(leftOperand, rightOperand Operand) bool {
			return leftOperand.Value.(int) >= rightOperand.Value.(int)
		},
		LessThanCompareOperator: func(leftOperand, rightOperand Operand) bool {
			return leftOperand.Value.(int) < rightOperand.Value.(int)
		},
		LessThanOrEqualCompareOperator: func(leftOperand, rightOperand Operand) bool {
			return leftOperand.Value.(int) <= rightOperand.Value.(int)
		},
	}

	op.Type = integerOperandType
	op.AllowedOperators = operatorFunctionsMap
}

func genericOperators() map[string]CompareOperator {
	var operatorFunctionsMap = make(map[string]CompareOperator)

	operatorFunctionsMap[EqualCompareOperator] = func(leftOperand, rightOperand Operand) bool {
		return leftOperand.Value == rightOperand.Value
	}

	operatorFunctionsMap[NotEqualCompareOperator] = func(leftOperand, rightOperand Operand) bool {
		return leftOperand.Value != rightOperand.Value
	}

	return operatorFunctionsMap
}

func (c *ConditionParams) Validate() error {
	for _, conditionGroup := range c.ConditionGroups {
		if len(conditionGroup.AnyOf) > 0 && len(conditionGroup.AllOf) > 0 {
			return ErrCombinedUsage
		}

		for _, condition := range conditionGroup.AnyOf {
			if canCompare(&condition) == false {
				return ErrNotComparableOperands
			}
		}

		for _, condition := range conditionGroup.AllOf {
			if canCompare(&condition) == false {
				return ErrNotComparableOperands
			}
		}
	}

	return nil
}

func (condition *Condition) IsTrue() bool {
	if canCompare(condition) {
		var allowedOperatorFunctions = condition.LeftOperand.AllowedOperators
		var compareFunction = allowedOperatorFunctions[condition.Operator]
		var result = compareFunction(condition.LeftOperand, condition.RightOperand)

		return result
	}

	return false
}

func (cg *ConditionGroup) PrepareOperands() {
	for i, condition := range cg.AnyOf {
		cg.AnyOf[i].LeftOperand, cg.AnyOf[i].RightOperand =
			prepareOperands(condition.LeftOperand, condition.RightOperand)
	}

	for i, condition := range cg.AllOf {
		cg.AnyOf[i].LeftOperand, cg.AnyOf[i].RightOperand =
			prepareOperands(condition.LeftOperand, condition.RightOperand)
	}
}

func prepareOperands(leftOperand, rightOperand Operand) (l, r Operand) {
	return prepareOperand(leftOperand), prepareOperand(rightOperand)
}

func prepareOperand(operand Operand) (o Operand) {
	switch operand.Type {
	case stringOperandType:
		{
			operand.AsString()
		}
	case integerOperandType:
		{
			operand.AsInteger()
		}
	case booleanOperandType:
		{
			operand.AsBoolean()
		}
	}

	return operand
}

func canCompare(condition *Condition) bool {
	return haveAllowedOperator(condition) &&
		haveIdenticalOperandTypes(condition.LeftOperand, condition.RightOperand)
}

func haveAllowedOperator(condition *Condition) bool {
	var operator = condition.Operator

	return operandHaveAllowedOperator(&condition.LeftOperand, operator) &&
		operandHaveAllowedOperator(&condition.RightOperand, operator)
}

func operandHaveAllowedOperator(operand *Operand, operatorType string) bool {
	for key, _ := range operand.AllowedOperators {
		if key == operatorType {
			return true
		}
	}

	return false
}

func haveIdenticalOperandTypes(leftOperand, rightOperand Operand) bool {
	return leftOperand.Type == rightOperand.Type
}
