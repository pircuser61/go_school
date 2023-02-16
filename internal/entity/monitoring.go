package entity

type BlockOutputs []BlockOutputValue

type BlockOutputValue struct {
	StepName string
	Name     string
	Value    interface{}
}
