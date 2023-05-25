package api

import (
	"testing"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

func TestValidation_EndExists(t *testing.T) {
	tests := []struct {
		Name      string
		Ef        entity.EriusScenario
		WantValid bool
	}{
		{
			Name: "test valid blocks",
			Ef: entity.EriusScenario{
				Pipeline: struct {
					Entrypoint string            `json:"entrypoint"`
					Blocks     entity.BlocksType `json:"blocks"`
				}{
					Blocks: entity.BlocksType{
						"block_1": {
							TypeID: "start",
						},
						"block_2": {
							TypeID: "end",
						},
					},
				},
			},
			WantValid: true,
		},
		{
			Name: "test invalid blocks",
			Ef: entity.EriusScenario{
				Pipeline: struct {
					Entrypoint string            `json:"entrypoint"`
					Blocks     entity.BlocksType `json:"blocks"`
				}{
					Blocks: entity.BlocksType{
						"block_1": {
							TypeID: "start",
						},
						"block_2": {
							TypeID: "approver",
						},
					},
				},
			},
			WantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			if tt.WantValid && !tt.Ef.Pipeline.Blocks.Validate() {
				t.Errorf("unexpected invalid %+v", tt.Ef.Pipeline.Blocks)
			}
		})
	}
}
