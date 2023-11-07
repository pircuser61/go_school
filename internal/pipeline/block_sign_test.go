package pipeline

import (
	"testing"
	"time"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

func TestSignData_SetDecision(t *testing.T) {
	const (
		login   = "example"
		login2  = "example2"
		comment = "test"

		fileID1 = "uuid1"

		invalidLogin = "foobar"
	)

	type fields struct {
		Signers          map[string]struct{}
		Decision         SignDecision
		ActualSigner     string
		SigningRule      script.SigningRule
		SignLog          []SignLogEntry
		SignatureType    script.SignatureType
		SignatureCarrier script.SignatureCarrier
	}
	type args struct {
		login       string
		decision    SignDecision
		comment     string
		attachments []entity.Attachment
	}
	tests := []struct {
		name             string
		fields           fields
		args             args
		wantErr          bool
		expectedDecision SignDecision
	}{
		{
			name: "signer service account ukep",
			fields: fields{
				Signers: map[string]struct{}{
					login: {},
				},
				SignatureType: script.SignatureTypeUKEP,
			},
			args: args{
				login:       ServiceAccountDev,
				decision:    SignDecisionSigned,
				comment:     comment,
				attachments: []entity.Attachment{{FileID: fileID1}},
			},
			wantErr: false,
		},
		{
			name: "bad decision",
			fields: fields{
				Signers: map[string]struct{}{
					login: {},
				},
			},
			args: args{
				login:    invalidLogin,
				decision: "bad",
				comment:  comment,
			},
			wantErr: true,
		},
		{
			name: "no decision",
			fields: fields{
				Signers: map[string]struct{}{
					login: {},
				},
			},
			args: args{
				login:   invalidLogin,
				comment: comment,
			},
			wantErr: true,
		},
		{
			name: "decision already set",
			fields: fields{
				Signers: map[string]struct{}{
					login: {},
				},
				Decision: SignDecisionRejected,
			},
			args: args{
				login:    login,
				decision: SignDecisionSigned,
				comment:  comment,
			},
			wantErr:          true,
			expectedDecision: SignDecisionRejected,
		},
		{
			name: "no attachments ukep token",
			fields: fields{
				Signers: map[string]struct{}{
					login: {},
				},
				SignatureType:    script.SignatureTypeUKEP,
				SignatureCarrier: script.SignatureCarrierToken,
			},
			args: args{
				login:    ServiceAccountDev,
				decision: SignDecisionSigned,
				comment:  comment,
			},
			wantErr: true,
		},
		{
			name: "attachments ukep not token",
			fields: fields{
				Signers: map[string]struct{}{
					login: {},
				},
				SignatureType:    script.SignatureTypeUKEP,
				SignatureCarrier: script.SignatureCarrierAll,
			},
			args: args{
				login:    ServiceAccountDev,
				decision: SignDecisionSigned,
				comment:  comment,
			},
			wantErr: false,
		},
		{
			name: "attachments ukep token",
			fields: fields{
				Signers: map[string]struct{}{
					login: {},
				},
				SignatureType:    script.SignatureTypeUKEP,
				SignatureCarrier: script.SignatureCarrierToken,
			},
			args: args{
				login:       ServiceAccountDev,
				decision:    SignDecisionSigned,
				comment:     comment,
				attachments: []entity.Attachment{{FileID: fileID1}},
			},
			wantErr: false,
		},
		{
			name: "decision signed one user",
			fields: fields{
				Signers: map[string]struct{}{
					login: {},
				},
				SigningRule: script.AnyOfSigningRequired,
			},
			args: args{
				login:    login,
				decision: SignDecisionSigned,
				comment:  comment,
			},
			wantErr:          false,
			expectedDecision: SignDecisionSigned,
		},
		{
			name: "decision rejected many users",
			fields: fields{
				Signers: map[string]struct{}{
					login:  {},
					login2: {},
				},
				SigningRule: script.AllOfSigningRequired,
			},
			args: args{
				login:    login,
				decision: SignDecisionRejected,
				comment:  comment,
			},
			wantErr:          false,
			expectedDecision: SignDecisionRejected,
		},
		{
			name: "decision rejected ukep",
			fields: fields{
				Signers: map[string]struct{}{
					login:  {},
					login2: {},
				},
				SigningRule:   script.AnyOfSigningRequired,
				SignatureType: script.SignatureTypeUKEP,
			},
			args: args{
				login:    login,
				decision: SignDecisionRejected,
				comment:  comment,
			},
			wantErr:          false,
			expectedDecision: SignDecisionRejected,
		},
		{
			name: "decision error many users",
			fields: fields{
				Signers: map[string]struct{}{
					login:  {},
					login2: {},
				},
				SigningRule: script.AllOfSigningRequired,
			},
			args: args{
				login:    login,
				decision: SignDecisionError,
				comment:  comment,
			},
			wantErr:          false,
			expectedDecision: SignDecisionError,
		},
		{
			name: "decision not final many users",
			fields: fields{
				Signers: map[string]struct{}{
					login:  {},
					login2: {},
				},
				SigningRule: script.AllOfSigningRequired,
			},
			args: args{
				login:    login,
				decision: SignDecisionSigned,
				comment:  comment,
			},
			wantErr: false,
		},
		{
			name: "decision already set by user",
			fields: fields{
				Signers: map[string]struct{}{
					login:  {},
					login2: {},
				},
				SigningRule: script.AllOfSigningRequired,
				SignLog: []SignLogEntry{
					{
						Login:     login,
						Decision:  SignDecisionSigned,
						Comment:   comment,
						CreatedAt: time.Time{},
					},
				},
			},
			args: args{
				login:    login,
				decision: SignDecisionSigned,
				comment:  comment,
			},
			wantErr: true,
		},
		{
			name: "decision finalize many users",
			fields: fields{
				Signers: map[string]struct{}{
					login:  {},
					login2: {},
				},
				SigningRule: script.AllOfSigningRequired,
				SignLog: []SignLogEntry{
					{
						Login:     login2,
						Decision:  SignDecisionSigned,
						Comment:   comment,
						CreatedAt: time.Time{},
					},
				},
			},
			args: args{
				login:    login,
				decision: SignDecisionSigned,
				comment:  comment,
			},
			wantErr:          false,
			expectedDecision: SignDecisionSigned,
		},
		{
			name: "decision anyof many users",
			fields: fields{
				Signers: map[string]struct{}{
					login:  {},
					login2: {},
				},
				SigningRule: script.AnyOfSigningRequired,
			},
			args: args{
				login:    login,
				decision: SignDecisionSigned,
				comment:  comment,
			},
			wantErr:          false,
			expectedDecision: SignDecisionSigned,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &SignData{
				Signers:          tt.fields.Signers,
				SignLog:          tt.fields.SignLog,
				SigningRule:      tt.fields.SigningRule,
				SignatureType:    tt.fields.SignatureType,
				SignatureCarrier: tt.fields.SignatureCarrier,
			}
			if tt.fields.Decision != "" {
				a.Decision = &tt.fields.Decision
			}

			if err := a.SetDecision(tt.args.login, &signSignatureParams{
				Decision:    tt.args.decision,
				Comment:     tt.args.comment,
				Attachments: tt.args.attachments,
			}); (err != nil) != tt.wantErr {
				t.Errorf(
					"SetDecision(%v, %v, %v), error: %v",
					tt.args.login,
					tt.args.decision,
					tt.args.comment,
					err,
				)
			}
			if a.Decision != nil && *a.Decision != tt.expectedDecision {
				t.Errorf(
					"SetDecision: expected %v, got %v)",
					tt.expectedDecision,
					a.Decision,
				)
			}
		})
	}
}
