package script

import (
	"errors"
)

type SdApplicationParams struct {
	BlueprintID string `json:"blueprint_id"`
}

func (a *SdApplicationParams) Validate() error {
	if a.BlueprintID == "" {
		return errors.New("name is empty")
	}

	return nil
}
