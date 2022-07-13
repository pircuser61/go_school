package script

import (
	"math"
	"strconv"

	"encoding/json"

	"github.com/pkg/errors"
)

const (
	EqualCompareOperator    string = "Equal"
	NotEqualCompareOperator string = "NotEqual"

	stringOperandType  string = "string"
	booleanOperandType string = "boolean"
	integerOperandType string = "integer"
	floatOperandType   string = "float"
)

var (
	ErrCantFindAllowedCastTypeFuncs = errors.New("Can't find allowed cast type functions.")
	ErrNotComparableOperands        = errors.New("Invalid condition. Check for operand types equality and used operator is allowed for type.")
	ErrNoAllowedOperators           = errors.New("Unable to find allowed operators for this type.")
	ErrUnableToConvert              = errors.New("Unable to convert operand.")
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
type CastFunction func(source Operand) interface{}

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

type TypeCast struct {
	From string
	To   string
}

type Operand interface {
	GetValue() interface{}
	GetType() string
	GetAllowedOperators() (map[string]CompareOperator, error)
	GetAllowedTypeCasts() (map[TypeCast]CastFunction, error)
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

func (valOp *OperandBase) GetAllowedTypeCasts() (map[TypeCast]CastFunction, error) {
	return getAllowedTypesCast(valOp.DataType)
}

func (valOp *OperandBase) ConvertType(operandType string) bool {
	allowedTypeCasts, err := valOp.GetAllowedTypeCasts()
	if err != nil {
		return false
	}

	var castFunction = getCastFunctionByOperandType(allowedTypeCasts, operandType)
	if castFunction != nil {
		valOp.DataType = operandType
		valOp.ValueToCompare = castFunction(valOp)

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
			ok := leftOperand.ConvertType(rightOperand.GetType())
			if !ok {
				return false, ErrUnableToConvert
			}
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

func getAllowedOperators(operandDataType string) (map[string]CompareOperator, error) {
	switch operandDataType {
	case stringOperandType, booleanOperandType:
		return genericOperators(), nil
	case integerOperandType:
		return genericOperators(), nil
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
		}, nil
	}
	return nil, ErrNoAllowedOperators
}

//nolint:goconst,gocyclo //it's ok
func getAllowedTypesCast(operandDataType string) (map[TypeCast]CastFunction, error) {
	var castFunctions = map[TypeCast]CastFunction{
		{From: stringOperandType, To: stringOperandType}: func(source Operand) interface{} {
			return source.GetValue()
		},
		{From: stringOperandType, To: integerOperandType}: func(source Operand) interface{} {
			var stringValue = source.GetValue().(string)
			floatValue, err := strconv.ParseFloat(stringValue, 64)
			if err != nil {
				return nil
			}
			return floatValue
		},
		{From: stringOperandType, To: floatOperandType}: func(source Operand) interface{} {
			var stringValue = source.GetValue().(string)
			floatValue, err := strconv.ParseFloat(stringValue, 64)
			if err != nil {
				return nil
			}
			return floatValue
		},
		{From: stringOperandType, To: booleanOperandType}: func(source Operand) interface{} {
			var stringValue = source.GetValue().(string)
			switch stringValue {
			case "0", "false":
				return false
			case "1", "true":
				return true
			default:
				return nil
			}
		},
		{From: booleanOperandType, To: booleanOperandType}: func(source Operand) interface{} {
			return source.GetValue()
		},
		{From: booleanOperandType, To: stringOperandType}: func(source Operand) interface{} {
			var boolValue = source.GetValue().(bool)
			switch boolValue {
			case false:
				return "false"
			case true:
				return "true"
			default:
				return nil
			}
		},
		{From: booleanOperandType, To: integerOperandType}: func(source Operand) interface{} {
			var boolValue = source.GetValue().(bool)
			switch boolValue {
			case false:
				return float64(0)
			case true:
				return float64(1)
			default:
				return nil
			}
		},
		{From: integerOperandType, To: integerOperandType}: func(source Operand) interface{} {
			return source.GetValue()
		},
		{From: integerOperandType, To: stringOperandType}: func(source Operand) interface{} {
			if floatVal, ok := source.GetValue().(float64); ok {
				return strconv.FormatFloat(floatVal, 'f', -1, 64)
			}
			return nil
		},
		{From: integerOperandType, To: floatOperandType}: func(source Operand) interface{} {
			if floatVal, ok := source.GetValue().(float64); ok {
				return floatVal
			}
			return nil
		},
		{From: integerOperandType, To: booleanOperandType}: func(source Operand) interface{} {
			var floatValue = source.GetValue().(float64)
			switch floatValue {
			case 0:
				return false
			case 1:
				return true
			default:
				return nil
			}
		},
		{From: floatOperandType, To: floatOperandType}: func(source Operand) interface{} {
			return source.GetValue()
		},
		{From: floatOperandType, To: stringOperandType}: func(source Operand) interface{} {
			if floatVal, ok := source.GetValue().(float64); ok {
				return strconv.FormatFloat(floatVal, 'f', -1, 64)
			}
			return nil
		},
		{From: floatOperandType, To: integerOperandType}: func(source Operand) interface{} {
			if floatVal, ok := source.GetValue().(float64); ok {
				return math.Trunc(floatVal)
			}
			return nil
		},
	}

	result := make(map[TypeCast]CastFunction, 0)

	for k, v := range castFunctions {
		if k.From == operandDataType {
			result[k] = v
		}
	}

	if len(result) == 0 {
		return nil, ErrCantFindAllowedCastTypeFuncs
	}

	return result, nil
}

func genericOperators() map[string]CompareOperator {
	var operatorFunctionsMap = map[string]CompareOperator{
		EqualCompareOperator: func(leftOperand, rightOperand Operand) bool {
			return leftOperand.GetValue() == rightOperand.GetValue()
		},
		NotEqualCompareOperator: func(leftOperand, rightOperand Operand) bool {
			return leftOperand.GetValue() == rightOperand.GetValue()
		},
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
	return haveAllowedOperator(condition) && (haveEqualOperandTypes(condition.LeftOperand, condition.RightOperand) ||
		canBeConverted(condition.LeftOperand, condition.RightOperand))
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

func canBeConverted(leftOperand, rightOperand Operand) bool {
	var neededTypeCast = rightOperand.GetType()

	allowedTypeCasts, err := leftOperand.GetAllowedTypeCasts()
	if err != nil {
		return false
	}

	for k := range allowedTypeCasts {
		if k.To == neededTypeCast {
			return true
		}
	}
	return false
}

func getCastFunctionByOperandType(m map[TypeCast]CastFunction, neededOperandCastType string) CastFunction {
	for k, v := range m {
		if k.To == neededOperandCastType {
			return v
		}
	}
	return nil
}

func haveEqualOperandTypes(leftOperand, rightOperand Operand) bool {
	return leftOperand.GetType() == rightOperand.GetType()
}
