package entity

type CreateUpdatesInputsHistory struct {
	WorkID   string
	EventID  string
	StepName string
	Author   string
	Inputs   map[string]interface{}
}
