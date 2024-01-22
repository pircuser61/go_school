package script

import (
	"testing"
)

func TestFormParams_Validate(t *testing.T) {
	type fields struct {
		FormExecutorType FormExecutorType
		SchemaId         string
		CheckSLA         bool
		SLA              int
	}

	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "form auto_fill with sla=0",
			fields: fields{
				FormExecutorType: FormExecutorTypeAutoFillUser,
				SchemaId:         "example",
				CheckSLA:         true,
				SLA:              0,
			},
			wantErr: false,
		},
		{
			name: "form not auto_fill with sla=0",
			fields: fields{
				FormExecutorType: FormExecutorTypeFromSchema,
				SchemaId:         "example",
				CheckSLA:         true,
				SLA:              0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &FormParams{
				FormExecutorType: tt.fields.FormExecutorType,
				SchemaID:         tt.fields.SchemaId,
				SLA:              tt.fields.SLA,
				CheckSLA:         tt.fields.CheckSLA,
			}
			if err := a.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("%v ValidateFormParam()", a)
			}
		})
	}
}
