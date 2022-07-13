package script

import (
	"encoding/json"
	"math"
	"strconv"

	"github.com/pkg/errors"
)

const (
	EqualCompareOperator           string = "Equal"
	NotEqualCompareOperator        string = "NotEqual"
	MoreThanCompareOperator        string = "MoreThan"
	MoreThanOrEqualCompareOperator string = "MoreThanOrEqual"
	LessThanCompareOperator        string = "LessThan"
	LessThanOrEqualCompareOperator string = "LessThanOrEqual"

	stringOperandType  string = "string"
	booleanOperandType string = "boolean"
	integerOperandType string = "integer"
	floatOperandType   string = "float"
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
	ConvertType(operandType string) (ok bool)
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

func (valOp *OperandBase) ConvertType(operandType string) bool {
	if ok, val := convertValue(valOp, operandType); ok {
		valOp.DataType = operandType
		valOp.ValueToCompare = val

		allowedOperators, err := getAllowedOperators(operandType)
		if err != nil {
			return false
		}

		valOp.AllowedOperators = allowedOperators
		return true
	}
	return false
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
		var leftOperand = condition.LeftOperand
		var rightOperand = condition.RightOperand

		if !haveEqualOperandTypes(leftOperand, rightOperand) {
			_ = leftOperand.ConvertType(rightOperand.GetType())
		}

		allowedOperatorFunctions, err := condition.LeftOperand.GetAllowedOperators()
		if err != nil {
			return false, err
		}
		var compareFunction = allowedOperatorFunctions[condition.Operator]

		var result = compareFunction(leftOperand, rightOperand)

		return result, nil
	}

	return false, nil
}

func getAllowedOperators(operandType string) (map[string]CompareOperator, error) {
	switch operandType {
	case stringOperandType, booleanOperandType:
		return genericOperators(), nil
	case integerOperandType:
		return map[string]CompareOperator{
			EqualCompareOperator: func(leftOperand, rightOperand Operand) bool {
				return leftOperand.GetValue() == rightOperand.GetValue()
			},
			NotEqualCompareOperator: func(leftOperand, rightOperand Operand) bool {
				return leftOperand.GetValue() != rightOperand.GetValue()
			},
			MoreThanCompareOperator: func(leftOperand, rightOperand Operand) bool {
				return leftOperand.GetValue().(int) > rightOperand.GetValue().(int)
			},
			MoreThanOrEqualCompareOperator: func(leftOperand, rightOperand Operand) bool {
				return leftOperand.GetValue().(int) >= rightOperand.GetValue().(int)
			},
			LessThanCompareOperator: func(leftOperand, rightOperand Operand) bool {
				return leftOperand.GetValue().(int) < rightOperand.GetValue().(int)
			},
			LessThanOrEqualCompareOperator: func(leftOperand, rightOperand Operand) bool {
				return leftOperand.GetValue().(int) <= rightOperand.GetValue().(int)
			},
		}, nil
	case floatOperandType:
		return map[string]CompareOperator{
			EqualCompareOperator: func(leftOperand, rightOperand Operand) bool {
				var equalityThreshold = 1e-9
				var leftValue = leftOperand.GetValue().(float64)
				var rightValue = leftOperand.GetValue().(float64)
				return math.Abs(leftValue-rightValue) <= equalityThreshold
			},
			NotEqualCompareOperator: func(leftOperand, rightOperand Operand) bool {
				var equalityThreshold = 1e-9
				var leftValue = leftOperand.GetValue().(float64)
				var rightValue = leftOperand.GetValue().(float64)
				return math.Abs(leftValue-rightValue) >= equalityThreshold
			},
			MoreThanCompareOperator: func(leftOperand, rightOperand Operand) bool {
				return leftOperand.GetValue().(float64) > rightOperand.GetValue().(float64)
			},
			MoreThanOrEqualCompareOperator: func(leftOperand, rightOperand Operand) bool {
				return leftOperand.GetValue().(float64) >= rightOperand.GetValue().(float64)
			},
			LessThanCompareOperator: func(leftOperand, rightOperand Operand) bool {
				return leftOperand.GetValue().(float64) < rightOperand.GetValue().(float64)
			},
			LessThanOrEqualCompareOperator: func(leftOperand, rightOperand Operand) bool {
				return leftOperand.GetValue().(float64) <= rightOperand.GetValue().(float64)
			},
		}, nil
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
			ok, leftVal = convertValue(leftOperand, rightOperand.GetType())
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
			ok, leftVal = convertValue(leftOperand, rightOperand.GetType())
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
		haveIdenticalOperandTypesOrCanBeConverted(condition.LeftOperand, condition.RightOperand)
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

func haveIdenticalOperandTypesOrCanBeConverted(leftOperand, rightOperand Operand) bool {
	canBeConverted, _ := convertValue(leftOperand, rightOperand.GetType())
	return haveEqualOperandTypes(leftOperand, rightOperand) || canBeConverted
}

func haveEqualOperandTypes(leftOperand, rightOperand Operand) bool {
	return leftOperand.GetType() == rightOperand.GetType()
}

//nolint:gocyclo //ok
func convertValue(original Operand, newOperandType string) (canBeConverted bool, res interface{}) {
	var originalValue = original.GetValue()
	switch original.GetType() {
	case stringOperandType:
		switch newOperandType {
		case stringOperandType:
			return true, originalValue
		case booleanOperandType:
			return true, originalValue == "true"
		case integerOperandType:
			val, err := strconv.ParseFloat(originalValue.(string), 32)
			if err != nil {
				return false, nil
			}
			return true, val
		case floatOperandType:
			val, err := strconv.ParseFloat(originalValue.(string), 32)
			if err != nil {
				return false, nil
			}
			return true, val
		default:
			return false, nil
		}
	case booleanOperandType:
		switch newOperandType {
		case booleanOperandType:
			return true, originalValue
		case stringOperandType:
			val := original.GetValue()
			if val != nil {
				val = strconv.FormatBool(val.(bool))
			}
			return true, val
		default:
			return false, nil
		}
	case integerOperandType:
		switch newOperandType {
		case integerOperandType:
			return true, originalValue
		case stringOperandType:
			val := originalValue
			if val != nil {
				// float64 type goes from map[string]interface{} unmarshaller
				// so first thing first we'll cast to float64, then to integer
				if floatVal, ok := val.(float64); ok {
					val = strconv.FormatFloat(floatVal, 'f', -1, 64)
					return true, val
				}
			}
			return false, nil
		default:
			return false, nil
		}
	case floatOperandType:
		switch newOperandType {
		case floatOperandType:
			return true, originalValue
		case stringOperandType:
			val := originalValue
			if val != nil {
				if floatVal, ok := val.(float64); ok {
					val = strconv.FormatFloat(floatVal, 'f', -1, 64)
					return true, val
				}
			}
			return false, nil
		default:
			return false, nil
		}
	}

	return false, nil
}
