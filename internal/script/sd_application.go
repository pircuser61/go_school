package script

import (
	"errors"
)

type SdApplicationParams struct {
	PipelineID string `json:"pipeline_id"`
}

func (a *SdApplicationParams) Validate() error {
	if a.PipelineID == "" {
		return errors.New("pipelineID is empty")
	}

	return nil
}
