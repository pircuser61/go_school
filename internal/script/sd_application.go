package script

type SdApplicationParams struct {
	PipelineID string `json:"pipeline_id"`
}

func (a *SdApplicationParams) Validate() error {
	return nil
}
