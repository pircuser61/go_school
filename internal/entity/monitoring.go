package entity

type BlockOutputs []BlockOutputValue

type BlockOutputValue struct {
	StepName string
	Name     string
	Value    interface{}
}

type BlockInputs []BlockInputValue

type BlockInputValue struct {
	Name  string
	Value interface{}
}
