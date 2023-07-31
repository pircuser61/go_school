package script

import (
	"testing"
)

func TestSignParams_Validate(t *testing.T) {
	type fields struct {
		Type              SignerType
		Rule              SigningRule
		Signer            string
		SignatureType     SignatureType
		SignatureCarrier  SignatureCarrier
		SignerGroupID     string
		SignerGroupIDPath string
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "PEP - user",
			fields: fields{
				SignatureType: SignatureTypePEP,
				Type:          SignerTypeUser,
				Signer:        "test",
			},
			wantErr: false,
		},
		{
			name: "PEP - no user",
			fields: fields{
				SignatureType: SignatureTypePEP,
				Type:          SignerTypeUser,
				Signer:        "",
			},
			wantErr: true,
		},
		{
			name: "PEP - group",
			fields: fields{
				SignatureType: SignatureTypePEP,
				Type:          SignerTypeGroup,
			},
			wantErr: true,
		},
		{
			name: "PEP - schema",
			fields: fields{
				SignatureType: SignatureTypePEP,
				Type:          SignerTypeFromSchema,
				Signer:        "test",
			},
			wantErr: true,
		},
		{
			name: "UNEP - user",
			fields: fields{
				SignatureType: SignatureTypeUNEP,
				Type:          SignerTypeUser,
				Signer:        "test",
			},
			wantErr: false,
		},
		{
			name: "UNEP - no user",
			fields: fields{
				SignatureType: SignatureTypeUNEP,
				Type:          SignerTypeUser,
				Signer:        "",
			},
			wantErr: true,
		},
		{
			name: "UNEP - group id",
			fields: fields{
				SignatureType: SignatureTypeUNEP,
				Type:          SignerTypeGroup,
				SignerGroupID: "test",
			},
			wantErr: false,
		},
		{
			name: "UNEP - group id path",
			fields: fields{
				SignatureType:     SignatureTypeUNEP,
				Type:              SignerTypeGroup,
				SignerGroupIDPath: "test",
			},
			wantErr: false,
		},
		{
			name: "UNEP - group bad rule",
			fields: fields{
				SignatureType:     SignatureTypeUNEP,
				Type:              SignerTypeGroup,
				SignerGroupIDPath: "test",
				Rule:              "bad rule",
			},
			wantErr: true,
		},
		{
			name: "UNEP - no group",
			fields: fields{
				SignatureType: SignatureTypeUNEP,
				Type:          SignerTypeGroup,
			},
			wantErr: true,
		},
		{
			name: "UNEP - schema user",
			fields: fields{
				SignatureType: SignatureTypeUNEP,
				Type:          SignerTypeFromSchema,
				Signer:        "test",
			},
			wantErr: false,
		},
		{
			name: "UNEP - schema no user",
			fields: fields{
				SignatureType: SignatureTypeUNEP,
				Type:          SignerTypeFromSchema,
				Signer:        "",
			},
			wantErr: true,
		},
		{
			name: "UNEP - schema group",
			fields: fields{
				SignatureType: SignatureTypeUNEP,
				Type:          SignerTypeFromSchema,
				Signer:        "test;test",
			},
			wantErr: false,
		},
		{
			name: "UNEP - schema group bad rule",
			fields: fields{
				SignatureType: SignatureTypeUNEP,
				Type:          SignerTypeFromSchema,
				Signer:        "test;test",
				Rule:          "bad rule",
			},
			wantErr: true,
		},
		{
			name: "UNEP - bad type",
			fields: fields{
				SignatureType: SignatureTypeUNEP,
				Type:          "test",
			},
			wantErr: true,
		},
		{
			name: "UKEP - user",
			fields: fields{
				SignatureType:    SignatureTypeUKEP,
				Type:             SignerTypeUser,
				Signer:           "test",
				SignatureCarrier: "all",
			},
			wantErr: false,
		},
		{
			name: "UKEP - no user",
			fields: fields{
				SignatureType:    SignatureTypeUKEP,
				Type:             SignerTypeUser,
				Signer:           "",
				SignatureCarrier: "all",
			},
			wantErr: true,
		},
		{
			name: "UKEP - user",
			fields: fields{
				SignatureType:    SignatureTypeUKEP,
				Type:             SignerTypeUser,
				Signer:           "test",
				SignatureCarrier: "all",
			},
			wantErr: false,
		},
		{
			name: "UKEP - group id",
			fields: fields{
				SignatureType:    SignatureTypeUKEP,
				Type:             SignerTypeGroup,
				SignerGroupID:    "test",
				SignatureCarrier: "all",
			},
			wantErr: false,
		},
		{
			name: "UKEP - group id path",
			fields: fields{
				SignatureType:     SignatureTypeUKEP,
				Type:              SignerTypeGroup,
				SignerGroupIDPath: "test",
				SignatureCarrier:  "all",
			},
			wantErr: false,
		},
		{
			name: "UKEP - group bad rule",
			fields: fields{
				SignatureType:     SignatureTypeUKEP,
				Type:              SignerTypeGroup,
				SignerGroupIDPath: "test",
				Rule:              "bad rule",
				SignatureCarrier:  "all",
			},
			wantErr: true,
		},
		{
			name: "UKEP - no group",
			fields: fields{
				SignatureType:    SignatureTypeUKEP,
				Type:             SignerTypeGroup,
				SignatureCarrier: "all",
			},
			wantErr: true,
		},
		{
			name: "UKEP - schema user",
			fields: fields{
				SignatureType:    SignatureTypeUKEP,
				Type:             SignerTypeFromSchema,
				Signer:           "test",
				SignatureCarrier: "all",
			},
			wantErr: false,
		},
		{
			name: "UKEP - schema no user",
			fields: fields{
				SignatureType:    SignatureTypeUKEP,
				Type:             SignerTypeFromSchema,
				Signer:           "",
				SignatureCarrier: "all",
			},
			wantErr: true,
		},
		{
			name: "UKEP - schema group",
			fields: fields{
				SignatureType:    SignatureTypeUKEP,
				Type:             SignerTypeFromSchema,
				Signer:           "test;test",
				SignatureCarrier: "all",
			},
			wantErr: false,
		},
		{
			name: "UKEP - schema group bad rule",
			fields: fields{
				SignatureType:    SignatureTypeUKEP,
				Type:             SignerTypeFromSchema,
				Signer:           "test;test",
				Rule:             "bad rule",
				SignatureCarrier: "all",
			},
			wantErr: true,
		},
		{
			name: "UKEP - bad type",
			fields: fields{
				SignatureType:    SignatureTypeUKEP,
				Type:             "test",
				SignatureCarrier: "all",
			},
			wantErr: true,
		},
		{
			name: "UKEP - all",
			fields: fields{
				SignatureType:    SignatureTypeUKEP,
				Type:             SignerTypeUser,
				Signer:           "test",
				SignatureCarrier: "all",
			},
			wantErr: false,
		},
		{
			name: "UKEP - cloud",
			fields: fields{
				SignatureType:    SignatureTypeUKEP,
				Type:             SignerTypeUser,
				Signer:           "test",
				SignatureCarrier: "cloud",
			},
			wantErr: false,
		},
		{
			name: "UKEP - token",
			fields: fields{
				SignatureType:    SignatureTypeUKEP,
				Type:             SignerTypeUser,
				Signer:           "test",
				SignatureCarrier: "token",
			},
			wantErr: false,
		},
		{
			name: "UKEP - bad carrier",
			fields: fields{
				SignatureType:    SignatureTypeUKEP,
				Type:             SignerTypeUser,
				Signer:           "test",
				SignatureCarrier: "bad",
			},
			wantErr: true,
		},
		{
			name: "bad type",
			fields: fields{
				SignatureType: "test",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &SignParams{
				Type:              tt.fields.Type,
				SigningRule:       tt.fields.Rule,
				Signer:            tt.fields.Signer,
				SignatureType:     tt.fields.SignatureType,
				SignatureCarrier:  tt.fields.SignatureCarrier,
				SignerGroupID:     tt.fields.SignerGroupID,
				SignerGroupIDPath: tt.fields.SignerGroupIDPath,
			}
			if err := a.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("%v ValidateSchemas()", a)
			}
		})
	}
}
