package script

import "errors"

type FormParams struct {
	FormId   string `json:"form_id"`
	FormName string `json:"form_name"`
}

func (a *FormParams) Validate() error {
	if a.FormId == "" || a.FormName == "" {
		return errors.New("got no form name or id")
	}

	return nil
}
