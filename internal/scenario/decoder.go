package scenario

import (
	"fmt"
)

type CreateOrUpdateTemplateDto struct {
	Name        string
	Nodes       []CreateOrUpdateTemplateNodeDto
	Transitions []CreateOrUpdateTemplateTransitionDto
}

type CreateOrUpdateTemplateTransitionDto struct {
	FromPort string
	From     string
	To       string
}

type CreateOrUpdateTemplateNodeDto struct {
	Id    string
	Name  string
	Type  string
	Props []CreateOrUpdateTemplateProps
}

type CreateOrUpdateTemplateProps struct {
	Key   string
	Type  string
	Value string
}

type validationNode struct {
	Id   string
	Name string
	Type string
}

type ValidationError struct {
	Nodes   []validationNode
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%+v", e.Nodes) + ": " + e.Message
}
