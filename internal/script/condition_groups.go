package script

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
