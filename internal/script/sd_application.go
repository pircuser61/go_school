package script

import (
	"errors"
)

type SdApplicationParams struct {
	BlueprintID     string                 `json:"blueprint_id"`
	Description     string                 `json:"description"`
	ApplicationBody map[string]interface{} `json:"application_body"`
}

func (a *SdApplicationParams) Validate() error {
	if a.BlueprintID == "" {
		return errors.New("blueprintID is empty")
	}

	if a.Description == "" {
		return errors.New("description is empty")
	}

	return nil
}
