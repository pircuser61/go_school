package script

import (
	"testing"
)

func TestApproverParams_Validate(t *testing.T) {
	type fields struct {
		Type          ApproverType
		ApproverLogin string
		SLA           int
		AutoAction    string
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "approver is empty",
			fields: fields{
				Type:          ApproverTypeUser,
				ApproverLogin: "",
				SLA:           0,
			},
			wantErr: true,
		},
		{
			name: "unknown approver type",
			fields: fields{
				Type:          ApproverType("unknown"),
				ApproverLogin: "example",
				SLA:           0,
			},
			wantErr: true,
		},
		{
			name: "bad SLA",
			fields: fields{
				Type:          ApproverTypeUser,
				ApproverLogin: "example",
				SLA:           0,
			},
			wantErr: true,
		},
		{
			name: "acceptance test",
			fields: fields{
				Type:          ApproverTypeUser,
				ApproverLogin: "example",
				SLA:           1,
				AutoAction:    "approve",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &ApproverParams{
				Type:       tt.fields.Type,
				Approver:   tt.fields.ApproverLogin,
				SLA:        tt.fields.SLA,
				AutoAction: &tt.fields.AutoAction,
			}
			if err := a.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("%v Validate()", a)
			}
		})
	}
}
