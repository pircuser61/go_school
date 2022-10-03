package script

import "errors"

type FormParams struct {
	BlueprintId   string `json:"blueprint_id"`
	BlueprintName string `json:"blueprint_name"`
	Executor      string `json:"executor"`
}

func (a *FormParams) Validate() error {
	if a.BlueprintId == "" || a.BlueprintName == "" || a.Executor == "" {
		return errors.New("got no form name, id or executor")
	}

	return nil
}
