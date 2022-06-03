package script

import "testing"

func TestApproverParams_Validate(t *testing.T) {
	type fields struct {
		Type          ApproverType
		ApproverLogin string
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
			},
			wantErr: true,
		},
		{
			name: "unknown approver type",
			fields: fields{
				Type:          ApproverType("unknown"),
				ApproverLogin: "example",
			},
			wantErr: true,
		},
		{
			name: "acceptance test",
			fields: fields{
				Type:          ApproverTypeUser,
				ApproverLogin: "example",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &ApproverParams{
				Type:          tt.fields.Type,
				ApproverLogin: tt.fields.ApproverLogin,
			}
			if err := a.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("%v Validate()", a)
			}
		})
	}
}
