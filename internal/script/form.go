package script

import "errors"

type FormExecutorType string

func (f FormExecutorType) String() string {
	return string(f)
}

const (
	FormExecutorTypeUser       FormExecutorType = "user"
	FormExecutorTypeInitiator  FormExecutorType = "initiator"
	FormExecutorTypeFromSchema FormExecutorType = "from_schema"
)

type FormParams struct {
	SchemaId         string           `json:"schema_id"`
	SchemaName       string           `json:"schema_name"`
	Executor         string           `json:"executor"`
	FormExecutorType FormExecutorType `json:"form_executor_type"`
}

func (a *FormParams) Validate() error {
	if a.SchemaId == "" || (a.FormExecutorType == FormExecutorTypeUser && a.Executor == "") {
		return errors.New("got no form name, id or executor")
	}

	return nil
}
