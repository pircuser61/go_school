package script

import "errors"

type FormParams struct {
	SchemaId   string `json:"schema_id"`
	SchemaName string `json:"schema_name"`
	Executor   string `json:"executor"`
}

func (a *FormParams) Validate() error {
	if a.SchemaId == "" || a.SchemaName == "" || a.Executor == "" {
		return errors.New("got no form name, id or executor")
	}

	return nil
}
