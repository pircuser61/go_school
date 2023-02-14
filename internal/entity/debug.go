package entity

type BlockOutputs []BlockOutputValue

type BlockOutputValue struct {
	Name  string
	Value interface{}
}
