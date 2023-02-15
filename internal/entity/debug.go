package entity

type BlockOutputs []BlockOutputValue

type BlockOutputValue struct {
	Name  string
	Value interface{}
}

type BlockInputs []BlockInputValue

type BlockInputValue struct {
	Name  string
	Value interface{}
}
