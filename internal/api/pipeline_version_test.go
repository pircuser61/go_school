package api

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
)

func TestAPIEnv_getClientIDFromToken(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		want    string
		wantErr bool
	}{
		{
			name:    "success case",
			token:   "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyLCJhenAiOiJzZXJ2aWNlZGVzay1kZXZlbG9wIn0.HaFL3_LNAV7uoe77twZ7bCU3KoHo89wIOi1_1xvJBDM",
			want:    "servicedesk-develop",
			wantErr: false,
		},
		{
			name:    "invalid clientID, error",
			token:   "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyLCJhenAiOlsic2VydmljZWRlc2stZGV2ZWxvcCJdfQ.LrMDsRkbMZtHIxogs_sYguCRcBq5KeucDcUyJ2lBEzY",
			want:    "",
			wantErr: true,
		},
		{
			name:    "missing clientID, error",
			token:   "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ae := &APIEnv{}
			got, err := ae.getClietIDFromToken(tt.token)
			if (err != nil) != tt.wantErr {
				t.Errorf("getClietIDFromToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getClietIDFromToken() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestChangeOutput(t *testing.T) {
	tests := []struct {
		Name        string
		Ef          entity.EriusScenario
		WantedGroup entity.EriusScenario
	}{
		{
			Name:        "accepts test",
			Ef:          *unmarshalFromTestFile(t, "testdata/test_change_output.json"),
			WantedGroup: *unmarshalFromTestFile(t, "testdata/test_change_output_result.json"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			keyOutputs := map[string]string{
				pipeline.BlockGoApproverID:  "approver",
				pipeline.BlockGoSignID:      "signer",
				pipeline.BlockGoExecutionID: "login",
			}

			tt.Ef.Pipeline.ChangeOutput(keyOutputs)

			assert.Equal(t, tt.Ef.Pipeline, tt.WantedGroup.Pipeline, "ChangeOutput(%v)", keyOutputs)
		})
	}
}
