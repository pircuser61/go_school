package script

import "errors"

type FormParams struct {
	BlueprintId   string `json:"blueprint_id"`
	BlueprintName string `json:"blueprint_name"`
}

func (a *FormParams) Validate() error {
	if a.BlueprintId == "" || a.BlueprintName == "" {
		return errors.New("got no form name or id")
	}

	return nil
}
