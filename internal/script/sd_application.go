package script

import (
	"errors"
)

type SdApplicationParams struct {
	BlueprintID string `json:"blueprintId"`
}

func (a *SdApplicationParams) Validate() error {
	if a.BlueprintID == "" {
		return errors.New("blueprintID is empty")
	}

	return nil
}
