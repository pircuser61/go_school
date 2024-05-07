package entity

type CreateTaskStepInputs struct {
	WorkID   string
	EventID  string
	StepName string
	Author   string
	Inputs   map[string]interface{}
}
