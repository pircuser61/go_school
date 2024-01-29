package api

import (
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
)

func (ae *Env) handlePipelineBlockLenght(p *entity.EriusScenario) {
	if len(p.Pipeline.Blocks) == 0 {
		p.Pipeline.FillEmptyPipeline()
	} else {
		keyOutputs := map[string]string{
			pipeline.BlockGoApproverID:  "approver",
			pipeline.BlockGoSignID:      "signer",
			pipeline.BlockGoExecutionID: "login",
		}

		p.Pipeline.ChangeOutput(keyOutputs)
	}
}
