package script

import "errors"

type ExecutableFunctionParams struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func (a *ExecutableFunctionParams) Validate() error {
	if a.Name == "" || a.Version == "" {
		return errors.New("got no function name or version")
	}

	return nil
}
