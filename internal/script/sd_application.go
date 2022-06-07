package script

import (
	"errors"
)

type SdApplicationParams struct {
	Name string
}

func (a *SdApplicationParams) Validate() error {
	if a.Name == "" {
		return errors.New("name is empty")
	}

	return nil
}
