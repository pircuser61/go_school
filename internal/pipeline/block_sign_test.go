package pipeline

import (
	"bytes"
	"context"
	c "context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/humantasks/nocache"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db/mocks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/httpclient"
	human_tasks "gitlab.services.mts.ru/jocasta/pipeliner/internal/humantasks"
	mocks2 "gitlab.services.mts.ru/jocasta/pipeliner/internal/humantasks/mocks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"
	peopleMock "gitlab.services.mts.ru/jocasta/pipeliner/internal/people/mocks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc"
	serviceDeskMocks "gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc/mocks"
	sd_nocache "gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc/nocache"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sla"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"

	"github.com/hashicorp/go-retryablehttp"

	cachekit "gitlab.services.mts.ru/jocasta/cache-kit"

	delegationht "gitlab.services.mts.ru/jocasta/human-tasks/pkg/proto/gen/proto/go/delegation"
)

type PeopleServiceTest struct {
	SearchURL string
	Cli       *retryablehttp.Client `json:"-"`
	Sso       *sso.Service
	Cache     cachekit.Cache
}

func getTaskRunContext() db.Database {
	res := &mocks.MockedDatabase{}

	res.On("GetAttach", nil).Return(nil, nil)
	res.On("GetTaskRunContext", c.Background(), "J001").Return(entity.TaskRunContext{}, nil)
	res.On("GetApplicationData", "J001").Return("", nil)
	res.On("GetAdditionalDescriptionForms", "J001", "sign").Return([]entity.DescriptionForm{}, nil)
	res.On("UpdateStepContext",
		mock.MatchedBy(func(ctx c.Context) bool { return true }),
		mock.AnythingOfType("*db.UpdateStepRequest"),
	).Return(
		nil,
	)

	return res
}

func TestSignData_SetDecision(t *testing.T) {
	const (
		login   = "example"
		login2  = "example2"
		comment = "test"

		fileID1 = "uuid1"

		invalidLogin = "foobar"
	)

	type (
		fields struct {
			Signers          map[string]struct{}
			Decision         SignDecision
			ActualSigner     string
			SigningRule      script.SigningRule
			SignLog          []SignLogEntry
			SignatureType    script.SignatureType
			SignatureCarrier script.SignatureCarrier
		}
		args struct {
			login       string
			decision    SignDecision
			comment     string
			attachments []entity.Attachment
			signatures  []fileSignature
		}
	)

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
				login:      ServiceAccountDev,
				decision:   SignDecisionSigned,
				comment:    comment,
				signatures: []fileSignature{{FileID: fileID1}},
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
						LogType:   SignerLogDecision,
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
				Signatures:  tt.args.signatures,
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

func TestGoSignBlock_createGoSignBlock(t *testing.T) {
	const (
		example    = "example"
		title      = "title"
		shortTitle = "Нода Подписания"
	)

	varStore := store.NewStore()

	workTypeVal := "8/5"
	slaVal := 8

	varStore.SetValue("form_0.user", map[string]interface{}{
		"username": "test",
		"fullname": "test test test",
	})
	varStore.SetValue("form_1.user", map[string]interface{}{
		"username": "test2",
		"fullname": "test2 test test",
	})

	next := []entity.Socket{
		{
			ID:           DefaultSocketID,
			Title:        script.DefaultSocketTitle,
			NextBlockIds: []string{"next_0"},
		},
		{
			ID:           rejectedSocketID,
			Title:        script.RejectSocketTitle,
			NextBlockIds: []string{"next_1"},
		},
	}

	type args struct {
		name   string
		ef     *entity.EriusFunc
		runCtx *BlockRunContext
	}

	tests := []struct {
		name string
		args args
		want *GoSignBlock
	}{
		{
			name: "no sign params",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType:  BlockGoSignID,
					Sockets:    next,
					Input:      nil,
					Output:     nil,
					Params:     nil,
					Title:      title,
					ShortTitle: shortTitle,
				},
				runCtx: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
				},
			},
			want: nil,
		},
		{
			name: "invalid execution params",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType:  BlockGoSignID,
					Sockets:    next,
					Input:      nil,
					Output:     nil,
					Params:     []byte("{}"),
					Title:      title,
					ShortTitle: shortTitle,
				},
				runCtx: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
				},
			},
			want: nil,
		},
		{
			name: "SignatureCarrierAll and SignatureTypeUNEP",
			args: args{
				name: example,
				runCtx: &BlockRunContext{
					skipNotifications: true,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						r, _ := json.Marshal(&SignData{
							Signers: map[string]struct{}{
								"tester": {},
							},
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
							SignatureType:      script.SignatureTypeUNEP,
							SignatureCarrier:   script.SignatureCarrierAll,
							Type:               script.SignerTypeUser,
							SigningRule:        script.AllOfSigningRequired,
							Reentered:          false,
						})
						s.State = map[string]json.RawMessage{
							example: r,
						}

						return s
					}(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
					},
				},
				ef: &entity.EriusFunc{
					BlockType:  BlockGoSignID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
					Input: []entity.EriusFunctionValue{
						{
							Name:   "foo",
							Type:   "string",
							Global: "bar",
						},
					},
					Output: &script.JSONSchema{
						Type: "object",
						Properties: map[string]script.JSONSchemaPropertiesValue{
							"foo": {
								Type:   "string",
								Global: "bar",
							},
							"empty_global": {
								Type:   "string",
								Global: "",
							},
						},
					},
					Params: func() []byte {
						r, _ := json.Marshal(&script.SignParams{
							SignatureType:      script.SignatureTypeUNEP,
							SignatureCarrier:   script.SignatureCarrierAll,
							Type:               script.SignerTypeUser,
							Signer:             "tester",
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})

						return r
					}(),
				},
			},
			want: &GoSignBlock{
				Name:      "example",
				ShortName: "Нода Подписания",
				Title:     "title",
				Input:     map[string]string{"foo": "bar"},
				Output:    map[string]string{"foo": "bar"},
				Sockets: []script.Socket{
					{
						ID:           "default",
						Title:        "Выход по умолчанию",
						NextBlockIds: []string{"next_0"},
						ActionType:   "",
					},
					{
						ID:           "rejected",
						Title:        "Отклонить",
						NextBlockIds: []string{"next_1"},
						ActionType:   "",
					},
				},
				State: &SignData{
					Attachments:         make([]entity.Attachment, 0),
					AdditionalApprovers: make([]AdditionalSignApprover, 0),
					Type:                "user",
					Signers: map[string]struct{}{
						"tester": {},
					},
					SignatureType:      "unep",
					SigningRule:        "AnyOf",
					Signatures:         []FileSignaturePair{},
					SignatureCarrier:   "all",
					SignLog:            []SignLogEntry{},
					FormsAccessibility: []script.FormAccessibility{{}},
					Reentered:          true,
					WorkType:           &workTypeVal,
					SLA:                &slaVal,
					Deadline:           time.Date(0001, 01, 01, 14, 00, 00, 00, time.UTC),
				},
				RunContext: &BlockRunContext{
					TaskID:      uuid.MustParse("00000000-0000-0000-0000-000000000000"),
					IsTest:      false,
					Delegations: human_tasks.Delegations(nil),
					VarStore: &store.VariableStore{
						Mutex: sync.Mutex{},
						State: map[string]json.RawMessage{
							"example": unmarshalFromTestFile(t, "testdata/signing_params/signing_state_signing_params.json"),
						},
						Values:     map[string]interface{}{},
						Steps:      []string{"example"},
						Errors:     []string{},
						StopPoints: store.StopPoints{},
					},
					CurrBlockStartTime: time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
					skipNotifications:  true,
					Services:           RunContextServices{},
					TaskSubscriptionData: TaskSubscriptionData{
						NotificationSchema: script.JSONSchema{},
					},
				},
				happenedEvents: []entity.NodeEvent{},
			},
		},
		{
			name: "SignatureCarrierAll and SignatureTypePEP",
			args: args{
				name: example,
				runCtx: &BlockRunContext{
					skipNotifications: true,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						r, _ := json.Marshal(&SignData{
							Type: script.SignerTypeUser,
							Signers: map[string]struct{}{
								"tester": {},
							},
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})
						s.State = map[string]json.RawMessage{
							example: r,
						}

						return s
					}(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
					},
				},
				ef: &entity.EriusFunc{
					BlockType:  BlockGoSignID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
					Input: []entity.EriusFunctionValue{
						{
							Name:   "foo",
							Type:   "string",
							Global: "bar",
						},
					},
					Output: &script.JSONSchema{
						Type: "object",
						Properties: map[string]script.JSONSchemaPropertiesValue{
							"foo": {
								Type:   "string",
								Global: "bar",
							},
							"empty_global": {
								Type:   "string",
								Global: "",
							},
						},
					},
					Params: func() []byte {
						r, _ := json.Marshal(&script.SignParams{
							SignatureType:      script.SignatureTypePEP,
							SignatureCarrier:   script.SignatureCarrierAll,
							Type:               script.SignerTypeUser,
							Signer:             "tester",
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})

						return r
					}(),
				},
			},
			want: &GoSignBlock{
				Name:      "example",
				ShortName: "Нода Подписания",
				Title:     "title",
				Input:     map[string]string{"foo": "bar"},
				Output:    map[string]string{"foo": "bar"},
				Sockets: []script.Socket{
					{
						ID:           "default",
						Title:        "Выход по умолчанию",
						NextBlockIds: []string{"next_0"},
						ActionType:   "",
					},
					{
						ID:           "rejected",
						Title:        "Отклонить",
						NextBlockIds: []string{"next_1"},
						ActionType:   "",
					},
				},
				State: &SignData{
					Attachments:         make([]entity.Attachment, 0),
					AdditionalApprovers: make([]AdditionalSignApprover, 0),
					Type:                "user",
					Signers: map[string]struct{}{
						"tester": {},
					},
					SignatureType:      "pep",
					SigningRule:        "AnyOf",
					Signatures:         []FileSignaturePair{},
					SignatureCarrier:   "all",
					SignLog:            []SignLogEntry{},
					FormsAccessibility: []script.FormAccessibility{{}},
					Reentered:          true,
					WorkType:           &workTypeVal,
					SLA:                &slaVal,
					Deadline:           time.Date(0001, 01, 01, 14, 00, 00, 00, time.UTC),
				},
				RunContext: &BlockRunContext{
					TaskID:      uuid.MustParse("00000000-0000-0000-0000-000000000000"),
					IsTest:      false,
					Delegations: human_tasks.Delegations(nil),
					VarStore: &store.VariableStore{
						Mutex: sync.Mutex{},
						State: map[string]json.RawMessage{
							"example": unmarshalFromTestFile(t, "testdata/signing_params/signing_state_signing_params.json"),
						},
						Values:     map[string]interface{}{},
						Steps:      []string{"example"},
						Errors:     []string{},
						StopPoints: store.StopPoints{},
					},
					CurrBlockStartTime: time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
					skipNotifications:  true,
					Services:           RunContextServices{},
					TaskSubscriptionData: TaskSubscriptionData{
						NotificationSchema: script.JSONSchema{},
					},
				},
				happenedEvents: []entity.NodeEvent{},
			},
		},
		{
			name: "SignatureCarrierAll and SignatureTypeUKEP signingParams",
			args: args{
				name: example,
				runCtx: &BlockRunContext{
					skipNotifications: true,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						s.SetValue("form_3.inn", "inn_1")
						s.SetValue("form_3.snils", "snils_1")
						s.SetValue("form_3.files", []entity.Attachment{
							{FileID: "uuid1"},
							{FileID: "uuid2"},
						})
						r, _ := json.Marshal(&SignData{
							Type: script.SignerTypeUser,
							Signers: map[string]struct{}{
								"tester": {},
							},
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})
						s.State = map[string]json.RawMessage{
							example: r,
						}

						return s
					}(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
					},
				},
				ef: &entity.EriusFunc{
					BlockType:  BlockGoSignID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
					Input: []entity.EriusFunctionValue{
						{
							Name:   "foo",
							Type:   "string",
							Global: "bar",
						},
					},
					Output: &script.JSONSchema{
						Type: "object",
						Properties: map[string]script.JSONSchemaPropertiesValue{
							"foo": {
								Type:   "string",
								Global: "bar",
							},
							"empty_global": {
								Type:   "string",
								Global: "",
							},
						},
					},
					Params: func() []byte {
						r, _ := json.Marshal(&script.SignParams{
							SignatureType:    script.SignatureTypeUKEP,
							SignatureCarrier: script.SignatureCarrierAll,
							Type:             script.SignerTypeUser,
							SigningParamsPaths: script.SigningParamsPaths{
								INN:   "form_3.inn",
								SNILS: "form_3.snils",
								Files: []string{"form_3.files"},
							},
							Signer:             "tester",
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})

						return r
					}(),
				},
			},
			want: &GoSignBlock{
				Name:      "example",
				ShortName: "Нода Подписания",
				Title:     "title",
				Input:     map[string]string{"foo": "bar"},
				Output:    map[string]string{"foo": "bar"},
				Sockets: []script.Socket{
					{
						ID:           "default",
						Title:        "Выход по умолчанию",
						NextBlockIds: []string{"next_0"},
						ActionType:   "",
					},
					{
						ID:           "rejected",
						Title:        "Отклонить",
						NextBlockIds: []string{"next_1"},
						ActionType:   "",
					},
				},
				State: &SignData{
					Attachments:         make([]entity.Attachment, 0),
					AdditionalApprovers: make([]AdditionalSignApprover, 0),
					Type:                "user",
					Signers: map[string]struct{}{
						"tester": {},
					},
					SignatureType:      "ukep",
					SigningRule:        "AnyOf",
					Signatures:         []FileSignaturePair{},
					SignatureCarrier:   "all",
					SignLog:            []SignLogEntry{},
					FormsAccessibility: []script.FormAccessibility{{}},
					Reentered:          true,
					SigningParams: SigningParams{
						INN:   "inn_1",
						SNILS: "snils_1",
						Files: []entity.Attachment{
							{FileID: "uuid1"},
							{FileID: "uuid2"},
						},
					},
					SigningParamsPaths: script.SigningParamsPaths{
						INN:   "form_3.inn",
						SNILS: "form_3.snils",
						Files: []string{"form_3.files"},
					},
					WorkType: &workTypeVal,
					SLA:      &slaVal,
					Deadline: time.Date(0001, 01, 01, 14, 00, 00, 00, time.UTC),
				},
				RunContext: &BlockRunContext{
					TaskID:      uuid.MustParse("00000000-0000-0000-0000-000000000000"),
					IsTest:      false,
					Delegations: human_tasks.Delegations(nil),
					VarStore: &store.VariableStore{
						Mutex: sync.Mutex{},
						State: map[string]json.RawMessage{
							"example": unmarshalFromTestFile(t, "testdata/signing_params/signing_state_signing_params.json"),
						},
						Values: map[string]interface{}{
							"form_3.snils": "snils_1",
							"form_3.inn":   "inn_1",
							"form_3.files": []interface{}{
								map[string]interface{}{"file_id": "uuid1"},
								map[string]interface{}{"file_id": "uuid2"},
							},
						},
						Steps:      []string{"example"},
						Errors:     []string{},
						StopPoints: store.StopPoints{},
					},
					CurrBlockStartTime: time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
					skipNotifications:  true,
					Services:           RunContextServices{},
					TaskSubscriptionData: TaskSubscriptionData{
						NotificationSchema: script.JSONSchema{},
					},
				},
				happenedEvents: []entity.NodeEvent{},
			},
		},
		{
			name: "SignatureCarrierAll and SignatureTypeUNEP",
			args: args{
				name: example,
				runCtx: &BlockRunContext{
					skipNotifications: true,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						r, _ := json.Marshal(&SignData{
							Type: script.SignerTypeUser,
							Signers: map[string]struct{}{
								"tester": {},
							},
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})
						s.State = map[string]json.RawMessage{
							example: r,
						}

						return s
					}(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
					},
				},
				ef: &entity.EriusFunc{
					BlockType:  BlockGoSignID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
					Input: []entity.EriusFunctionValue{
						{
							Name:   "foo",
							Type:   "string",
							Global: "bar",
						},
					},
					Output: &script.JSONSchema{
						Type: "object",
						Properties: map[string]script.JSONSchemaPropertiesValue{
							"foo": {
								Type:   "string",
								Global: "bar",
							},
							"empty_global": {
								Type:   "string",
								Global: "",
							},
						},
					},
					Params: func() []byte {
						r, _ := json.Marshal(&script.SignParams{
							SignatureType:      script.SignatureTypeUNEP,
							SignatureCarrier:   script.SignatureCarrierToken,
							Type:               script.SignerTypeUser,
							Signer:             "tester",
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})

						return r
					}(),
				},
			},
			want: &GoSignBlock{
				Name:      "example",
				ShortName: "Нода Подписания",
				Title:     "title",
				Input:     map[string]string{"foo": "bar"},
				Output:    map[string]string{"foo": "bar"},
				Sockets: []script.Socket{
					{
						ID:           "default",
						Title:        "Выход по умолчанию",
						NextBlockIds: []string{"next_0"},
						ActionType:   "",
					},
					{
						ID:           "rejected",
						Title:        "Отклонить",
						NextBlockIds: []string{"next_1"},
						ActionType:   "",
					},
				},
				State: &SignData{
					Attachments:         make([]entity.Attachment, 0),
					AdditionalApprovers: make([]AdditionalSignApprover, 0),
					Type:                "user",
					Signers: map[string]struct{}{
						"tester": {},
					},
					SignatureType:      "unep",
					SigningRule:        "AnyOf",
					Signatures:         []FileSignaturePair{},
					SignatureCarrier:   "token",
					SignLog:            []SignLogEntry{},
					FormsAccessibility: []script.FormAccessibility{{}},
					Reentered:          true,
					WorkType:           &workTypeVal,
					SLA:                &slaVal,
					Deadline:           time.Date(0001, 01, 01, 14, 00, 00, 00, time.UTC),
				},
				RunContext: &BlockRunContext{
					TaskID:      uuid.MustParse("00000000-0000-0000-0000-000000000000"),
					IsTest:      false,
					Delegations: human_tasks.Delegations(nil),
					VarStore: &store.VariableStore{
						Mutex: sync.Mutex{},
						State: map[string]json.RawMessage{
							"example": unmarshalFromTestFile(t, "testdata/signing_params/signing_state_signing_params.json"),
						},
						Values:     map[string]interface{}{},
						Steps:      []string{"example"},
						Errors:     []string{},
						StopPoints: store.StopPoints{},
					},
					CurrBlockStartTime: time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
					skipNotifications:  true,
					Services:           RunContextServices{},
					TaskSubscriptionData: TaskSubscriptionData{
						NotificationSchema: script.JSONSchema{},
					},
				},
				happenedEvents: []entity.NodeEvent{},
			},
		},
		{
			name: "SignatureCarrierToken and SignatureTypePEP",
			args: args{
				name: example,
				runCtx: &BlockRunContext{
					skipNotifications: true,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						r, _ := json.Marshal(&SignData{
							Type: script.SignerTypeUser,
							Signers: map[string]struct{}{
								"tester": {},
							},
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})
						s.State = map[string]json.RawMessage{
							example: r,
						}

						return s
					}(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
					},
				},
				ef: &entity.EriusFunc{
					BlockType:  BlockGoSignID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
					Input: []entity.EriusFunctionValue{
						{
							Name:   "foo",
							Type:   "string",
							Global: "bar",
						},
					},
					Output: &script.JSONSchema{
						Type: "object",
						Properties: map[string]script.JSONSchemaPropertiesValue{
							"foo": {
								Type:   "string",
								Global: "bar",
							},
							"empty_global": {
								Type:   "string",
								Global: "",
							},
						},
					},
					Params: func() []byte {
						r, _ := json.Marshal(&script.SignParams{
							SignatureType:      script.SignatureTypePEP,
							SignatureCarrier:   script.SignatureCarrierToken,
							Type:               script.SignerTypeUser,
							Signer:             "tester",
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})

						return r
					}(),
				},
			},
			want: &GoSignBlock{
				Name:      "example",
				ShortName: "Нода Подписания",
				Title:     "title",
				Input:     map[string]string{"foo": "bar"},
				Output:    map[string]string{"foo": "bar"},
				Sockets: []script.Socket{
					{
						ID:           "default",
						Title:        "Выход по умолчанию",
						NextBlockIds: []string{"next_0"},
						ActionType:   "",
					},
					{
						ID:           "rejected",
						Title:        "Отклонить",
						NextBlockIds: []string{"next_1"},
						ActionType:   "",
					},
				},
				State: &SignData{
					Attachments:         make([]entity.Attachment, 0),
					AdditionalApprovers: make([]AdditionalSignApprover, 0),
					Type:                "user",
					Signers: map[string]struct{}{
						"tester": {},
					},
					SignatureType:      "pep",
					SigningRule:        "AnyOf",
					Signatures:         []FileSignaturePair{},
					SignatureCarrier:   "token",
					SignLog:            []SignLogEntry{},
					FormsAccessibility: []script.FormAccessibility{{}},
					Reentered:          true,
					WorkType:           &workTypeVal,
					SLA:                &slaVal,
					Deadline:           time.Date(0001, 01, 01, 14, 00, 00, 00, time.UTC),
				},
				RunContext: &BlockRunContext{
					TaskID:      uuid.MustParse("00000000-0000-0000-0000-000000000000"),
					IsTest:      false,
					Delegations: human_tasks.Delegations(nil),
					VarStore: &store.VariableStore{
						Mutex: sync.Mutex{},
						State: map[string]json.RawMessage{
							"example": unmarshalFromTestFile(t, "testdata/signing_params/signing_state_signing_params.json"),
						},
						Values:     map[string]interface{}{},
						Steps:      []string{"example"},
						Errors:     []string{},
						StopPoints: store.StopPoints{},
					},
					CurrBlockStartTime: time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
					skipNotifications:  true,
					Services:           RunContextServices{},
					TaskSubscriptionData: TaskSubscriptionData{
						NotificationSchema: script.JSONSchema{},
					},
				},
				happenedEvents: []entity.NodeEvent{},
			},
		},
		{
			name: "SignatureCarrierToken and SignatureTypeUKEP",
			args: args{
				name: example,
				runCtx: &BlockRunContext{
					skipNotifications: true,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						s.SetValue("form_3.inn", "inn_1")
						s.SetValue("form_3.snils", "snils_1")
						s.SetValue("form_3.files", []entity.Attachment{
							{FileID: "uuid1"},
							{FileID: "uuid2"},
						})
						r, _ := json.Marshal(&SignData{
							Type: script.SignerTypeUser,
							Signers: map[string]struct{}{
								"tester": {},
							},
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})
						s.State = map[string]json.RawMessage{
							example: r,
						}

						return s
					}(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
					},
				},
				ef: &entity.EriusFunc{
					BlockType:  BlockGoSignID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
					Input: []entity.EriusFunctionValue{
						{
							Name:   "foo",
							Type:   "string",
							Global: "bar",
						},
					},
					Output: &script.JSONSchema{
						Type: "object",
						Properties: map[string]script.JSONSchemaPropertiesValue{
							"foo": {
								Type:   "string",
								Global: "bar",
							},
							"empty_global": {
								Type:   "string",
								Global: "",
							},
						},
					},
					Params: func() []byte {
						r, _ := json.Marshal(&script.SignParams{
							SignatureType:    script.SignatureTypeUKEP,
							SignatureCarrier: script.SignatureCarrierToken,
							Type:             script.SignerTypeUser,
							SigningParamsPaths: script.SigningParamsPaths{
								INN:   "form_3.inn",
								SNILS: "form_3.snils",
								Files: []string{"form_3.files"},
							},
							Signer:             "tester",
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})

						return r
					}(),
				},
			},
			want: &GoSignBlock{
				Name:      "example",
				ShortName: "Нода Подписания",
				Title:     "title",
				Input:     map[string]string{"foo": "bar"},
				Output:    map[string]string{"foo": "bar"},
				Sockets: []script.Socket{
					{
						ID:           "default",
						Title:        "Выход по умолчанию",
						NextBlockIds: []string{"next_0"},
						ActionType:   "",
					},
					{
						ID:           "rejected",
						Title:        "Отклонить",
						NextBlockIds: []string{"next_1"},
						ActionType:   "",
					},
				},
				State: &SignData{
					Attachments:         make([]entity.Attachment, 0),
					AdditionalApprovers: make([]AdditionalSignApprover, 0),
					Type:                "user",
					Signers: map[string]struct{}{
						"tester": {},
					},
					SignatureType: "ukep",
					SigningRule:   "AnyOf",
					Signatures:    []FileSignaturePair{},
					SigningParams: SigningParams{
						INN:   "inn_1",
						SNILS: "snils_1",
						Files: []entity.Attachment{
							{FileID: "uuid1"},
							{FileID: "uuid2"},
						},
					},
					SigningParamsPaths: script.SigningParamsPaths{
						INN:   "form_3.inn",
						SNILS: "form_3.snils",
						Files: []string{"form_3.files"},
					},
					SignatureCarrier:   "token",
					SignLog:            []SignLogEntry{},
					FormsAccessibility: []script.FormAccessibility{{}},
					Reentered:          true,
					WorkType:           &workTypeVal,
					SLA:                &slaVal,
					Deadline:           time.Date(0001, 01, 01, 14, 00, 00, 00, time.UTC),
				},
				RunContext: &BlockRunContext{
					TaskID:      uuid.MustParse("00000000-0000-0000-0000-000000000000"),
					IsTest:      false,
					Delegations: human_tasks.Delegations(nil),
					VarStore: &store.VariableStore{
						Mutex: sync.Mutex{},
						State: map[string]json.RawMessage{
							"example": unmarshalFromTestFile(t, "testdata/signing_params/signing_state_signing_params.json"),
						},
						Values: map[string]interface{}{
							"form_3.snils": "snils_1",
							"form_3.inn":   "inn_1",
							"form_3.files": []interface{}{
								map[string]interface{}{"file_id": "uuid1"},
								map[string]interface{}{"file_id": "uuid2"},
							},
						},
						Steps:      []string{"example"},
						Errors:     []string{},
						StopPoints: store.StopPoints{},
					},
					CurrBlockStartTime: time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
					skipNotifications:  true,
					Services:           RunContextServices{},
					TaskSubscriptionData: TaskSubscriptionData{
						NotificationSchema: script.JSONSchema{},
					},
				},
				happenedEvents: []entity.NodeEvent{},
			},
		},
		{
			name: "SignTypeGroud and SignatureCarrierCloud and SignatureTypeUNEP",
			args: args{
				name: example,
				runCtx: &BlockRunContext{
					skipNotifications: true,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						r, _ := json.Marshal(&SignData{
							Type: script.SignerTypeUser,
							Signers: map[string]struct{}{
								"tester": {},
							},
							FormsAccessibility: make([]script.FormAccessibility, 1),
						})
						s.State = map[string]json.RawMessage{
							example: r,
						}

						return s
					}(),
				},
				ef: &entity.EriusFunc{
					BlockType:  BlockGoSignID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
					Input: []entity.EriusFunctionValue{
						{
							Name:   "foo",
							Type:   "string",
							Global: "bar",
						},
					},
					Output: &script.JSONSchema{
						Type: "object",
						Properties: map[string]script.JSONSchemaPropertiesValue{
							"foo": {
								Type:   "string",
								Global: "bar",
							},
							"empty_global": {
								Type:   "string",
								Global: "",
							},
						},
					},
					Params: func() []byte {
						r, _ := json.Marshal(&script.SignParams{
							SignatureType:      script.SignatureTypeUNEP,
							SignatureCarrier:   script.SignatureCarrierCloud,
							Type:               script.SignerTypeGroup,
							Signer:             "tester",
							FormsAccessibility: make([]script.FormAccessibility, 1),
						})

						return r
					}(),
				},
			},
			want: nil,
		},
		{
			name: "SignerTypeGroup and SignatureCarrierCloud and SignatureTypePEP",
			args: args{
				name: example,
				runCtx: &BlockRunContext{
					skipNotifications: true,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						r, _ := json.Marshal(&SignData{
							Type: script.SignerTypeUser,
							Signers: map[string]struct{}{
								"tester": {},
							},
							FormsAccessibility: make([]script.FormAccessibility, 1),
						})
						s.State = map[string]json.RawMessage{
							example: r,
						}

						return s
					}(),
				},
				ef: &entity.EriusFunc{
					BlockType:  BlockGoSignID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
					Input: []entity.EriusFunctionValue{
						{
							Name:   "foo",
							Type:   "string",
							Global: "bar",
						},
					},
					Output: &script.JSONSchema{
						Type: "object",
						Properties: map[string]script.JSONSchemaPropertiesValue{
							"foo": {
								Type:   "string",
								Global: "bar",
							},
							"empty_global": {
								Type:   "string",
								Global: "",
							},
						},
					},
					Params: func() []byte {
						r, _ := json.Marshal(&script.SignParams{
							SignatureType:      script.SignatureTypePEP,
							SignatureCarrier:   script.SignatureCarrierCloud,
							Type:               script.SignerTypeGroup,
							Signer:             "tester",
							FormsAccessibility: make([]script.FormAccessibility, 1),
						})

						return r
					}(),
				},
			},
			want: nil,
		},
		{
			name: "SignerType Group and SignatureCarrierCloud and SignatureTypeUKEP",
			args: args{
				name: example,
				runCtx: &BlockRunContext{
					skipNotifications: true,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						r, _ := json.Marshal(&SignData{
							Type: script.SignerTypeUser,
							Signers: map[string]struct{}{
								"tester": {},
							},
							FormsAccessibility: make([]script.FormAccessibility, 1),
						})
						s.State = map[string]json.RawMessage{
							example: r,
						}

						return s
					}(),
				},
				ef: &entity.EriusFunc{
					BlockType:  BlockGoSignID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
					Input: []entity.EriusFunctionValue{
						{
							Name:   "foo",
							Type:   "string",
							Global: "bar",
						},
					},
					Output: &script.JSONSchema{
						Type: "object",
						Properties: map[string]script.JSONSchemaPropertiesValue{
							"foo": {
								Type:   "string",
								Global: "bar",
							},
							"empty_global": {
								Type:   "string",
								Global: "",
							},
						},
					},
					Params: func() []byte {
						r, _ := json.Marshal(&script.SignParams{
							SignatureType:      script.SignatureTypeUKEP,
							SignatureCarrier:   script.SignatureCarrierCloud,
							Type:               script.SignerTypeGroup,
							Signer:             "tester",
							FormsAccessibility: make([]script.FormAccessibility, 1),
						})

						return r
					}(),
				},
			},
			want: nil,
		},
		{
			name: "SignerTypeUser and SignatureTypeUNEP",
			args: args{
				name: example,
				runCtx: &BlockRunContext{
					skipNotifications: true,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						r, _ := json.Marshal(&SignData{
							Type: script.SignerTypeUser,
							Signers: map[string]struct{}{
								"tester": {},
							},
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})
						s.State = map[string]json.RawMessage{
							example: r,
						}

						return s
					}(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
					},
				},
				ef: &entity.EriusFunc{
					BlockType:  BlockGoSignID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
					Input: []entity.EriusFunctionValue{
						{
							Name:   "foo",
							Type:   "string",
							Global: "bar",
						},
					},
					Output: &script.JSONSchema{
						Type: "object",
						Properties: map[string]script.JSONSchemaPropertiesValue{
							"foo": {
								Type:   "string",
								Global: "bar",
							},
							"empty_global": {
								Type:   "string",
								Global: "",
							},
						},
					},
					Params: func() []byte {
						r, _ := json.Marshal(&script.SignParams{
							SignatureType:      script.SignatureTypeUNEP,
							SignatureCarrier:   script.SignatureCarrierCloud,
							Type:               script.SignerTypeUser,
							Signer:             "tester",
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})

						return r
					}(),
				},
			},
			want: &GoSignBlock{
				Name:      "example",
				ShortName: "Нода Подписания",
				Title:     "title",
				Input:     map[string]string{"foo": "bar"},
				Output:    map[string]string{"foo": "bar"},
				Sockets: []script.Socket{
					{
						ID:           "default",
						Title:        "Выход по умолчанию",
						NextBlockIds: []string{"next_0"},
						ActionType:   "",
					},
					{
						ID:           "rejected",
						Title:        "Отклонить",
						NextBlockIds: []string{"next_1"},
						ActionType:   "",
					},
				},
				State: &SignData{
					Attachments:         make([]entity.Attachment, 0),
					AdditionalApprovers: make([]AdditionalSignApprover, 0),
					Type:                "user",
					Signers: map[string]struct{}{
						"tester": {},
					},
					SignatureType:      "unep",
					SigningRule:        "AnyOf",
					Signatures:         []FileSignaturePair{},
					SignatureCarrier:   "cloud",
					SignLog:            []SignLogEntry{},
					FormsAccessibility: []script.FormAccessibility{{}},
					Reentered:          true,
					WorkType:           &workTypeVal,
					SLA:                &slaVal,
					Deadline:           time.Date(0001, 01, 01, 14, 00, 00, 00, time.UTC),
				},
				RunContext: &BlockRunContext{
					TaskID:      uuid.MustParse("00000000-0000-0000-0000-000000000000"),
					IsTest:      false,
					Delegations: human_tasks.Delegations(nil),
					VarStore: &store.VariableStore{
						Mutex: sync.Mutex{},
						State: map[string]json.RawMessage{
							"example": unmarshalFromTestFile(t, "testdata/signing_params/signing_state_signing_params.json"),
						},
						Values:     map[string]interface{}{},
						Steps:      []string{"example"},
						Errors:     []string{},
						StopPoints: store.StopPoints{},
					},
					CurrBlockStartTime: time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
					skipNotifications:  true,
					Services:           RunContextServices{},
					TaskSubscriptionData: TaskSubscriptionData{
						NotificationSchema: script.JSONSchema{},
					},
				},
				happenedEvents: []entity.NodeEvent{},
			},
		},
		{
			name: "SignerTypeUser and SignatureTypePEP",
			args: args{
				name: example,
				runCtx: &BlockRunContext{
					skipNotifications: true,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						s.SetValue("form_3.inn", "inn_1")
						s.SetValue("form_3.snils", "snils_1")
						s.SetValue("form_3.files", []entity.Attachment{
							{FileID: "uuid1"},
							{FileID: "uuid2"},
						})
						r, _ := json.Marshal(&SignData{
							Type: script.SignerTypeUser,
							Signers: map[string]struct{}{
								"tester": {},
							},
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})
						s.State = map[string]json.RawMessage{
							example: r,
						}

						return s
					}(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
					},
				},
				ef: &entity.EriusFunc{
					BlockType:  BlockGoSignID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
					Input: []entity.EriusFunctionValue{
						{
							Name:   "foo",
							Type:   "string",
							Global: "bar",
						},
					},
					Output: &script.JSONSchema{
						Type: "object",
						Properties: map[string]script.JSONSchemaPropertiesValue{
							"foo": {
								Type:   "string",
								Global: "bar",
							},
							"empty_global": {
								Type:   "string",
								Global: "",
							},
						},
					},
					Params: func() []byte {
						r, _ := json.Marshal(&script.SignParams{
							SignatureType:    script.SignatureTypePEP,
							SignatureCarrier: script.SignatureCarrierCloud,
							Type:             script.SignerTypeUser,
							SigningParamsPaths: script.SigningParamsPaths{
								INN:   "form_3.inn",
								SNILS: "form_3.snils",
								Files: []string{"form_3.files"},
							},
							Signer:             "tester",
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})

						return r
					}(),
				},
			},
			want: &GoSignBlock{
				Name:      "example",
				ShortName: "Нода Подписания",
				Title:     "title",
				Input:     map[string]string{"foo": "bar"},
				Output:    map[string]string{"foo": "bar"},
				Sockets: []script.Socket{
					{
						ID:           "default",
						Title:        "Выход по умолчанию",
						NextBlockIds: []string{"next_0"},
						ActionType:   "",
					},
					{
						ID:           "rejected",
						Title:        "Отклонить",
						NextBlockIds: []string{"next_1"},
						ActionType:   "",
					},
				},
				State: &SignData{
					Attachments:         make([]entity.Attachment, 0),
					AdditionalApprovers: make([]AdditionalSignApprover, 0),
					Type:                "user",
					Signers: map[string]struct{}{
						"tester": {},
					},
					SignatureType:    "pep",
					SigningRule:      "AnyOf",
					SignatureCarrier: "cloud",
					Signatures:       []FileSignaturePair{},
					SigningParamsPaths: script.SigningParamsPaths{
						INN:   "form_3.inn",
						SNILS: "form_3.snils",
						Files: []string{"form_3.files"},
					},
					SignLog:            []SignLogEntry{},
					FormsAccessibility: []script.FormAccessibility{{}},
					Reentered:          true,
					WorkType:           &workTypeVal,
					SLA:                &slaVal,
					Deadline:           time.Date(0001, 01, 01, 14, 00, 00, 00, time.UTC),
				},
				RunContext: &BlockRunContext{
					TaskID:      uuid.MustParse("00000000-0000-0000-0000-000000000000"),
					IsTest:      false,
					Delegations: human_tasks.Delegations(nil),
					VarStore: &store.VariableStore{
						Mutex: sync.Mutex{},
						State: map[string]json.RawMessage{
							"example": unmarshalFromTestFile(t, "testdata/signing_params/signing_state_signing_params.json"),
						},
						Values: map[string]interface{}{
							"form_3.snils": "snils_1",
							"form_3.inn":   "inn_1",
							"form_3.files": []interface{}{
								map[string]interface{}{"file_id": "uuid1"},
								map[string]interface{}{"file_id": "uuid2"},
							},
						},
						Steps:      []string{"example"},
						Errors:     []string{},
						StopPoints: store.StopPoints{},
					},
					CurrBlockStartTime: time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
					skipNotifications:  true,
					Services:           RunContextServices{},
					TaskSubscriptionData: TaskSubscriptionData{
						NotificationSchema: script.JSONSchema{},
					},
				},
				happenedEvents: []entity.NodeEvent{},
			},
		},
		{
			name: "acceptance test",
			args: args{
				name: example,
				runCtx: &BlockRunContext{
					skipNotifications: true,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						r, _ := json.Marshal(&SignData{
							Type: script.SignerTypeUser,
							Signers: map[string]struct{}{
								"tester": {},
							},
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})
						s.State = map[string]json.RawMessage{
							example: r,
						}

						return s
					}(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
					},
				},
				ef: &entity.EriusFunc{
					BlockType:  BlockGoSignID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
					Input: []entity.EriusFunctionValue{
						{
							Name:   "foo",
							Type:   "string",
							Global: "bar",
						},
					},
					Output: &script.JSONSchema{
						Type: "object",
						Properties: map[string]script.JSONSchemaPropertiesValue{
							"foo": {
								Type:   "string",
								Global: "bar",
							},
							"empty_global": {
								Type:   "string",
								Global: "",
							},
						},
					},
					Params: func() []byte {
						r, _ := json.Marshal(&script.SignParams{
							SignatureType:      script.SignatureTypeUKEP,
							SignatureCarrier:   script.SignatureCarrierCloud,
							Type:               script.SignerTypeUser,
							Signer:             "tester",
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})

						return r
					}(),
				},
			},
			want: &GoSignBlock{
				Name:      "example",
				ShortName: "Нода Подписания",
				Title:     "title",
				Input:     map[string]string{"foo": "bar"},
				Output:    map[string]string{"foo": "bar"},
				Sockets: []script.Socket{
					{
						ID:           "default",
						Title:        "Выход по умолчанию",
						NextBlockIds: []string{"next_0"},
						ActionType:   "",
					},
					{
						ID:           "rejected",
						Title:        "Отклонить",
						NextBlockIds: []string{"next_1"},
						ActionType:   "",
					},
				},
				State: &SignData{
					Attachments:         make([]entity.Attachment, 0),
					AdditionalApprovers: make([]AdditionalSignApprover, 0),
					Type:                "user",
					Signers: map[string]struct{}{
						"tester": {},
					},
					SignatureType:    "ukep",
					SigningRule:      "AnyOf",
					Signatures:       make([]FileSignaturePair, 0),
					SignatureCarrier: "cloud",
					SignLog:          make([]SignLogEntry, 0),
					FormsAccessibility: []script.FormAccessibility{{
						NodeID:      "",
						Name:        "",
						Description: "",
						AccessType:  "",
					}},
					Reentered: true,
					WorkType:  &workTypeVal,
					SLA:       &slaVal,
					Deadline:  time.Date(0001, 01, 01, 14, 00, 00, 00, time.UTC),
				},
				RunContext: &BlockRunContext{
					TaskID:      uuid.MustParse("00000000-0000-0000-0000-000000000000"),
					IsTest:      false,
					Delegations: human_tasks.Delegations(nil),
					VarStore: &store.VariableStore{
						Mutex: sync.Mutex{},
						State: map[string]json.RawMessage{
							"example": unmarshalFromTestFile(t, "testdata/signing_params/signing_state_signing_params.json"),
						},
						Values:     map[string]interface{}{},
						Steps:      []string{"example"},
						Errors:     []string{},
						StopPoints: store.StopPoints{},
					},
					CurrBlockStartTime: time.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
					skipNotifications:  true,
					Services:           RunContextServices{},
					TaskSubscriptionData: TaskSubscriptionData{
						NotificationSchema: script.JSONSchema{},
					},
				},
				happenedEvents: []entity.NodeEvent{},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := c.Background()

			got, _, _ := createGoSignBlock(ctx, test.args.name, test.args.ef, test.args.runCtx, nil)

			if test.want == nil {
				if got == nil {
					assert.Equal(t, true, true)
					return
				}
				assert.FailNow(t, "expected no State")
			}
			assert.Equal(t, test.want.State, got.State)
		})
	}
}

func TestGoSignBlock_Update(t *testing.T) {
	const (
		invalidLogin = "foobar"

		login  = "example"
		login2 = "example2"

		stepName = "sign"
	)

	logins := "example"

	workTypeVal := "8/5"
	slaVal := 8

	type (
		fields struct {
			Name             string
			Title            string
			Input            map[string]string
			Output           map[string]string
			NextStep         []script.Socket
			SignData         *SignData
			RunContext       *BlockRunContext
			SigningRule      script.SigningRule
			SignLog          []SignLogEntry
			SignatureType    script.SignatureType
			SignatureCarrier script.SignatureCarrier
		}
		args struct {
			ctx  c.Context
			data *script.BlockUpdateData
		}
	)

	tests := []struct {
		name             string
		fields           fields
		args             args
		wantErr          bool
		expectedDecision SignDecision
	}{
		{
			name: "empty data",
			fields: fields{
				Name: stepName,
				RunContext: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("CheckIsOnEditing",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								"00000000-0000-0000-0000-000000000000",
							).Return(
								false, nil,
							)

							return res
						}(),
					},
				},
			},
			args: args{
				ctx:  c.Background(),
				data: nil,
			},
			wantErr: true,
		},
		{
			name: "one signer ukep",
			fields: fields{
				Name:          stepName,
				SignatureType: script.SignatureTypeUKEP,
				SignData: &SignData{
					Type: script.SignerTypeUser,
					Signers: map[string]struct{}{
						invalidLogin: {},
					},
					ActualSigner: &logins,
					WorkType:     &workTypeVal,
					SLA:          &slaVal,
				},
				RunContext: &BlockRunContext{
					skipNotifications: false,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("CheckIsOnEditing",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								"00000000-0000-0000-0000-000000000000",
							).Return(
								false, nil,
							)

							return res
						}(),
						ServiceDesc: func() servicedesc.Service {
							httpClient := http.DefaultClient
							retryableHttpClient := httpclient.NewClient(httpClient, nil, 0, 0)

							mockTransport := serviceDeskMocks.RoundTripper{}
							fResponse := func(*http.Request) *http.Response {
								b, _ := json.Marshal(people.Person{})
								body := io.NopCloser(bytes.NewReader(b))

								defer body.Close()

								return &http.Response{
									Status:     http.StatusText(http.StatusOK),
									StatusCode: http.StatusOK,
									Body:       body,
								}
							}

							fError := func(*http.Request) error {
								return nil
							}

							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, fError)
							httpClient.Transport = &mockTransport

							sdMock, _ := sd_nocache.NewService(&servicedesc.Config{}, nil, nil)
							sdMock.SetCli(retryableHttpClient)

							return sdMock
						}(),
					},
				},
			},
			args: args{
				ctx: c.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    invalidLogin,
					Action:     string(entity.TaskUpdateActionSign),
					Parameters: []byte(`{"decision":"` + SignDecisionRejected + `"}`),
				},
			},
			wantErr: false,
		},
		{
			name: "one signer unep",
			fields: fields{
				Name:          stepName,
				SignatureType: script.SignatureTypeUNEP,
				SignData: &SignData{
					Type: script.SignerTypeUser,
					Signers: map[string]struct{}{
						invalidLogin: {},
					},
					ActualSigner: &logins,
					WorkType:     &workTypeVal,
					SLA:          &slaVal,
				},
				RunContext: &BlockRunContext{
					skipNotifications: false,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("CheckIsOnEditing",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								"00000000-0000-0000-0000-000000000000",
							).Return(
								false, nil,
							)

							return res
						}(),
						ServiceDesc: func() servicedesc.Service {
							httpClient := http.DefaultClient
							retryableHttpClient := httpclient.NewClient(httpClient, nil, 0, 0)

							mockTransport := serviceDeskMocks.RoundTripper{}
							fResponse := func(*http.Request) *http.Response {
								b, _ := json.Marshal(people.Person{})
								body := io.NopCloser(bytes.NewReader(b))

								defer body.Close()

								return &http.Response{
									Status:     http.StatusText(http.StatusOK),
									StatusCode: http.StatusOK,
									Body:       body,
								}
							}

							fError := func(*http.Request) error {
								return nil
							}

							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, fError)
							httpClient.Transport = &mockTransport

							sdMock, _ := sd_nocache.NewService(&servicedesc.Config{}, nil, nil)
							sdMock.SetCli(retryableHttpClient)

							return sdMock
						}(),
					},
				},
			},

			args: args{
				ctx: c.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    invalidLogin,
					Action:     string(entity.TaskUpdateActionSign),
					Parameters: []byte(`{"decision":"` + SignDecisionRejected + `"}`),
				},
			},
			wantErr: false,
		},
		{
			name: "one signer pep",
			fields: fields{
				Name:          stepName,
				SignatureType: script.SignatureTypePEP,
				SignData: &SignData{
					Type: script.SignerTypeUser,
					Signers: map[string]struct{}{
						invalidLogin: {},
					},
					ActualSigner: &logins,
					WorkType:     &workTypeVal,
					SLA:          &slaVal,
				},
				RunContext: &BlockRunContext{
					skipNotifications: false,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("CheckIsOnEditing",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								"00000000-0000-0000-0000-000000000000",
							).Return(
								false, nil,
							)

							return res
						}(),
						ServiceDesc: func() servicedesc.Service {
							httpClient := http.DefaultClient
							retryableHttpClient := httpclient.NewClient(httpClient, nil, 0, 0)
							mockTransport := serviceDeskMocks.RoundTripper{}
							fResponse := func(*http.Request) *http.Response {
								b, _ := json.Marshal(people.Person{})
								body := io.NopCloser(bytes.NewReader(b))

								defer body.Close()

								return &http.Response{
									Status:     http.StatusText(http.StatusOK),
									StatusCode: http.StatusOK,
									Body:       body,
								}
							}

							fError := func(*http.Request) error {
								return nil
							}

							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, fError)
							httpClient.Transport = &mockTransport
							sdMock, _ := sd_nocache.NewService(&servicedesc.Config{}, nil, nil)
							sdMock.SetCli(retryableHttpClient)

							return sdMock
						}(),
					},
				},
			},
			args: args{
				ctx: c.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    invalidLogin,
					Action:     string(entity.TaskUpdateActionSign),
					Parameters: []byte(`{"decision":"` + SignDecisionRejected + `"}`),
				},
			},
			wantErr: false,
		},
		{
			name: "one signer ukep and all carrier",
			fields: fields{
				Name:             stepName,
				SignatureType:    script.SignatureTypeUKEP,
				SignatureCarrier: script.SignatureCarrierAll,
				SignData: &SignData{
					Type: script.SignerTypeUser,
					Signers: map[string]struct{}{
						invalidLogin: {},
					},
					ActualSigner: &logins,
					WorkType:     &workTypeVal,
					SLA:          &slaVal,
				},
				RunContext: &BlockRunContext{
					skipNotifications: false,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("CheckIsOnEditing",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								"00000000-0000-0000-0000-000000000000",
							).Return(
								false, nil,
							)

							return res
						}(),
						ServiceDesc: func() servicedesc.Service {
							httpClient := http.DefaultClient
							retryableHttpClient := httpclient.NewClient(httpClient, nil, 0, 0)
							mockTransport := serviceDeskMocks.RoundTripper{}
							fResponse := func(*http.Request) *http.Response {
								b, _ := json.Marshal(people.Person{})
								body := io.NopCloser(bytes.NewReader(b))

								defer body.Close()

								return &http.Response{
									Status:     http.StatusText(http.StatusOK),
									StatusCode: http.StatusOK,
									Body:       body,
								}
							}

							fError := func(*http.Request) error {
								return nil
							}

							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, fError)
							httpClient.Transport = &mockTransport
							sdMock, _ := sd_nocache.NewService(&servicedesc.Config{}, nil, nil)
							sdMock.SetCli(retryableHttpClient)

							return sdMock
						}(),
					},
				},
			},
			args: args{
				ctx: c.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    invalidLogin,
					Action:     string(entity.TaskUpdateActionSign),
					Parameters: []byte(`{"decision":"` + SignDecisionRejected + `"}`),
				},
			},
			wantErr: false,
		},
		{
			name: "no decision",
			fields: fields{
				Name:          stepName,
				SignatureType: script.SignatureTypeUKEP,
				SigningRule:   script.AnyOfSigningRequired,
				SignData: &SignData{
					Type: script.SignerTypeUser,
					Signers: map[string]struct{}{
						invalidLogin: {},
					},
					ActualSigner: &logins,
					WorkType:     &workTypeVal,
					SLA:          &slaVal,
				},
				RunContext: &BlockRunContext{
					skipNotifications: false,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("CheckIsOnEditing",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								"00000000-0000-0000-0000-000000000000",
							).Return(
								false, nil,
							)

							return res
						}(),
						ServiceDesc: func() servicedesc.Service {
							httpClient := http.DefaultClient
							retryableHttpClient := httpclient.NewClient(httpClient, nil, 0, 0)
							mockTransport := serviceDeskMocks.RoundTripper{}
							fResponse := func(*http.Request) *http.Response {
								b, _ := json.Marshal(people.Person{})
								body := io.NopCloser(bytes.NewReader(b))

								defer body.Close()

								return &http.Response{
									Status:     http.StatusText(http.StatusOK),
									StatusCode: http.StatusOK,
									Body:       body,
								}
							}

							fError := func(*http.Request) error {
								return nil
							}
							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, fError)
							httpClient.Transport = &mockTransport
							sdMock, _ := sd_nocache.NewService(&servicedesc.Config{}, nil, nil)
							sdMock.SetCli(retryableHttpClient)

							return sdMock
						}(),
					},
				},
			},
			args: args{
				ctx: c.Background(),
				data: &script.BlockUpdateData{
					ByLogin: invalidLogin,
					Action:  string(entity.TaskUpdateActionSign),
				},
			},
			wantErr: true,
		},
		{
			name: "signed not valid login UKEP",
			fields: fields{
				Name: stepName,
				SignData: &SignData{
					SignatureType: script.SignatureTypeUKEP,
					Type:          script.SignerTypeUser,
					Signers: map[string]struct{}{
						login2: {},
					},
					ActualSigner: &logins,
					WorkType:     &workTypeVal,
					SLA:          &slaVal,
				},
				RunContext: &BlockRunContext{
					skipNotifications: false,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("CheckIsOnEditing",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								"00000000-0000-0000-0000-000000000000",
							).Return(
								false, nil,
							)

							return res
						}(),
						ServiceDesc: func() servicedesc.Service {
							httpClient := http.DefaultClient
							retryableHttpClient := httpclient.NewClient(httpClient, nil, 0, 0)
							mockTransport := serviceDeskMocks.RoundTripper{}
							fResponse := func(*http.Request) *http.Response {
								b, _ := json.Marshal(people.Person{})
								body := io.NopCloser(bytes.NewReader(b))

								defer body.Close()

								return &http.Response{
									Status:     http.StatusText(http.StatusOK),
									StatusCode: http.StatusOK,
									Body:       body,
								}
							}

							fError := func(*http.Request) error {
								return nil
							}

							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, fError)
							httpClient.Transport = &mockTransport
							sdMock, _ := sd_nocache.NewService(&servicedesc.Config{}, nil, nil)
							sdMock.SetCli(retryableHttpClient)

							return sdMock
						}(),
					},
				},
			},
			args: args{
				ctx: c.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    login2,
					Action:     string(entity.TaskUpdateActionSign),
					Parameters: []byte(`{"decision":"` + SignDecisionSigned + `"}`),
				},
			},
			wantErr: true,
		},
		{
			name: "signed with not valid login UNEP",
			fields: fields{
				Name: stepName,
				SignData: &SignData{
					SignatureType: script.SignatureTypeUNEP,
					Type:          script.SignerTypeUser,
					Signers: map[string]struct{}{
						login2: {},
					},
					ActualSigner: &logins,
					WorkType:     &workTypeVal,
					SLA:          &slaVal,
				},
				RunContext: &BlockRunContext{
					skipNotifications: false,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("CheckIsOnEditing",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								"00000000-0000-0000-0000-000000000000",
							).Return(
								false, nil,
							)

							return res
						}(),
						ServiceDesc: func() servicedesc.Service {
							httpClient := http.DefaultClient
							retryableHttpClient := httpclient.NewClient(httpClient, nil, 0, 0)
							mockTransport := serviceDeskMocks.RoundTripper{}
							fResponse := func(*http.Request) *http.Response {
								b, _ := json.Marshal(people.Person{})
								body := io.NopCloser(bytes.NewReader(b))

								defer body.Close()

								return &http.Response{
									Status:     http.StatusText(http.StatusOK),
									StatusCode: http.StatusOK,
									Body:       body,
								}
							}

							fError := func(*http.Request) error {
								return nil
							}
							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, fError)
							httpClient.Transport = &mockTransport
							sdMock, _ := sd_nocache.NewService(&servicedesc.Config{}, nil, nil)
							sdMock.SetCli(retryableHttpClient)

							return sdMock
						}(),
					},
				},
			},
			args: args{
				ctx: c.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    login2,
					Action:     string(entity.TaskUpdateActionSign),
					Parameters: []byte(`{"decision":"` + SignDecisionSigned + `"}`),
				},
			},
			wantErr: false,
		},
		{
			name: "signed with not valid login PEP",
			fields: fields{
				Name: stepName,
				SignData: &SignData{
					SignatureType: script.SignatureTypePEP,
					Type:          script.SignerTypeUser,
					Signers: map[string]struct{}{
						login2: {},
					},
					ActualSigner: &logins,
					WorkType:     &workTypeVal,
					SLA:          &slaVal,
				},
				RunContext: &BlockRunContext{
					skipNotifications: false,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("CheckIsOnEditing",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								"00000000-0000-0000-0000-000000000000",
							).Return(
								false, nil,
							)

							return res
						}(),
						ServiceDesc: func() servicedesc.Service {
							httpClient := http.DefaultClient
							retryableHttpClient := httpclient.NewClient(httpClient, nil, 0, 0)
							mockTransport := serviceDeskMocks.RoundTripper{}
							fResponse := func(*http.Request) *http.Response {
								b, _ := json.Marshal(people.Person{})
								body := io.NopCloser(bytes.NewReader(b))

								defer body.Close()

								return &http.Response{
									Status:     http.StatusText(http.StatusOK),
									StatusCode: http.StatusOK,
									Body:       body,
								}
							}

							fError := func(*http.Request) error {
								return nil
							}
							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, fError)
							httpClient.Transport = &mockTransport
							sdMock, _ := sd_nocache.NewService(&servicedesc.Config{}, nil, nil)
							sdMock.SetCli(retryableHttpClient)

							return sdMock
						}(),
					},
				},
			},
			args: args{
				ctx: c.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    login2,
					Action:     string(entity.TaskUpdateActionSign),
					Parameters: []byte(`{"decision":"` + SignDecisionSigned + `"}`),
				},
			},
			wantErr: false,
		},
		{
			name: "Sign decision signed UKEP",
			fields: fields{
				Name: stepName,
				SignData: &SignData{
					SignatureType: script.SignatureTypeUKEP,
					Type:          script.SignerTypeUser,
					Signers: map[string]struct{}{
						invalidLogin: {},
					},
					ActualSigner: &logins,
					WorkType:     &workTypeVal,
					SLA:          &slaVal,
				},
				RunContext: &BlockRunContext{
					skipNotifications: false,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("CheckIsOnEditing",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								"00000000-0000-0000-0000-000000000000",
							).Return(
								false, nil,
							)

							return res
						}(),
						ServiceDesc: func() servicedesc.Service {
							httpClient := http.DefaultClient
							retryableHttpClient := httpclient.NewClient(httpClient, nil, 0, 0)
							mockTransport := serviceDeskMocks.RoundTripper{}
							fResponse := func(*http.Request) *http.Response {
								b, _ := json.Marshal(people.Person{})
								body := io.NopCloser(bytes.NewReader(b))

								defer body.Close()

								return &http.Response{
									Status:     http.StatusText(http.StatusOK),
									StatusCode: http.StatusOK,
									Body:       body,
								}
							}

							fError := func(*http.Request) error {
								return nil
							}
							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, fError)
							httpClient.Transport = &mockTransport
							sdMock, _ := sd_nocache.NewService(&servicedesc.Config{}, nil, nil)
							sdMock.SetCli(retryableHttpClient)

							return sdMock
						}(),
					},
				},
			},
			args: args{
				ctx: c.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    invalidLogin,
					Action:     string(entity.TaskUpdateActionSign),
					Parameters: []byte(`{"decision":"` + SignDecisionSigned + `"}`),
				},
			},
			wantErr: true,
		},
		{
			name: "Sign decision signed UNEP",
			fields: fields{
				Name: stepName,
				SignData: &SignData{
					SignatureType: script.SignatureTypeUNEP,
					Type:          script.SignerTypeUser,
					Signers: map[string]struct{}{
						invalidLogin: {},
					},
					ActualSigner: &logins,
					WorkType:     &workTypeVal,
					SLA:          &slaVal,
				},
				RunContext: &BlockRunContext{
					skipNotifications: false,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("CheckIsOnEditing",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								"00000000-0000-0000-0000-000000000000",
							).Return(
								false, nil,
							)

							return res
						}(),
						ServiceDesc: func() servicedesc.Service {
							httpClient := http.DefaultClient
							retryableHttpClient := httpclient.NewClient(httpClient, nil, 0, 0)
							mockTransport := serviceDeskMocks.RoundTripper{}
							fResponse := func(*http.Request) *http.Response {
								b, _ := json.Marshal(people.Person{})
								body := io.NopCloser(bytes.NewReader(b))

								defer body.Close()

								return &http.Response{
									Status:     http.StatusText(http.StatusOK),
									StatusCode: http.StatusOK,
									Body:       body,
								}
							}

							fError := func(*http.Request) error {
								return nil
							}
							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, fError)
							httpClient.Transport = &mockTransport
							sdMock, _ := sd_nocache.NewService(&servicedesc.Config{}, nil, nil)
							sdMock.SetCli(retryableHttpClient)

							return sdMock
						}(),
					},
				},
			},
			args: args{
				ctx: c.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    invalidLogin,
					Action:     string(entity.TaskUpdateActionSign),
					Parameters: []byte(`{"decision":"` + SignDecisionSigned + `"}`),
				},
			},
			wantErr: false,
		},
		{
			name: "Sign decision signed PEP",
			fields: fields{
				Name: stepName,
				SignData: &SignData{
					Type:          script.SignerTypeUser,
					SignatureType: script.SignatureTypePEP,
					Signers: map[string]struct{}{
						invalidLogin: {},
					},
					ActualSigner: &logins,
					WorkType:     &workTypeVal,
					SLA:          &slaVal,
				},
				RunContext: &BlockRunContext{
					skipNotifications: false,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("CheckIsOnEditing",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								"00000000-0000-0000-0000-000000000000",
							).Return(
								false, nil,
							)

							return res
						}(),
						ServiceDesc: func() servicedesc.Service {
							httpClient := http.DefaultClient
							retryableHttpClient := httpclient.NewClient(httpClient, nil, 0, 0)
							mockTransport := serviceDeskMocks.RoundTripper{}
							fResponse := func(*http.Request) *http.Response {
								b, _ := json.Marshal(people.Person{})
								body := io.NopCloser(bytes.NewReader(b))

								defer body.Close()

								return &http.Response{
									Status:     http.StatusText(http.StatusOK),
									StatusCode: http.StatusOK,
									Body:       body,
								}
							}

							fError := func(*http.Request) error {
								return nil
							}
							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, fError)
							httpClient.Transport = &mockTransport
							sdMock, _ := sd_nocache.NewService(&servicedesc.Config{}, nil, nil)
							sdMock.SetCli(retryableHttpClient)

							return sdMock
						}(),
					},
				},
			},
			args: args{
				ctx: c.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    invalidLogin,
					Action:     string(entity.TaskUpdateActionSign),
					Parameters: []byte(`{"decision":"` + SignDecisionSigned + `"}`),
				},
			},
			wantErr: false,
		},
		{
			name: "Sign decision rejected UKEP",
			fields: fields{
				Name: stepName,
				SignData: &SignData{
					Type:          script.SignerTypeUser,
					SignatureType: script.SignatureTypeUKEP,
					Signers: map[string]struct{}{
						ServiceAccount: {},
					},
					ActualSigner: &logins,
					WorkType:     &workTypeVal,
					SLA:          &slaVal,
				},
				RunContext: &BlockRunContext{
					skipNotifications: false,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("CheckIsOnEditing",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								"00000000-0000-0000-0000-000000000000",
							).Return(
								false, nil,
							)

							return res
						}(),
						ServiceDesc: func() servicedesc.Service {
							httpClient := http.DefaultClient
							retryableHttpClient := httpclient.NewClient(httpClient, nil, 0, 0)
							mockTransport := serviceDeskMocks.RoundTripper{}
							fResponse := func(*http.Request) *http.Response {
								b, _ := json.Marshal(people.Person{})
								body := io.NopCloser(bytes.NewReader(b))

								defer body.Close()

								return &http.Response{
									Status:     http.StatusText(http.StatusOK),
									StatusCode: http.StatusOK,
									Body:       body,
								}
							}

							fError := func(*http.Request) error {
								return nil
							}

							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, fError)
							httpClient.Transport = &mockTransport
							sdMock, _ := sd_nocache.NewService(&servicedesc.Config{}, nil, nil)
							sdMock.SetCli(retryableHttpClient)

							return sdMock
						}(),
					},
				},
			},
			args: args{
				ctx: c.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    ServiceAccount,
					Action:     string(entity.TaskUpdateActionSign),
					Parameters: []byte(`{"decision":"` + SignDecisionRejected + `"}`),
				},
			},
			wantErr: false,
		},
		{
			name: "Sign decision rejected UNEP",
			fields: fields{
				Name: stepName,
				SignData: &SignData{
					Type:          script.SignerTypeUser,
					SignatureType: script.SignatureTypeUNEP,
					Signers: map[string]struct{}{
						ServiceAccount: {},
					},
					ActualSigner: &logins,
					WorkType:     &workTypeVal,
					SLA:          &slaVal,
				},
				RunContext: &BlockRunContext{
					skipNotifications: false,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("CheckIsOnEditing",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								"00000000-0000-0000-0000-000000000000",
							).Return(
								false, nil,
							)

							return res
						}(),
						ServiceDesc: func() servicedesc.Service {
							httpClient := http.DefaultClient
							retryableHttpClient := httpclient.NewClient(httpClient, nil, 0, 0)
							mockTransport := serviceDeskMocks.RoundTripper{}
							fResponse := func(*http.Request) *http.Response {
								b, _ := json.Marshal(people.Person{})
								body := io.NopCloser(bytes.NewReader(b))

								defer body.Close()

								return &http.Response{
									Status:     http.StatusText(http.StatusOK),
									StatusCode: http.StatusOK,
									Body:       body,
								}
							}

							fError := func(*http.Request) error {
								return nil
							}

							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, fError)
							httpClient.Transport = &mockTransport
							sdMock, _ := sd_nocache.NewService(&servicedesc.Config{}, nil, nil)
							sdMock.SetCli(retryableHttpClient)

							return sdMock
						}(),
					},
				},
			},
			args: args{
				ctx: c.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    ServiceAccount,
					Action:     string(entity.TaskUpdateActionSign),
					Parameters: []byte(`{"decision":"` + SignDecisionRejected + `"}`),
				},
			},
			wantErr: false,
		},
		{
			name: "Sign decision rejected PEP",
			fields: fields{
				Name: stepName,
				SignData: &SignData{
					Type:          script.SignerTypeUser,
					SignatureType: script.SignatureTypePEP,
					Signers: map[string]struct{}{
						ServiceAccount: {},
					},
					ActualSigner: &logins,
					WorkType:     &workTypeVal,
					SLA:          &slaVal,
				},
				RunContext: &BlockRunContext{
					skipNotifications: false,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("CheckIsOnEditing",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								"00000000-0000-0000-0000-000000000000",
							).Return(
								false, nil,
							)

							return res
						}(),
						ServiceDesc: func() servicedesc.Service {
							httpClient := http.DefaultClient
							retryableHttpClient := httpclient.NewClient(httpClient, nil, 0, 0)
							mockTransport := serviceDeskMocks.RoundTripper{}
							fResponse := func(*http.Request) *http.Response {
								b, _ := json.Marshal(people.Person{})
								body := io.NopCloser(bytes.NewReader(b))

								defer body.Close()

								return &http.Response{
									Status:     http.StatusText(http.StatusOK),
									StatusCode: http.StatusOK,
									Body:       body,
								}
							}

							fError := func(*http.Request) error {
								return nil
							}

							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, fError)
							httpClient.Transport = &mockTransport
							sdMock, _ := sd_nocache.NewService(&servicedesc.Config{}, nil, nil)
							sdMock.SetCli(retryableHttpClient)

							return sdMock
						}(),
					},
				},
			},
			args: args{
				ctx: c.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    ServiceAccount,
					Action:     string(entity.TaskUpdateActionSign),
					Parameters: []byte(`{"decision":"` + SignDecisionRejected + `"}`),
				},
			},
			wantErr: false,
		},
		{
			name: "Sign decision error ukep",
			fields: fields{
				Name: stepName,
				SignData: &SignData{
					SignatureType: script.SignatureTypeUKEP,
					SigningRule:   script.AllOfSigningRequired,
					Type:          script.SignerTypeUser,
					Signers: map[string]struct{}{
						login:  {},
						login2: {},
					},
					ActualSigner: &logins,
					WorkType:     &workTypeVal,
					SLA:          &slaVal,
				},
				RunContext: &BlockRunContext{
					skipNotifications: false,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("CheckIsOnEditing",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								"00000000-0000-0000-0000-000000000000",
							).Return(
								false, nil,
							)

							return res
						}(),
						ServiceDesc: func() servicedesc.Service {
							httpClient := http.DefaultClient
							retryableHttpClient := httpclient.NewClient(httpClient, nil, 0, 0)
							mockTransport := serviceDeskMocks.RoundTripper{}
							fResponse := func(*http.Request) *http.Response {
								b, _ := json.Marshal(people.Person{})
								body := io.NopCloser(bytes.NewReader(b))

								defer body.Close()

								return &http.Response{
									Status:     http.StatusText(http.StatusOK),
									StatusCode: http.StatusOK,
									Body:       body,
								}
							}

							fError := func(*http.Request) error {
								return nil
							}

							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, fError)
							httpClient.Transport = &mockTransport

							sdMock, _ := sd_nocache.NewService(&servicedesc.Config{}, nil, nil)
							sdMock.SetCli(retryableHttpClient)

							return sdMock
						}(),
					},
				},
			},
			args: args{
				ctx: c.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    login,
					Action:     string(entity.TaskUpdateActionSign),
					Parameters: []byte(`{"decision":"` + SignDecisionError + `"}`),
				},
			},
			wantErr: true,
		},
		{
			name: "Nil executors",
			fields: fields{
				Name: stepName,
				SignData: &SignData{
					IsTakenInWork: true,
					Type:          script.SignerTypeUser,
					Signers:       map[string]struct{}{},
					WorkType:      &workTypeVal,
					SLA:           &slaVal,
				},
				RunContext: &BlockRunContext{
					skipNotifications: false,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("CheckIsOnEditing",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								"00000000-0000-0000-0000-000000000000",
							).Return(
								false, nil,
							)

							return res
						}(),
					},
				},
			},

			args: args{
				ctx: c.Background(),
				data: &script.BlockUpdateData{
					Action:     string(entity.TaskUpdateActionSign),
					Parameters: []byte(`{"decision":"` + SignDecisionSigned + `"}`),
				},
			},
			wantErr: true,
		},
		{
			name: "acceptance test",
			fields: fields{
				Name: stepName,
				SignData: &SignData{
					IsTakenInWork: true,
					Type:          script.SignerTypeUser,
					Signers: map[string]struct{}{
						invalidLogin: {},
					},
					ActualSigner: &logins,
					WorkType:     &workTypeVal,
					SLA:          &slaVal,
				},
				RunContext: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("CheckIsOnEditing",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								"00000000-0000-0000-0000-000000000000",
							).Return(
								false, nil,
							)

							return res
						}(),
						ServiceDesc: func() servicedesc.Service {
							httpClient := http.DefaultClient
							retryableHttpClient := httpclient.NewClient(httpClient, nil, 0, 0)
							mockTransport := serviceDeskMocks.RoundTripper{}
							fResponse := func(*http.Request) *http.Response {
								b, _ := json.Marshal(people.Person{})
								body := io.NopCloser(bytes.NewReader(b))

								defer body.Close()

								return &http.Response{
									Status:     http.StatusText(http.StatusOK),
									StatusCode: http.StatusOK,
									Body:       body,
								}
							}

							fError := func(*http.Request) error {
								return nil
							}

							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, fError)
							httpClient.Transport = &mockTransport

							sdMock, _ := sd_nocache.NewService(&servicedesc.Config{}, nil, nil)
							sdMock.SetCli(retryableHttpClient)

							return sdMock
						}(),
					},
				},
			},
			args: args{
				ctx: c.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    invalidLogin,
					Action:     string(entity.TaskUpdateActionSign),
					Parameters: []byte(`{"decision":"` + SignDecisionSigned + `"}`),
				},
			},
			wantErr: false,
		},
		{
			name: "one signer ukep where attachments and signatures are not equal",
			fields: fields{
				Name:          stepName,
				SignatureType: script.SignatureTypeUKEP,
				SignData: &SignData{
					Type:        script.SignerTypeUser,
					Attachments: []entity.Attachment{{FileID: "some_file_id"}},
					Signers: map[string]struct{}{
						invalidLogin: {},
					},
					ActualSigner: &logins,
					WorkType:     &workTypeVal,
					SLA:          &slaVal,
				},
				RunContext: &BlockRunContext{
					skipNotifications: false,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("CheckIsOnEditing",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								"00000000-0000-0000-0000-000000000000",
							).Return(
								false, nil,
							)

							return res
						}(),
						ServiceDesc: func() servicedesc.Service {
							httpClient := http.DefaultClient
							retryableHttpClient := httpclient.NewClient(httpClient, nil, 0, 0)
							mockTransport := serviceDeskMocks.RoundTripper{}
							fResponse := func(*http.Request) *http.Response {
								b, _ := json.Marshal(people.Person{})
								body := io.NopCloser(bytes.NewReader(b))

								defer body.Close()

								return &http.Response{
									Status:     http.StatusText(http.StatusOK),
									StatusCode: http.StatusOK,
									Body:       body,
								}
							}
							fError := func(*http.Request) error {
								return nil
							}
							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, fError)
							httpClient.Transport = &mockTransport

							sdMock, _ := sd_nocache.NewService(&servicedesc.Config{}, nil, nil)
							sdMock.SetCli(retryableHttpClient)

							return sdMock
						}(),
					},
				},
			},
			args: args{
				ctx: c.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    invalidLogin,
					Action:     string(entity.TaskUpdateActionSign),
					Parameters: []byte(`{"decision":"` + SignDecisionRejected + `",'attachments':[{"file_id":"some_file_id"}]}`),
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gb := &GoSignBlock{
				Name:       tt.fields.Name,
				Title:      tt.fields.Title,
				Input:      tt.fields.Input,
				Output:     tt.fields.Output,
				Sockets:    tt.fields.NextStep,
				State:      tt.fields.SignData,
				RunContext: tt.fields.RunContext,
			}
			tt.fields.RunContext.UpdateData = tt.args.data
			_, err := gb.Update(tt.args.ctx)
			assert.Equalf(t, tt.wantErr, err != nil, fmt.Sprintf("Update(%v, %v)", tt.args.ctx, tt.args.data))
		})
	}
}

func TestGoSignBlock_CreateState(t *testing.T) {
	const (
		example    = "example"
		title      = "title"
		shortTitle = "Нода Подписания"

		stepName = "sign"
	)

	workTypeVal := "8/5"
	slaVal := 8

	varStore := store.NewStore()

	varStore.SetValue("form_0.user", map[string]interface{}{
		"username": "test",
		"fullname": "test test test",
	})
	varStore.SetValue("form_1.user", map[string]interface{}{
		"username": "test2",
		"fullname": "test2 test test",
	})

	next := []entity.Socket{
		{
			ID:           DefaultSocketID,
			Title:        script.DefaultSocketTitle,
			NextBlockIds: []string{"next_0"},
		},
		{
			ID:           rejectedSocketID,
			Title:        script.RejectSocketTitle,
			NextBlockIds: []string{"next_1"},
		},
	}

	type (
		fields struct {
			Name             string
			Title            string
			Input            map[string]string
			Output           map[string]string
			NextStep         []script.Socket
			RunContext       *BlockRunContext
			SigningRule      script.SigningRule
			SignLog          []SignLogEntry
			SignatureType    script.SignatureType
			SignatureCarrier script.SignatureCarrier
		}
		args struct {
			name string
			ef   *entity.EriusFunc
			ctx  c.Context
		}
	)

	tests := []struct {
		name    string
		args    args
		fields  fields
		wantErr bool
	}{
		{
			name: "no execution params",
			fields: fields{
				Name: stepName,
				RunContext: &BlockRunContext{
					WorkNumber:        "J001",
					skipNotifications: false,
					VarStore:          varStore,
				},
			},
			args: args{
				name: example,
				ctx:  c.Background(),
				ef: &entity.EriusFunc{
					BlockType:  BlockGoSignID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
				},
			},
			wantErr: true,
		},
		{
			name: "SignatureTypeUNEP with SignatureCarrierToken and SignerTypeFromSchema",
			fields: fields{
				Name: stepName,
				RunContext: &BlockRunContext{
					skipNotifications: true,
					WorkNumber:        "J001",
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						People: func() people.Service {
							res := new(peopleMock.Service)

							res.On("GetUser", "ravshan", false).
								Return(people.SSOUser{}, nil)

							return res
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("GetApplicationData", "J001").Return("", nil)
							res.On("GetAdditionalForms", "J001", "sign").Return([]string{}, nil)
							res.On("UpdateStepContext",
								mock.MatchedBy(func(ctx c.Context) bool { return true }),
								mock.AnythingOfType("*db.UpdateStepRequest"),
							).Return(
								nil,
							)

							return res
						}(),
					},
				},
			},

			args: args{
				ctx: c.Background(),
				ef: &entity.EriusFunc{
					BlockType:  BlockGoSignID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
					Params: func() []byte {
						r, _ := json.Marshal(&script.SignParams{
							SignatureType:      script.SignatureTypeUNEP,
							SignatureCarrier:   script.SignatureCarrierToken,
							Type:               script.SignerTypeFromSchema,
							Signer:             "tester",
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})

						return r
					}(),
				},
			},
			wantErr: true,
		},
		{
			name: "SignatureTypePEP with SignatureCarrierToken and SignerTypeGroup",
			fields: fields{
				Name: stepName,
				RunContext: &BlockRunContext{
					skipNotifications: true,
					WorkNumber:        "J001",
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						People: func() people.Service {
							res := new(peopleMock.Service)

							res.On("GetUser", "ravshan", false).
								Return(people.SSOUser{}, nil)

							return res
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("GetApplicationData", "J001").Return("", nil)
							res.On("GetAdditionalForms", "J001", "sign").Return([]string{}, nil)
							res.On("UpdateStepContext",
								mock.MatchedBy(func(ctx c.Context) bool { return true }),
								mock.AnythingOfType("*db.UpdateStepRequest"),
							).Return(
								nil,
							)

							return res
						}(),
					},
				},
			},

			args: args{
				ctx: c.Background(),
				ef: &entity.EriusFunc{
					BlockType:  BlockGoSignID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
					Params: func() []byte {
						r, _ := json.Marshal(&script.SignParams{
							SignatureType:      script.SignatureTypePEP,
							SignatureCarrier:   script.SignatureCarrierToken,
							Type:               script.SignerTypeGroup,
							Signer:             "tester",
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})

						return r
					}(),
				},
			},
			wantErr: true,
		},
		{
			name: "SignatureTypeUKEP and SignatureCarrierToken and SignerTypeFromSchema",
			fields: fields{
				Name: stepName,
				RunContext: &BlockRunContext{
					skipNotifications: true,
					WorkNumber:        "J001",
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						People: func() people.Service {
							res := new(peopleMock.Service)

							res.On("GetUser", "ravshan", false).
								Return(people.SSOUser{}, nil)

							return res
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("GetApplicationData", "J001").Return("", nil)
							res.On("GetAdditionalForms", "J001", "sign").Return([]string{}, nil)
							res.On("UpdateStepContext",
								mock.MatchedBy(func(ctx c.Context) bool { return true }),
								mock.AnythingOfType("*db.UpdateStepRequest"),
							).Return(
								nil,
							)

							return res
						}(),
					},
				},
			},

			args: args{
				ctx: c.Background(),
				ef: &entity.EriusFunc{
					BlockType:  BlockGoSignID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
					Params: func() []byte {
						r, _ := json.Marshal(&script.SignParams{
							SignatureType:      script.SignatureTypeUKEP,
							SignatureCarrier:   script.SignatureCarrierToken,
							Type:               script.SignerTypeFromSchema,
							Signer:             "tester",
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})

						return r
					}(),
				},
			},
			wantErr: true,
		},
		{
			name: "SignatureTypeUKEP and SignatureCarrierToken and SignerTypeGroup",
			fields: fields{
				Name: stepName,
				RunContext: &BlockRunContext{
					skipNotifications: true,
					WorkNumber:        "J001",
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						People: func() people.Service {
							res := new(peopleMock.Service)

							res.On("GetUser", "ravshan", false).
								Return(people.SSOUser{}, nil)

							return res
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("GetApplicationData", "J001").Return("", nil)
							res.On("GetAdditionalForms", "J001", "sign").Return([]string{}, nil)
							res.On("UpdateStepContext",
								mock.MatchedBy(func(ctx c.Context) bool { return true }),
								mock.AnythingOfType("*db.UpdateStepRequest"),
							).Return(
								nil,
							)

							return res
						}(),
					},
				},
			},

			args: args{
				ctx: c.Background(),
				ef: &entity.EriusFunc{
					BlockType:  BlockGoSignID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
					Params: func() []byte {
						r, _ := json.Marshal(&script.SignParams{
							SignatureType:      script.SignatureTypeUKEP,
							SignatureCarrier:   script.SignatureCarrierToken,
							Type:               script.SignerTypeGroup,
							Signer:             "tester",
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})

						return r
					}(),
				},
			},
			wantErr: true,
		},
		{
			name: "SignatureTypeUNEP with SignatureCarrierCloud and SignerTypeFromSchema",
			fields: fields{
				Name: stepName,
				RunContext: &BlockRunContext{
					skipNotifications: true,
					WorkNumber:        "J001",
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						People: func() people.Service {
							res := new(peopleMock.Service)

							res.On("GetUser", "ravshan", false).
								Return(people.SSOUser{}, nil)

							return res
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("GetApplicationData", "J001").Return("", nil)
							res.On("GetAdditionalForms", "J001", "sign").Return([]string{}, nil)
							res.On("UpdateStepContext",
								mock.MatchedBy(func(ctx c.Context) bool { return true }),
								mock.AnythingOfType("*db.UpdateStepRequest"),
							).Return(
								nil,
							)

							return res
						}(),
					},
				},
			},

			args: args{
				ctx: c.Background(),
				ef: &entity.EriusFunc{
					BlockType:  BlockGoSignID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
					Params: func() []byte {
						r, _ := json.Marshal(&script.SignParams{
							SignatureType:      script.SignatureTypeUNEP,
							SignatureCarrier:   script.SignatureCarrierCloud,
							Type:               script.SignerTypeFromSchema,
							Signer:             "tester",
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})

						return r
					}(),
				},
			},
			wantErr: true,
		},
		{
			name: "SignatureTypeUNEP with SignatureCarrierCloud and SignerTypeGroup",
			fields: fields{
				Name: stepName,
				RunContext: &BlockRunContext{
					skipNotifications: true,
					WorkNumber:        "J001",
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						People: func() people.Service {
							res := new(peopleMock.Service)

							res.On("GetUser", "ravshan", false).
								Return(people.SSOUser{}, nil)

							return res
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("CheckIsOnEditing", "J001").Return(false, nil)
							res.On("GetApplicationData", "J001").Return("", nil)
							res.On("GetAdditionalForms", "J001", "sign").Return([]string{}, nil)
							res.On("UpdateStepContext",
								mock.MatchedBy(func(ctx c.Context) bool { return true }),
								mock.AnythingOfType("*db.UpdateStepRequest"),
							).Return(
								nil,
							)

							return res
						}(),
					},
				},
			},

			args: args{
				ctx: c.Background(),
				ef: &entity.EriusFunc{
					BlockType:  BlockGoSignID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
					Params: func() []byte {
						r, _ := json.Marshal(&script.SignParams{
							SignatureType:      script.SignatureTypeUNEP,
							SignatureCarrier:   script.SignatureCarrierCloud,
							Type:               script.SignerTypeGroup,
							Signer:             "tester",
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})

						return r
					}(),
				},
			},
			wantErr: true,
		},
		{
			name: "SignatureTypePEP with SignatureCarrierCloud and SignerTypeFromSchema",
			fields: fields{
				Name: stepName,
				RunContext: &BlockRunContext{
					skipNotifications: true,
					WorkNumber:        "J001",
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						People: func() people.Service {
							res := new(peopleMock.Service)

							res.On("GetUser", "ravshan", false).
								Return(people.SSOUser{}, nil)

							return res
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("GetApplicationData", "J001").Return("", nil)
							res.On("GetAdditionalForms", "J001", "sign").Return([]string{}, nil)
							res.On("UpdateStepContext",
								mock.MatchedBy(func(ctx c.Context) bool { return true }),
								mock.AnythingOfType("*db.UpdateStepRequest"),
							).Return(
								nil,
							)

							return res
						}(),
					},
				},
			},

			args: args{
				ctx: c.Background(),
				ef: &entity.EriusFunc{
					BlockType:  BlockGoSignID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
					Params: func() []byte {
						r, _ := json.Marshal(&script.SignParams{
							SignatureType:      script.SignatureTypePEP,
							SignatureCarrier:   script.SignatureCarrierCloud,
							Type:               script.SignerTypeFromSchema,
							Signer:             "tester",
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})

						return r
					}(),
				},
			},
			wantErr: true,
		},
		{
			name: "SignatureTypePEP with SignatureCarrierCloud and SignerTypeGroup",
			fields: fields{
				Name: stepName,
				RunContext: &BlockRunContext{
					skipNotifications: true,
					WorkNumber:        "J001",
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						People: func() people.Service {
							res := new(peopleMock.Service)

							res.On("GetUser", "ravshan", false).
								Return(people.SSOUser{}, nil)

							return res
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("GetApplicationData", "J001").Return("", nil)
							res.On("GetAdditionalForms", "J001", "sign").Return([]string{}, nil)
							res.On("UpdateStepContext",
								mock.MatchedBy(func(ctx c.Context) bool { return true }),
								mock.AnythingOfType("*db.UpdateStepRequest"),
							).Return(
								nil,
							)

							return res
						}(),
					},
				},
			},

			args: args{
				ctx: c.Background(),
				ef: &entity.EriusFunc{
					BlockType:  BlockGoSignID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
					Params: func() []byte {
						r, _ := json.Marshal(&script.SignParams{
							SignatureType:      script.SignatureTypePEP,
							SignatureCarrier:   script.SignatureCarrierCloud,
							Type:               script.SignerTypeGroup,
							Signer:             "tester",
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})

						return r
					}(),
				},
			},
			wantErr: true,
		},
		{
			name: "SignatureTypeUKEP and SignatureCarrierCloud and SignerTypeFromSchema",
			fields: fields{
				Name: stepName,
				RunContext: &BlockRunContext{
					skipNotifications: true,
					WorkNumber:        "J001",
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						People: func() people.Service {
							res := new(peopleMock.Service)

							res.On("GetUser", "ravshan", false).
								Return(people.SSOUser{}, nil)

							return res
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("GetApplicationData", "J001").Return("", nil)
							res.On("GetAdditionalForms", "J001", "sign").Return([]string{}, nil)
							res.On("UpdateStepContext",
								mock.MatchedBy(func(ctx c.Context) bool { return true }),
								mock.AnythingOfType("*db.UpdateStepRequest"),
							).Return(
								nil,
							)

							return res
						}(),
					},
				},
			},

			args: args{
				ctx: c.Background(),
				ef: &entity.EriusFunc{
					BlockType:  BlockGoSignID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
					Params: func() []byte {
						r, _ := json.Marshal(&script.SignParams{
							SignatureType:      script.SignatureTypeUKEP,
							SignatureCarrier:   script.SignatureCarrierCloud,
							Type:               script.SignerTypeFromSchema,
							Signer:             "tester",
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})

						return r
					}(),
				},
			},
			wantErr: true,
		},
		{
			name: "SignatureTypeUKEP and SignatureCarrierCloud and SignerTypeGroup",
			fields: fields{
				Name: stepName,
				RunContext: &BlockRunContext{
					skipNotifications: true,
					WorkNumber:        "J001",
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						People: func() people.Service {
							res := new(peopleMock.Service)

							res.On("GetUser", "ravshan", false).
								Return(people.SSOUser{}, nil)

							return res
						}(),
						Storage: getTaskRunContext(),
					},
				},
			},

			args: args{
				ctx: c.Background(),
				ef: &entity.EriusFunc{
					BlockType:  BlockGoSignID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
					Params: func() []byte {
						r, _ := json.Marshal(&script.SignParams{
							SignatureType:      script.SignatureTypeUKEP,
							SignatureCarrier:   script.SignatureCarrierCloud,
							Type:               script.SignerTypeGroup,
							Signer:             "tester",
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})

						return r
					}(),
				},
			},
			wantErr: true,
		},
		{
			name: "SignatureTypeUNEP with SignatureCarrierToken",
			fields: fields{
				Name: stepName,
				RunContext: &BlockRunContext{
					skipNotifications: true,
					WorkNumber:        "J001",
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						People: func() people.Service {
							res := new(peopleMock.Service)

							res.On("GetUser", "ravshan", false).
								Return(people.SSOUser{}, nil)

							return res
						}(),
						Storage: getTaskRunContext(),
					},
				},
			},

			args: args{
				ctx: c.Background(),
				ef: &entity.EriusFunc{
					BlockType:  BlockGoSignID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
					Params: func() []byte {
						r, _ := json.Marshal(&script.SignParams{
							SignatureType:      script.SignatureTypeUNEP,
							SignatureCarrier:   script.SignatureCarrierToken,
							Type:               script.SignerTypeUser,
							Signer:             "tester",
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})

						return r
					}(),
				},
			},
			wantErr: false,
		},
		{
			name: "SignatureTypePEP with SignatureCarrierToken",
			fields: fields{
				Name: stepName,
				RunContext: &BlockRunContext{
					skipNotifications: true,
					WorkNumber:        "J001",
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						People: func() people.Service {
							res := new(peopleMock.Service)

							res.On("GetUser", "ravshan", false).
								Return(people.SSOUser{}, nil)

							return res
						}(),
						Storage: getTaskRunContext(),
					},
				},
			},

			args: args{
				ctx: c.Background(),
				ef: &entity.EriusFunc{
					BlockType:  BlockGoSignID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
					Params: func() []byte {
						r, _ := json.Marshal(&script.SignParams{
							SignatureType:      script.SignatureTypePEP,
							SignatureCarrier:   script.SignatureCarrierToken,
							Type:               script.SignerTypeUser,
							Signer:             "tester",
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})

						return r
					}(),
				},
			},
			wantErr: false,
		},
		{
			name: "SignatureTypeUKEP and SignatureCarrierToken",
			fields: fields{
				Name: stepName,
				RunContext: &BlockRunContext{
					skipNotifications: true,
					WorkNumber:        "J001",
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						s.SetValue("form_3.inn", "inn_1")
						s.SetValue("form_3.snils", "snils_1")
						s.SetValue("form_3.files", []entity.Attachment{
							{FileID: "uuid1"},
							{FileID: "uuid2"},
						})
						r, _ := json.Marshal(&SignData{
							Type: script.SignerTypeUser,
							Signers: map[string]struct{}{
								"tester": {},
							},
							FormsAccessibility: make([]script.FormAccessibility, 1),
						})
						s.State = map[string]json.RawMessage{
							stepName: r,
						}

						return s
					}(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						People: func() people.Service {
							res := new(peopleMock.Service)

							res.On("GetUser", "ravshan", false).
								Return(people.SSOUser{}, nil)

							return res
						}(),
						Storage: getTaskRunContext(),
					},
				},
			},

			args: args{
				ctx: c.Background(),
				ef: &entity.EriusFunc{
					BlockType:  BlockGoSignID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
					Params: func() []byte {
						r, _ := json.Marshal(&script.SignParams{
							SignatureType:    script.SignatureTypeUKEP,
							SignatureCarrier: script.SignatureCarrierToken,
							Type:             script.SignerTypeUser,
							SigningParamsPaths: script.SigningParamsPaths{
								INN:   "form_3.inn",
								SNILS: "form_3.snils",
								Files: []string{"form_3.files"},
							},
							Signer:             "tester",
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})

						return r
					}(),
				},
			},
			wantErr: false,
		},
		{
			name: "SignatureTypeUNEP with SignatureCarrierCloud",
			fields: fields{
				Name: stepName,
				RunContext: &BlockRunContext{
					skipNotifications: true,
					WorkNumber:        "J001",
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						People: func() people.Service {
							res := new(peopleMock.Service)

							res.On("GetUser", "ravshan", false).
								Return(people.SSOUser{}, nil)

							return res
						}(),
						Storage: getTaskRunContext(),
					},
				},
			},

			args: args{
				ctx: c.Background(),
				ef: &entity.EriusFunc{
					BlockType:  BlockGoSignID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
					Params: func() []byte {
						r, _ := json.Marshal(&script.SignParams{
							SignatureType:      script.SignatureTypeUNEP,
							SignatureCarrier:   script.SignatureCarrierCloud,
							Type:               script.SignerTypeUser,
							Signer:             "tester",
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})

						return r
					}(),
				},
			},
			wantErr: false,
		},
		{
			name: "SignatureTypePEP with SignatureCarrierCloud",
			fields: fields{
				Name: stepName,
				RunContext: &BlockRunContext{
					skipNotifications: true,
					WorkNumber:        "J001",
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						People: func() people.Service {
							res := new(peopleMock.Service)

							res.On("GetUser", "ravshan", false).
								Return(people.SSOUser{}, nil)

							return res
						}(),
						Storage: getTaskRunContext(),
					},
				},
			},

			args: args{
				ctx: c.Background(),
				ef: &entity.EriusFunc{
					BlockType:  BlockGoSignID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
					Params: func() []byte {
						r, _ := json.Marshal(&script.SignParams{
							SignatureType:      script.SignatureTypePEP,
							SignatureCarrier:   script.SignatureCarrierCloud,
							Type:               script.SignerTypeUser,
							Signer:             "tester",
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})

						return r
					}(),
				},
			},
			wantErr: false,
		},
		{
			name: "SignatureTypeUKEP and SignatureCarrierCloud",
			fields: fields{
				Name: stepName,
				RunContext: &BlockRunContext{
					skipNotifications: true,
					WorkNumber:        "J001",
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						People: func() people.Service {
							res := new(peopleMock.Service)

							res.On("GetUser", "ravshan", false).
								Return(people.SSOUser{}, nil)

							return res
						}(),
						Storage: getTaskRunContext(),
					},
				},
			},

			args: args{
				ctx: c.Background(),
				ef: &entity.EriusFunc{
					BlockType:  BlockGoSignID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
					Params: func() []byte {
						r, _ := json.Marshal(&script.SignParams{
							SignatureType:      script.SignatureTypeUKEP,
							SignatureCarrier:   script.SignatureCarrierCloud,
							Type:               script.SignerTypeUser,
							Signer:             "tester",
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})

						return r
					}(),
				},
			},
			wantErr: false,
		},
		{
			name: "signature type UNEP with empty signer",
			fields: fields{
				Name: stepName,
				RunContext: &BlockRunContext{
					skipNotifications: true,
					WorkNumber:        "J001",
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						People: func() people.Service {
							res := new(peopleMock.Service)

							res.On("GetUser", "ravshan", false).
								Return(people.SSOUser{}, nil)

							return res
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("GetApplicationData", "J001").Return("", nil)
							res.On("GetAdditionalForms", "J001", "sign").Return([]string{}, nil)
							res.On("UpdateStepContext",
								mock.MatchedBy(func(ctx c.Context) bool { return true }),
								mock.AnythingOfType("*db.UpdateStepRequest"),
							).Return(
								nil,
							)

							return res
						}(),
					},
				},
			},

			args: args{
				ctx: c.Background(),
				ef: &entity.EriusFunc{
					BlockType:  BlockGoSignID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
					Params: func() []byte {
						r, _ := json.Marshal(&script.SignParams{
							SignatureType:      script.SignatureTypeUNEP,
							SignatureCarrier:   script.SignatureCarrierAll,
							Type:               script.SignerTypeUser,
							Signer:             "",
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})

						return r
					}(),
				},
			},
			wantErr: true,
		},
		{
			name: "signature type PEP with empty signer",
			fields: fields{
				Name: stepName,
				RunContext: &BlockRunContext{
					skipNotifications: true,
					WorkNumber:        "J001",
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						People: func() people.Service {
							res := new(peopleMock.Service)

							res.On("GetUser", "ravshan", false).
								Return(people.SSOUser{}, nil)

							return res
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("GetApplicationData", "J001").Return("", nil)
							res.On("GetAdditionalForms", "J001", "sign").Return([]string{}, nil)
							res.On("UpdateStepContext",
								mock.MatchedBy(func(ctx c.Context) bool { return true }),
								mock.AnythingOfType("*db.UpdateStepRequest"),
							).Return(
								nil,
							)

							return res
						}(),
					},
				},
			},

			args: args{
				ctx: c.Background(),
				ef: &entity.EriusFunc{
					BlockType:  BlockGoSignID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
					Params: func() []byte {
						r, _ := json.Marshal(&script.SignParams{
							SignatureType:      script.SignatureTypePEP,
							SignatureCarrier:   script.SignatureCarrierAll,
							Type:               script.SignerTypeUser,
							Signer:             "",
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})

						return r
					}(),
				},
			},
			wantErr: true,
		},
		{
			name: "signature type UKEP with empty signer",
			fields: fields{
				Name: stepName,
				RunContext: &BlockRunContext{
					skipNotifications: true,
					WorkNumber:        "J001",
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						People: func() people.Service {
							res := new(peopleMock.Service)

							res.On("GetUser", "ravshan", false).
								Return(people.SSOUser{}, nil)

							return res
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("GetApplicationData", "J001").Return("", nil)
							res.On("GetAdditionalForms", "J001", "sign").Return([]string{}, nil)
							res.On("UpdateStepContext",
								mock.MatchedBy(func(ctx c.Context) bool { return true }),
								mock.AnythingOfType("*db.UpdateStepRequest"),
							).Return(
								nil,
							)

							return res
						}(),
					},
				},
			},

			args: args{
				ctx: c.Background(),
				ef: &entity.EriusFunc{
					BlockType:  BlockGoSignID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
					Params: func() []byte {
						r, _ := json.Marshal(&script.SignParams{
							SignatureType:      script.SignatureTypeUKEP,
							SignatureCarrier:   script.SignatureCarrierAll,
							Type:               script.SignerTypeUser,
							Signer:             "",
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})

						return r
					}(),
				},
			},
			wantErr: true,
		},
		{
			name: "signature type UNEP",
			fields: fields{
				Name: stepName,
				RunContext: &BlockRunContext{
					skipNotifications: true,
					WorkNumber:        "J001",
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						People: func() people.Service {
							res := new(peopleMock.Service)

							res.On("GetUser", "ravshan", false).
								Return(people.SSOUser{}, nil)

							return res
						}(),
						Storage: getTaskRunContext(),
					},
				},
			},

			args: args{
				ctx: c.Background(),
				ef: &entity.EriusFunc{
					BlockType:  BlockGoSignID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
					Params: func() []byte {
						r, _ := json.Marshal(&script.SignParams{
							SignatureType:      script.SignatureTypeUNEP,
							SignatureCarrier:   script.SignatureCarrierAll,
							Type:               script.SignerTypeUser,
							Signer:             "tester",
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})

						return r
					}(),
				},
			},
			wantErr: false,
		},
		{
			name: "signature type PEP",
			fields: fields{
				Name: stepName,
				RunContext: &BlockRunContext{
					skipNotifications: true,
					WorkNumber:        "J001",
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						People: func() people.Service {
							res := new(peopleMock.Service)

							res.On("GetUser", "ravshan", false).
								Return(people.SSOUser{}, nil)

							return res
						}(),
						Storage: getTaskRunContext(),
					},
				},
			},

			args: args{
				ctx: c.Background(),
				ef: &entity.EriusFunc{
					BlockType:  BlockGoSignID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
					Params: func() []byte {
						r, _ := json.Marshal(&script.SignParams{
							SignatureType:      script.SignatureTypePEP,
							SignatureCarrier:   script.SignatureCarrierAll,
							Type:               script.SignerTypeUser,
							Signer:             "tester",
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})

						return r
					}(),
				},
			},
			wantErr: false,
		},
		{
			name: "signature type UKEP",
			fields: fields{
				Name: stepName,
				RunContext: &BlockRunContext{
					skipNotifications: true,
					WorkNumber:        "J001",
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						s.SetValue("form_3.inn", "inn_1")
						s.SetValue("form_3.snils", "snils_1")
						s.SetValue("form_3.files", []entity.Attachment{
							{FileID: "uuid1"},
							{FileID: "uuid2"},
						})
						r, _ := json.Marshal(&SignData{
							Type: script.SignerTypeUser,
							Signers: map[string]struct{}{
								"tester": {},
							},
							FormsAccessibility: make([]script.FormAccessibility, 1),
						})
						s.State = map[string]json.RawMessage{
							stepName: r,
						}

						return s
					}(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						People: func() people.Service {
							res := new(peopleMock.Service)

							res.On("GetUser", "ravshan", false).
								Return(people.SSOUser{}, nil)

							return res
						}(),
						Storage: getTaskRunContext(),
					},
				},
			},

			args: args{
				ctx: c.Background(),
				ef: &entity.EriusFunc{
					BlockType:  BlockGoSignID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
					Params: func() []byte {
						r, _ := json.Marshal(&script.SignParams{
							SignatureType:    script.SignatureTypeUKEP,
							SignatureCarrier: script.SignatureCarrierAll,
							Type:             script.SignerTypeUser,
							SigningParamsPaths: script.SigningParamsPaths{
								INN:   "form_3.inn",
								SNILS: "form_3.snils",
								Files: []string{"form_3.files"},
							},
							Signer:             "tester",
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})

						return r
					}(),
				},
			},
			wantErr: false,
		},
		{
			name: "SignatureTypeUKEP and SignatureCarrierToken with missing inn",
			fields: fields{
				Name: stepName,
				RunContext: &BlockRunContext{
					skipNotifications: true,
					WorkNumber:        "J001",
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						s.SetValue("form_3.snils", "snils_1")
						s.SetValue("form_3.files", []entity.Attachment{
							{FileID: "uuid1"},
							{FileID: "uuid2"},
						})
						r, _ := json.Marshal(&SignData{
							Type: script.SignerTypeUser,
							Signers: map[string]struct{}{
								"tester": {},
							},
							FormsAccessibility: make([]script.FormAccessibility, 1),
						})
						s.State = map[string]json.RawMessage{
							stepName: r,
						}

						return s
					}(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						People: func() people.Service {
							res := new(peopleMock.Service)

							res.On("GetUser", "ravshan", false).
								Return(people.SSOUser{}, nil)

							return res
						}(),
						Storage: getTaskRunContext(),
					},
				},
			},

			args: args{
				ctx: c.Background(),
				ef: &entity.EriusFunc{
					BlockType:  BlockGoSignID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
					Params: func() []byte {
						r, _ := json.Marshal(&script.SignParams{
							SignatureType:    script.SignatureTypeUKEP,
							SignatureCarrier: script.SignatureCarrierToken,
							Type:             script.SignerTypeUser,
							SigningParamsPaths: script.SigningParamsPaths{
								INN:   "form_3.inn",
								SNILS: "form_3.snils",
								Files: []string{"form_3.files"},
							},
							Signer:             "tester",
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})

						return r
					}(),
				},
			},
			wantErr: false,
		},
		{
			name: "SignatureTypeUKEP and SignatureCarrierToken with missing snils",
			fields: fields{
				Name: stepName,
				RunContext: &BlockRunContext{
					skipNotifications: true,
					WorkNumber:        "J001",
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						s.SetValue("form_3.inn", "inn_1")
						s.SetValue("form_3.files", []entity.Attachment{
							{FileID: "uuid1"},
							{FileID: "uuid2"},
						})
						r, _ := json.Marshal(&SignData{
							Type: script.SignerTypeUser,
							Signers: map[string]struct{}{
								"tester": {},
							},
							FormsAccessibility: make([]script.FormAccessibility, 1),
						})
						s.State = map[string]json.RawMessage{
							stepName: r,
						}

						return s
					}(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						People: func() people.Service {
							res := new(peopleMock.Service)

							res.On("GetUser", "ravshan", false).
								Return(people.SSOUser{}, nil)

							return res
						}(),
						Storage: getTaskRunContext(),
					},
				},
			},

			args: args{
				ctx: c.Background(),
				ef: &entity.EriusFunc{
					BlockType:  BlockGoSignID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
					Params: func() []byte {
						r, _ := json.Marshal(&script.SignParams{
							SignatureType:    script.SignatureTypeUKEP,
							SignatureCarrier: script.SignatureCarrierToken,
							Type:             script.SignerTypeUser,
							SigningParamsPaths: script.SigningParamsPaths{
								INN:   "form_3.inn",
								SNILS: "form_3.snils",
								Files: []string{"form_3.files"},
							},
							Signer:             "tester",
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})

						return r
					}(),
				},
			},
			wantErr: false,
		},
		{
			name: "SignatureTypeUKEP and SignatureCarrierToken with missing files",
			fields: fields{
				Name: stepName,
				RunContext: &BlockRunContext{
					skipNotifications: true,
					WorkNumber:        "J001",
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						s.SetValue("form_3.inn", "inn_1")
						s.SetValue("form_3.snils", "snils_1")
						r, _ := json.Marshal(&SignData{
							Type: script.SignerTypeUser,
							Signers: map[string]struct{}{
								"tester": {},
							},
							FormsAccessibility: make([]script.FormAccessibility, 1),
						})
						s.State = map[string]json.RawMessage{
							stepName: r,
						}

						return s
					}(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						People: func() people.Service {
							res := new(peopleMock.Service)

							res.On("GetUser", "ravshan", false).
								Return(people.SSOUser{}, nil)

							return res
						}(),
						Storage: getTaskRunContext(),
					},
				},
			},

			args: args{
				ctx: c.Background(),
				ef: &entity.EriusFunc{
					BlockType:  BlockGoSignID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
					Params: func() []byte {
						r, _ := json.Marshal(&script.SignParams{
							SignatureType:    script.SignatureTypeUKEP,
							SignatureCarrier: script.SignatureCarrierToken,
							Type:             script.SignerTypeUser,
							SigningParamsPaths: script.SigningParamsPaths{
								INN:   "form_3.inn",
								SNILS: "form_3.snils",
								Files: []string{"form_3.files"},
							},
							Signer:             "tester",
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})

						return r
					}(),
				},
			},
			wantErr: false,
		},
		{
			name: "signature type UKEP with inn of wrong type",
			fields: fields{
				Name: stepName,
				RunContext: &BlockRunContext{
					skipNotifications: true,
					WorkNumber:        "J001",
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						s.SetValue("form_3.inn", 123)
						s.SetValue("form_3.snils", "snils_1")
						s.SetValue("form_3.files", []entity.Attachment{
							{FileID: "uuid1"},
							{FileID: "uuid2"},
						})
						r, _ := json.Marshal(&SignData{
							Type: script.SignerTypeUser,
							Signers: map[string]struct{}{
								"tester": {},
							},
							FormsAccessibility: make([]script.FormAccessibility, 1),
						})
						s.State = map[string]json.RawMessage{
							stepName: r,
						}

						return s
					}(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						People: func() people.Service {
							res := new(peopleMock.Service)

							res.On("GetUser", "ravshan", false).
								Return(people.SSOUser{}, nil)

							return res
						}(),
						Storage: getTaskRunContext(),
					},
				},
			},

			args: args{
				ctx: c.Background(),
				ef: &entity.EriusFunc{
					BlockType:  BlockGoSignID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
					Params: func() []byte {
						r, _ := json.Marshal(&script.SignParams{
							SignatureType:    script.SignatureTypeUKEP,
							SignatureCarrier: script.SignatureCarrierAll,
							Type:             script.SignerTypeUser,
							SigningParamsPaths: script.SigningParamsPaths{
								INN:   "form_3.inn",
								SNILS: "form_3.snils",
								Files: []string{"form_3.files"},
							},
							Signer:             "tester",
							FormsAccessibility: make([]script.FormAccessibility, 1),
							WorkType:           &workTypeVal,
							SLA:                &slaVal,
						})

						return r
					}(),
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gb := &GoSignBlock{
				Name:       tt.fields.Name,
				Title:      tt.fields.Title,
				Input:      tt.fields.Input,
				Output:     tt.fields.Output,
				Sockets:    tt.fields.NextStep,
				RunContext: tt.fields.RunContext,
			}

			err := gb.createState(tt.args.ctx, tt.args.ef)
			assert.Equalf(t, tt.wantErr, err != nil, fmt.Sprintf("createState(%v, %v)", tt.args.ctx, tt.args.ef))
		})
	}
}

func TestGoSignBlock_LoadState(t *testing.T) {
	const (
		invalidLogin = "foobar"
		stepName     = "sign"
	)

	type (
		fields struct {
			Name             string
			Title            string
			Input            map[string]string
			Output           map[string]string
			NextStep         []script.Socket
			SignData         *SignData
			RunContext       *BlockRunContext
			SigningRule      script.SigningRule
			SignLog          []SignLogEntry
			SignatureType    script.SignatureType
			SignatureCarrier script.SignatureCarrier
		}
		args struct {
			ctx  c.Context
			data *script.BlockUpdateData
			raw  json.RawMessage
		}
	)

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "empty raw",
			fields: fields{
				Name: stepName,
				RunContext: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
				},
			},
			args: args{
				ctx:  c.Background(),
				data: nil,
			},
			wantErr: true,
		},
		{
			name: "accept test",
			fields: fields{
				Name: stepName,
				RunContext: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
				},
			},
			args: args{
				ctx:  c.Background(),
				data: nil,
				raw: func() json.RawMessage {
					s := &SignData{
						Type: script.SignerTypeUser,
						Signers: map[string]struct{}{
							invalidLogin: {},
						},
					}
					data, _ := json.Marshal(s)

					return data
				}(),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gb := &GoSignBlock{
				Name:       tt.fields.Name,
				Title:      tt.fields.Title,
				Input:      tt.fields.Input,
				Output:     tt.fields.Output,
				Sockets:    tt.fields.NextStep,
				State:      tt.fields.SignData,
				RunContext: tt.fields.RunContext,
			}
			tt.fields.RunContext.UpdateData = tt.args.data
			err := gb.loadState(tt.args.raw)
			assert.Equalf(t, tt.wantErr, err != nil, fmt.Sprintf("loadState(%v)", tt.args.raw))
		})
	}
}

func unmarshalFromTestFile(t *testing.T, in string) json.RawMessage {
	b, err := os.ReadFile(in)
	if err != nil {
		t.Fatal(err)
	}

	return b
}

func TestGoSignActions(t *testing.T) {
	const (
		exampleExecutor = "example"
		stepName        = "exec"
	)

	login := "user1"
	delLogin1 := "delLogin1"

	type (
		fields struct {
			Name       string
			Title      string
			Input      map[string]string
			Output     map[string]string
			NextStep   []script.Socket
			SignData   *SignData
			RunContext *BlockRunContext
		}
		args struct {
			ctx  c.Context
			data *script.BlockUpdateData
		}
	)

	tests := []struct {
		name        string
		fields      fields
		args        args
		wantActions []MemberAction
	}{
		{
			name: "empty form accessibility",
			fields: fields{
				SignData: &SignData{
					SignatureType: script.SignatureTypeUNEP,
				},
				Name: stepName,
				RunContext: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						Storage: nil,
					},
				},
			},
			args: args{
				ctx:  c.Background(),
				data: nil,
			},
			wantActions: []MemberAction{
				{ID: "sign_sign", Type: "primary", Params: map[string]interface{}{"signature_type": script.SignatureTypeUNEP}},
				{ID: "sign_reject", Type: "secondary", Params: map[string]interface{}{"reason": []string{
					reasonWrongElement,
					reasonCancelRequired,
					reasonSentByMistake,
					reasonDontAgree},
				},
				},
				{ID: "add_approvers", Type: "other", Params: map[string]interface{}(nil)},
			},
		},
		{
			name: "one form ReadWrite",
			fields: fields{
				Name: stepName,
				SignData: &SignData{
					IsTakenInWork: true,
					Signers: map[string]struct{}{
						exampleExecutor: {},
					},
					SignatureType: script.SignatureTypeUNEP,
					FormsAccessibility: []script.FormAccessibility{
						{
							Name:        "Форма",
							NodeID:      "form_0",
							AccessType:  "ReadWrite",
							Description: "форма",
						},
					},
				},
				RunContext: &BlockRunContext{
					skipNotifications: false,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						s.State = map[string]json.RawMessage{
							"form_0": []byte{},
						}

						return s
					}(),
					Services: RunContextServices{
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							return res
						}(),
					},
				},
			},

			args: args{
				ctx: c.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    exampleExecutor,
					Action:     string(entity.TaskUpdateActionExecution),
					Parameters: []byte(`{"decision":"` + ExecutionDecisionExecuted + `"}`),
				},
			},
			wantActions: []MemberAction{
				{ID: "sign_sign", Type: "primary", Params: map[string]interface{}{"signature_type": script.SignatureTypeUNEP}},
				{ID: "sign_reject", Type: "secondary", Params: map[string]interface{}{"reason": []string{
					reasonWrongElement,
					reasonCancelRequired,
					reasonSentByMistake,
					reasonDontAgree},
				}},
				{ID: "fill_form", Type: "custom", Params: map[string]interface{}{"form_name": []string{"form_0"}}},
				{ID: "add_approvers", Type: "other", Params: map[string]interface{}(nil)},
			},
		},
		{
			name: "two form (ReadWrite)",
			fields: fields{
				Name: stepName,
				SignData: &SignData{
					IsTakenInWork: true,
					Signers: map[string]struct{}{
						exampleExecutor: {},
					},
					SignatureType: script.SignatureTypeUNEP,
					FormsAccessibility: []script.FormAccessibility{
						{
							Name:        "Форма",
							NodeID:      "form_0",
							AccessType:  "ReadWrite",
							Description: "форма",
						},
						{
							Name:        "Форма",
							NodeID:      "form_1",
							AccessType:  "ReadWrite",
							Description: "форма",
						},
					},
				},
				RunContext: &BlockRunContext{
					skipNotifications: false,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						s.State = map[string]json.RawMessage{
							"form_0": []byte{},
							"form_1": []byte{},
						}

						return s
					}(),
					Services: RunContextServices{
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							return res
						}(),
					},
				},
			},

			args: args{
				ctx: c.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    exampleExecutor,
					Action:     string(entity.TaskUpdateActionExecution),
					Parameters: []byte(`{"decision":"` + ExecutionDecisionExecuted + `"}`),
				},
			},
			wantActions: []MemberAction{
				{ID: "sign_sign", Type: "primary", Params: map[string]interface{}{"signature_type": script.SignatureTypeUNEP}},
				{ID: "sign_reject", Type: "secondary", Params: map[string]interface{}{"reason": []string{
					reasonWrongElement,
					reasonCancelRequired,
					reasonSentByMistake,
					reasonDontAgree},
				}},
				{ID: "fill_form", Type: "custom", Params: map[string]interface{}{"form_name": []string{"form_0", "form_1"}}},
				{ID: "add_approvers", Type: "other", Params: map[string]interface{}(nil)},
			},
		},
		{
			name: "Two form is filled true - ok (ReadWrite & RequiredFill)",
			fields: fields{
				Name: stepName,
				SignData: &SignData{
					IsTakenInWork: true,
					Signers: map[string]struct{}{
						login: {},
					},
					SignatureType: script.SignatureTypeUNEP,
					FormsAccessibility: []script.FormAccessibility{
						{
							Name:        "Форма",
							NodeID:      "form_0",
							AccessType:  "ReadWrite",
							Description: "форма",
						},
						{
							Name:        "Форма",
							NodeID:      "form_1",
							AccessType:  "RequiredFill",
							Description: "форма",
						},
					},
				},
				RunContext: &BlockRunContext{
					skipNotifications: false,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						s.State = map[string]json.RawMessage{
							"form_0": []byte{},
							"form_1": func() []byte {
								marshalForm, _ := json.Marshal(FormData{
									IsFilled: true,
									Executors: map[string]struct{}{
										login: {},
									},
									ActualExecutor: &login,
									ChangesLog: []ChangesLogItem{
										{
											Executor: login,
										},
									},
								})

								return marshalForm
							}(),
						}
						return s
					}(),
					Services: RunContextServices{
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							return res
						}(),
						HumanTasks: func() human_tasks.Service {
							htMock := mocks2.DelegationServiceClient{}

							htMock.On("GetDelegationsFromLogin", c.Background(), "users1").Return(nil, human_tasks.Delegations{})

							req := &delegationht.GetDelegationsRequest{
								FilterBy:  "fromLogin",
								FromLogin: login,
							}

							htMock.On("getDelegationsInternal", c.Background(), req).Return(human_tasks.Delegations{
								{
									ToLogin:   delLogin1,
									FromLogin: login,
								},
							}, nil)
							htMock.On("FilterByType", "users1").Return(delegationht.GetDelegationsResponse{
								Delegations: []*delegationht.Delegation{
									{
										FromUser: &delegationht.User{
											Fullname: login,
										},
									},
								},
							})
							htMock.On("GetDelegates", "users1").Return([]string{"a"})

							ht, _ := nocache.NewService(&human_tasks.Config{}, nil)
							ht.SetCli(&htMock)

							return ht
						}(),
					},
				},
			},

			args: args{
				ctx: c.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    exampleExecutor,
					Action:     string(entity.TaskUpdateActionExecution),
					Parameters: []byte(`{"decision":"` + ExecutionDecisionExecuted + `"}`),
				},
			},
			wantActions: []MemberAction{
				{ID: "sign_sign", Type: "primary", Params: map[string]interface{}{"signature_type": script.SignatureTypeUNEP}},
				{ID: "sign_reject", Type: "secondary", Params: map[string]interface{}{"reason": []string{
					reasonWrongElement,
					reasonCancelRequired,
					reasonSentByMistake,
					reasonDontAgree},
				}},
				{ID: "fill_form", Type: "custom", Params: map[string]interface{}{"form_name": []string{"form_0", "form_1"}}},
				{ID: "add_approvers", Type: "other", Params: map[string]interface{}(nil)},
			},
		},
		{
			name: "Two form is filled true and not exist ChangeLog - bad (ReadWrite & RequiredFill)",
			fields: fields{
				Name: stepName,
				SignData: &SignData{
					IsTakenInWork: true,
					Signers: map[string]struct{}{
						login: {},
					},
					SignatureType: script.SignatureTypeUNEP,
					FormsAccessibility: []script.FormAccessibility{
						{
							Name:        "Форма",
							NodeID:      "form_0",
							AccessType:  "ReadWrite",
							Description: "форма",
						},
						{
							Name:        "Форма",
							NodeID:      "form_1",
							AccessType:  "RequiredFill",
							Description: "форма",
						},
					},
				},
				RunContext: &BlockRunContext{
					skipNotifications: false,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						s.State = map[string]json.RawMessage{
							"form_0": []byte{},
							"form_1": func() []byte {
								marshalForm, _ := json.Marshal(FormData{
									IsFilled: true,
									Executors: map[string]struct{}{
										login: {},
									},
									ActualExecutor: &login,
								})

								return marshalForm
							}(),
						}
						return s
					}(),
					Services: RunContextServices{
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							return res
						}(),
						HumanTasks: func() human_tasks.Service {
							htMock := mocks2.DelegationServiceClient{}

							htMock.On("GetDelegationsFromLogin", c.Background(), "users1").Return(nil, human_tasks.Delegations{})

							req := &delegationht.GetDelegationsRequest{
								FilterBy:  "fromLogin",
								FromLogin: login,
							}

							htMock.On("getDelegationsInternal", c.Background(), req).Return(human_tasks.Delegations{
								{
									ToLogin:   delLogin1,
									FromLogin: login,
								},
							}, nil)
							htMock.On("FilterByType", "users1").Return(delegationht.GetDelegationsResponse{
								Delegations: []*delegationht.Delegation{
									{
										FromUser: &delegationht.User{
											Fullname: login,
										},
									},
								},
							})
							htMock.On("GetDelegates", "users1").Return([]string{"a"})

							ht, _ := nocache.NewService(&human_tasks.Config{}, nil)
							ht.SetCli(&htMock)

							return ht
						}(),
					},
				},
			},

			args: args{
				ctx: c.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    exampleExecutor,
					Action:     string(entity.TaskUpdateActionExecution),
					Parameters: []byte(`{"decision":"` + ExecutionDecisionExecuted + `"}`),
				},
			},
			wantActions: []MemberAction{
				{ID: "sign_sign", Type: "primary", Params: map[string]interface{}{"disabled": true, "hint_description": "Для продолжения работы над заявкой, необходимо {fill_form}"}},
				{ID: "sign_reject", Type: "secondary", Params: map[string]interface{}{"reason": []string{
					reasonWrongElement,
					reasonCancelRequired,
					reasonSentByMistake,
					reasonDontAgree},
				}},
				{ID: "fill_form", Type: "custom", Params: map[string]interface{}{"form_name": []string{"form_0", "form_1"}}},
				{ID: "add_approvers", Type: "other", Params: map[string]interface{}(nil)},
			},
		},
		{
			name: "Two form - is filled false (ReadWrite & RequiredFill)",
			fields: fields{
				Name: stepName,
				SignData: &SignData{
					IsTakenInWork: true,
					Signers: map[string]struct{}{
						exampleExecutor: {},
					},
					SignatureType: script.SignatureTypeUNEP,
					FormsAccessibility: []script.FormAccessibility{
						{
							Name:        "Форма",
							NodeID:      "form_0",
							AccessType:  "ReadWrite",
							Description: "форма",
						},
						{
							Name:        "Форма",
							NodeID:      "form_1",
							AccessType:  "RequiredFill",
							Description: "форма",
						},
					},
				},
				RunContext: &BlockRunContext{
					skipNotifications: false,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						s.State = map[string]json.RawMessage{
							"form_0": []byte{},
							"form_1": func() []byte {
								marshalForm, _ := json.Marshal(FormData{
									IsFilled:       false,
									ActualExecutor: &login,
								})

								return marshalForm
							}(),
						}
						return s
					}(),
					Services: RunContextServices{
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							return res
						}(),
						HumanTasks: func() human_tasks.Service {
							htMock := mocks2.DelegationServiceClient{}

							htMock.On("GetDelegationsFromLogin", c.Background(), "users1").Return(nil, human_tasks.Delegations{})

							req := &delegationht.GetDelegationsRequest{
								FilterBy:  "fromLogin",
								FromLogin: login,
							}

							htMock.On("getDelegationsInternal", c.Background(), req).Return(human_tasks.Delegations{
								{
									ToLogin:   delLogin1,
									FromLogin: login,
								},
							}, nil)
							htMock.On("FilterByType", "users1").Return(delegationht.GetDelegationsResponse{
								Delegations: []*delegationht.Delegation{
									{
										FromUser: &delegationht.User{
											Fullname: login,
										},
									},
								},
							})
							htMock.On("GetDelegates", "users1").Return([]string{"a"})

							ht, _ := nocache.NewService(&human_tasks.Config{}, nil)
							ht.SetCli(&htMock)

							return ht
						}(),
					},
				},
			},

			args: args{
				ctx: c.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    exampleExecutor,
					Action:     string(entity.TaskUpdateActionExecution),
					Parameters: []byte(`{"decision":"` + ExecutionDecisionExecuted + `"}`),
				},
			},
			wantActions: []MemberAction{
				{ID: "sign_sign", Type: "primary", Params: map[string]interface{}{"disabled": true, "hint_description": "Для продолжения работы над заявкой, необходимо {fill_form}"}},
				{ID: "sign_reject", Type: "secondary", Params: map[string]interface{}{"reason": []string{
					reasonWrongElement,
					reasonCancelRequired,
					reasonSentByMistake,
					reasonDontAgree},
				}},
				{ID: "fill_form", Type: "custom", Params: map[string]interface{}{"form_name": []string{"form_0", "form_1"}}},
				{ID: "add_approvers", Type: "other", Params: map[string]interface{}(nil)},
			},
		},
		{
			name: "Two form is filled (RequiredFill)",
			fields: fields{
				Name: stepName,
				SignData: &SignData{
					IsTakenInWork: true,
					SignatureType: script.SignatureTypeUNEP,
					Signers: map[string]struct{}{
						login: {},
					},
					FormsAccessibility: []script.FormAccessibility{
						{
							Name:        "Форма",
							NodeID:      "form_0",
							AccessType:  "RequiredFill",
							Description: "форма",
						},
						{
							Name:        "Форма",
							NodeID:      "form_1",
							AccessType:  "RequiredFill",
							Description: "форма",
						},
					},
				},
				RunContext: &BlockRunContext{
					skipNotifications: false,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						s.State = map[string]json.RawMessage{
							"form_0": func() []byte {
								marshalForm, _ := json.Marshal(FormData{
									IsFilled: true,
									Executors: map[string]struct{}{
										login: {},
									},
									ActualExecutor: &login,
									ChangesLog: []ChangesLogItem{
										{
											Executor: login,
										},
									},
								})

								return marshalForm
							}(),
							"form_1": func() []byte {
								marshalForm, _ := json.Marshal(FormData{
									IsFilled: true,
									Executors: map[string]struct{}{
										login: {},
									},
									ActualExecutor: &login,
									ChangesLog: []ChangesLogItem{
										{
											Executor: login,
										},
									},
								})

								return marshalForm
							}(),
						}
						return s
					}(),
					Services: RunContextServices{
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							return res
						}(),
						HumanTasks: func() human_tasks.Service {
							htMock := mocks2.DelegationServiceClient{}

							htMock.On("GetDelegationsFromLogin", c.Background(), "users1").Return(nil, human_tasks.Delegations{})

							req := &delegationht.GetDelegationsRequest{
								FilterBy:  "fromLogin",
								FromLogin: login,
							}

							htMock.On("getDelegationsInternal", c.Background(), req).Return(human_tasks.Delegations{
								{
									ToLogin:   delLogin1,
									FromLogin: login,
								},
							}, nil)
							htMock.On("FilterByType", "users1").Return(delegationht.GetDelegationsResponse{
								Delegations: []*delegationht.Delegation{
									{
										FromUser: &delegationht.User{
											Fullname: login,
										},
									},
								},
							})
							htMock.On("GetDelegates", "users1").Return([]string{"a"})

							ht, _ := nocache.NewService(&human_tasks.Config{}, nil)
							ht.SetCli(&htMock)

							return ht
						}(),
					},
				},
			},

			args: args{
				ctx: c.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    exampleExecutor,
					Action:     string(entity.TaskUpdateActionExecution),
					Parameters: []byte(`{"decision":"` + ExecutionDecisionExecuted + `"}`),
				},
			},
			wantActions: []MemberAction{
				{ID: "sign_sign", Type: "primary", Params: map[string]interface{}{"signature_type": script.SignatureTypeUNEP}},
				{ID: "sign_reject", Type: "secondary", Params: map[string]interface{}{"reason": []string{
					reasonWrongElement,
					reasonCancelRequired,
					reasonSentByMistake,
					reasonDontAgree},
				}},
				{ID: "fill_form", Type: "custom", Params: map[string]interface{}{"form_name": []string{"form_0", "form_1"}}},
				{ID: "add_approvers", Type: "other", Params: map[string]interface{}(nil)},
			},
		},
		{
			name: "Two form is filled and not filled (RequiredFill)",
			fields: fields{
				Name: stepName,
				SignData: &SignData{
					IsTakenInWork: true,
					SignatureType: script.SignatureTypeUNEP,
					Signers: map[string]struct{}{
						login: {},
					},
					FormsAccessibility: []script.FormAccessibility{
						{
							Name:        "Форма",
							NodeID:      "form_0",
							AccessType:  "RequiredFill",
							Description: "форма",
						},
						{
							Name:        "Форма",
							NodeID:      "form_1",
							AccessType:  "RequiredFill",
							Description: "форма",
						},
					},
				},
				RunContext: &BlockRunContext{
					skipNotifications: false,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						s.State = map[string]json.RawMessage{
							"form_0": func() []byte {
								marshalForm, _ := json.Marshal(FormData{
									IsFilled: true,
									Executors: map[string]struct{}{
										login: {},
									},
									ActualExecutor: &login,
									ChangesLog: []ChangesLogItem{
										{
											Executor: login,
										},
									},
								})

								return marshalForm
							}(),
							"form_1": func() []byte {
								marshalForm, _ := json.Marshal(FormData{
									IsFilled: false,
									Executors: map[string]struct{}{
										"user1": {},
									},
									ActualExecutor: &login,
								})

								return marshalForm
							}(),
						}
						return s
					}(),
					Services: RunContextServices{
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							return res
						}(),
						HumanTasks: func() human_tasks.Service {
							htMock := mocks2.DelegationServiceClient{}

							htMock.On("GetDelegationsFromLogin", c.Background(), "users1").Return(nil, human_tasks.Delegations{})

							req := &delegationht.GetDelegationsRequest{
								FilterBy:  "fromLogin",
								FromLogin: login,
							}

							htMock.On("getDelegationsInternal", c.Background(), req).Return(human_tasks.Delegations{
								{
									ToLogin:   delLogin1,
									FromLogin: login,
								},
							}, nil)
							htMock.On("FilterByType", "users1").Return(delegationht.GetDelegationsResponse{
								Delegations: []*delegationht.Delegation{
									{
										FromUser: &delegationht.User{
											Fullname: login,
										},
									},
								},
							})
							htMock.On("GetDelegates", "users1").Return([]string{"a"})

							ht, _ := nocache.NewService(&human_tasks.Config{}, nil)
							ht.SetCli(&htMock)

							return ht
						}(),
					},
				},
			},

			args: args{
				ctx: c.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    exampleExecutor,
					Action:     string(entity.TaskUpdateActionExecution),
					Parameters: []byte(`{"decision":"` + ExecutionDecisionExecuted + `"}`),
				},
			},
			wantActions: []MemberAction{
				{ID: "sign_sign", Type: "primary", Params: map[string]interface{}{"disabled": true, "hint_description": "Для продолжения работы над заявкой, необходимо {fill_form}"}},
				{ID: "sign_reject", Type: "secondary", Params: map[string]interface{}{"reason": []string{
					reasonWrongElement,
					reasonCancelRequired,
					reasonSentByMistake,
					reasonDontAgree},
				}},
				{ID: "fill_form", Type: "custom", Params: map[string]interface{}{"form_name": []string{"form_0", "form_1"}}},
				{ID: "add_approvers", Type: "other", Params: map[string]interface{}(nil)},
			},
		},
		{
			name: "Two form - not filled (RequiredFill)",
			fields: fields{
				Name: stepName,
				SignData: &SignData{
					IsTakenInWork: true,
					Signers: map[string]struct{}{
						login: {},
					},
					SignatureType: script.SignatureTypeUNEP,
					FormsAccessibility: []script.FormAccessibility{
						{
							Name:        "Форма",
							NodeID:      "form_0",
							AccessType:  "RequiredFill",
							Description: "форма",
						},
						{
							Name:        "Форма",
							NodeID:      "form_1",
							AccessType:  "RequiredFill",
							Description: "форма",
						},
					},
				},
				RunContext: &BlockRunContext{
					skipNotifications: false,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						s.State = map[string]json.RawMessage{
							"form_0": func() []byte {
								marshalForm, _ := json.Marshal(FormData{
									IsFilled: false,
									Executors: map[string]struct{}{
										login: {},
									},
									ActualExecutor: &delLogin1,
									ChangesLog: []ChangesLogItem{
										{
											Executor: login,
										},
									},
								})

								return marshalForm
							}(),
							"form_1": func() []byte {
								marshalForm, _ := json.Marshal(FormData{
									IsFilled: false,
									Executors: map[string]struct{}{
										login: {},
									},
									ActualExecutor: &login,
									ChangesLog: []ChangesLogItem{
										{
											Executor: login,
										},
									},
								})

								return marshalForm
							}(),
						}
						return s
					}(),
					Services: RunContextServices{
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							return res
						}(),
						HumanTasks: func() human_tasks.Service {
							htMock := mocks2.DelegationServiceClient{}

							htMock.On("GetDelegationsFromLogin", c.Background(), "users1").Return(nil, human_tasks.Delegations{})

							req := &delegationht.GetDelegationsRequest{
								FilterBy:  "fromLogin",
								FromLogin: login,
							}

							htMock.On("getDelegationsInternal", c.Background(), req).Return(human_tasks.Delegations{
								{
									ToLogin:   delLogin1,
									FromLogin: login,
								},
							}, nil)
							htMock.On("FilterByType", "users1").Return(delegationht.GetDelegationsResponse{
								Delegations: []*delegationht.Delegation{
									{
										FromUser: &delegationht.User{
											Fullname: login,
										},
									},
								},
							})
							htMock.On("GetDelegates", "users1").Return([]string{"a"})

							ht, _ := nocache.NewService(&human_tasks.Config{}, nil)
							ht.SetCli(&htMock)

							return ht
						}(),
					},
				},
			},

			args: args{
				ctx: c.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    exampleExecutor,
					Action:     string(entity.TaskUpdateActionExecution),
					Parameters: []byte(`{"decision":"` + ExecutionDecisionExecuted + `"}`),
				},
			},
			wantActions: []MemberAction{
				{ID: "sign_sign", Type: "primary", Params: map[string]interface{}{"disabled": true, "hint_description": "Для продолжения работы над заявкой, необходимо {fill_form}"}},
				{ID: "sign_reject", Type: "secondary", Params: map[string]interface{}{"reason": []string{
					reasonWrongElement,
					reasonCancelRequired,
					reasonSentByMistake,
					reasonDontAgree},
				}},
				{ID: "fill_form", Type: "custom", Params: map[string]interface{}{"form_name": []string{"form_0", "form_1"}}},
				{ID: "add_approvers", Type: "other", Params: map[string]interface{}(nil)},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gb := &GoSignBlock{
				Name:       tt.fields.Name,
				Title:      tt.fields.Title,
				Input:      tt.fields.Input,
				Output:     tt.fields.Output,
				Sockets:    tt.fields.NextStep,
				State:      tt.fields.SignData,
				RunContext: tt.fields.RunContext,
			}
			tt.fields.RunContext.UpdateData = tt.args.data

			actions := gb.signActions(login)
			assert.Equal(t, tt.wantActions, actions, fmt.Sprintf("signActions(%v)", login))
		})
	}
}
