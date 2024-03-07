package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db/mocks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	humanTasks "gitlab.services.mts.ru/jocasta/pipeliner/internal/humantasks"
	htMocks "gitlab.services.mts.ru/jocasta/pipeliner/internal/humantasks/mocks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc"
	serviceDeskMocks "gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc/mocks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sla"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"

	delegationht "gitlab.services.mts.ru/jocasta/human-tasks/pkg/proto/gen/proto/go/delegation"
)

func TestApproverData_SetDecision(t *testing.T) {
	const (
		login                       = "example"
		decision     ApproverAction = ApproverActionReject
		comment                     = "blah blah blah"
		invalidLogin                = "foobar"
	)

	type (
		fields struct {
			Type           script.ApproverType
			Approvers      map[string]struct{}
			Decision       *ApproverAction
			Comment        *string
			ActualApprover *string
		}
		args struct {
			login       string
			decision    ApproverAction
			comment     string
			delegations humanTasks.Delegations
		}
	)

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "approver not found",
			fields: fields{
				Type: script.ApproverTypeUser,
				Approvers: map[string]struct{}{
					login: {},
				},
				Decision: func() *ApproverAction {
					res := decision

					return &res
				}(),
				Comment:        nil,
				ActualApprover: nil,
			},
			args: args{
				login:       invalidLogin,
				decision:    decision,
				comment:     comment,
				delegations: []humanTasks.Delegation{},
			},
			wantErr: true,
		},
		{
			name: "decision already set",
			fields: fields{
				Type: script.ApproverTypeUser,
				Approvers: map[string]struct{}{
					login: {},
				},
				Decision: func() *ApproverAction {
					res := decision

					return &res
				}(),
				Comment: func() *string {
					res := comment

					return &res
				}(),
				ActualApprover: func() *string {
					res := login

					return &res
				}(),
			},
			args: args{
				login:       login,
				decision:    decision,
				comment:     comment,
				delegations: []humanTasks.Delegation{},
			},
			wantErr: true,
		},
		{
			name: "unknown decision",
			fields: fields{
				Type: script.ApproverTypeUser,
				Approvers: map[string]struct{}{
					login: {},
				},
				Decision: func() *ApproverAction {
					res := decision

					return &res
				}(),
				Comment:        nil,
				ActualApprover: nil,
			},
			args: args{
				login:       login,
				decision:    ApproverAction("unknown"),
				comment:     comment,
				delegations: []humanTasks.Delegation{},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := tt.fields.Decision.ToDecision()
			a := &ApproverData{
				Type:           tt.fields.Type,
				Approvers:      tt.fields.Approvers,
				Decision:       &d,
				Comment:        tt.fields.Comment,
				ActualApprover: tt.fields.ActualApprover,
			}

			if err := a.SetDecision(tt.args.login, tt.args.comment, tt.args.decision.ToDecision(), []entity.Attachment{}, tt.args.delegations); (err != nil) != tt.wantErr {
				t.Errorf(
					"SetDecision(%v, %v, %v)",
					tt.args.login,
					tt.args.decision,
					tt.args.comment,
				)
			}
		})
	}
}

func TestApproverData_SetDecisionByAdditionalApprover(t *testing.T) {
	var (
		login            = "login"
		login2           = "login2"
		login3           = "login3"
		login4           = "login4"
		decisionRejected = ApproverDecisionRejected
		decisionApproved = ApproverDecisionApproved
		comment          = "blah blah blah"
		question         = "need approval"
	)

	timeNow := time.Now()

	type fields struct {
		Decision            *ApproverDecision
		AdditionalApprovers []AdditionalApprover
	}

	type args struct {
		login       string
		params      additionalApproverUpdateParams
		delegations humanTasks.Delegations
	}

	tests := []struct {
		name                    string
		fields                  fields
		args                    args
		want                    []string
		wantErr                 bool
		wantAdditionalApprovers []AdditionalApprover
	}{
		{
			name: "additional approver not found",
			fields: fields{
				Decision:            nil,
				AdditionalApprovers: nil,
			},
			args: args{
				login: login,
				params: additionalApproverUpdateParams{
					Decision: decisionRejected,
					Comment:  comment,
				},
				delegations: []humanTasks.Delegation{},
			},
			want:                    nil,
			wantErr:                 true,
			wantAdditionalApprovers: nil,
		},
		{
			name: "decision already set",
			fields: fields{
				Decision:            &decisionRejected,
				AdditionalApprovers: nil,
			},
			args: args{
				login: login,
				params: additionalApproverUpdateParams{
					Decision: decisionRejected,
					Comment:  comment,
				},
				delegations: []humanTasks.Delegation{},
			},
			want:                    nil,
			wantErr:                 true,
			wantAdditionalApprovers: nil,
		},
		{
			name: "valid case",
			fields: fields{
				Decision: nil,
				AdditionalApprovers: []AdditionalApprover{
					{
						ApproverLogin:     login,
						BaseApproverLogin: login2,
						Question:          &question,
					},
					{
						ApproverLogin:     login,
						BaseApproverLogin: login3,
					},
					{
						ApproverLogin:     login,
						BaseApproverLogin: login4,
						Question:          &question,
						Comment:           &comment,
						Decision:          &decisionApproved,
					},
				},
			},
			args: args{
				login: login,
				params: additionalApproverUpdateParams{
					Decision: decisionRejected,
					Comment:  comment,
				},
				delegations: []humanTasks.Delegation{},
			},
			want:    []string{"login2", "login3"},
			wantErr: false,
			wantAdditionalApprovers: []AdditionalApprover{
				{
					ApproverLogin:     login,
					BaseApproverLogin: login2,
					Question:          &question,
					Comment:           &comment,
					Decision:          &decisionRejected,
					DecisionTime:      &timeNow,
				},
				{
					ApproverLogin:     login,
					BaseApproverLogin: login3,
					Comment:           &comment,
					Decision:          &decisionRejected,
					DecisionTime:      &timeNow,
				},
				{
					ApproverLogin:     login,
					BaseApproverLogin: login4,
					Question:          &question,
					Comment:           &comment,
					Decision:          &decisionApproved,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &ApproverData{
				Decision:            tt.fields.Decision,
				AdditionalApprovers: tt.fields.AdditionalApprovers,
			}
			got, err := a.SetDecisionByAdditionalApprover(tt.args.login, tt.args.params, tt.args.delegations)
			if (err != nil) != tt.wantErr {
				t.Errorf(
					"SetDecisionByAdditionalApprover(%v, %v)",
					tt.args.login,
					tt.args.params,
				)
			}

			assert.Equal(t, tt.want, got,
				fmt.Sprintf("Incorrect result. SetDecisionByAdditionalApprover() method. Expect %v, got %v", tt.want, got))
			assert.Equal(t, len(a.AdditionalApprovers), len(tt.wantAdditionalApprovers))
			for i := range tt.wantAdditionalApprovers {
				wantA := tt.wantAdditionalApprovers[i]
				gotA := a.AdditionalApprovers[i]
				check := wantA.ApproverLogin == gotA.ApproverLogin && wantA.BaseApproverLogin == gotA.BaseApproverLogin && *wantA.Decision == *gotA.Decision && *wantA.Comment == *gotA.Comment
				assert.Equal(t, check, true)
			}
		})
	}
}

func Test_createGoApproverBlock(t *testing.T) {
	const (
		example                  = "example"
		title                    = "title"
		login                    = "login1"
		shortTitle               = "Нода Согласование"
		approversFromSchema      = "a.var1;b.var2;var3"
		approversFromSchemaSlice = "sd_app_0.application_body.users"
		approverGroupID          = "uuid13456"
		loginFromSlice0          = "pilzner1"
		loginFromSlice1          = "pupok_na_jope"
	)

	myStorage := makeStorage()
	varStore := store.NewStore()

	varStore.SetValue("sd_app_0.application_body.users", []interface{}{
		map[string]interface{}{
			"username": loginFromSlice0,
		},
		map[string]interface{}{
			"username": loginFromSlice1,
		},
		map[string]interface{}{
			"userName": "noname",
		},
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
		name    string
		args    args
		want    *GoApproverBlock
		wantErr bool
	}{
		{
			name: "can not get approver parameters",
			args: args{
				name: example,
				runCtx: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						Storage: myStorage,
					},
				},
				ef: &entity.EriusFunc{
					BlockType:  BlockGoApproverID,
					Title:      title,
					ShortTitle: shortTitle,
					Input:      nil,
					Output:     nil,
					Params:     nil,
					Sockets:    next,
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid approver parameters",
			args: args{
				name: example,
				runCtx: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						Storage: myStorage,
					},
				},
				ef: &entity.EriusFunc{
					BlockType:  BlockGoApproverID,
					Title:      title,
					ShortTitle: shortTitle,
					Input:      nil,
					Output:     nil,
					Params:     []byte("{}"),
					Sockets:    next,
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid approvement rule for many approvers from schema",
			args: args{
				name: example,
				runCtx: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						Storage: myStorage,
					},
				},
				ef: &entity.EriusFunc{
					BlockType:  BlockGoApproverID,
					Title:      title,
					ShortTitle: shortTitle,
					Input:      nil,
					Output:     nil,
					Params: func() []byte {
						r, _ := json.Marshal(&script.ApproverParams{
							Type:            script.ApproverTypeFromSchema,
							Approver:        approversFromSchema,
							SLA:             1,
							ApprovementRule: "",
						})

						return r
					}(),
					Sockets: next,
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid approvement rule for group",
			args: args{
				name: example,
				runCtx: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						Storage: myStorage,
					},
				},
				ef: &entity.EriusFunc{
					BlockType:  BlockGoApproverID,
					Title:      title,
					ShortTitle: shortTitle,
					Input:      nil,
					Output:     nil,
					Params: func() []byte {
						r, _ := json.Marshal(&script.ApproverParams{
							Type:             script.ApproverTypeGroup,
							ApproversGroupID: approverGroupID,
							SLA:              1,
							ApprovementRule:  "",
						})

						return r
					}(),
					Sockets: next,
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "get logins from slice of SsoPerson",
			args: args{
				name: example,
				runCtx: &BlockRunContext{
					skipNotifications: true,
					VarStore:          varStore,
					Services: RunContextServices{
						Storage: myStorage,
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
					},
				},
				ef: &entity.EriusFunc{
					BlockType:  BlockGoApproverID,
					Title:      title,
					ShortTitle: shortTitle,
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
							keyOutputApprover: {
								Type:   "string",
								Global: example,
							},
						},
					},
					Params: func() []byte {
						r, _ := json.Marshal(&script.ApproverParams{
							Type:               script.ApproverTypeFromSchema,
							Approver:           approversFromSchemaSlice,
							FormsAccessibility: make([]script.FormAccessibility, 0),
						})

						return r
					}(),
					Sockets: next,
				},
			},
			want: &GoApproverBlock{
				Name:      example,
				ShortName: shortTitle,
				Title:     title,
				Input: map[string]string{
					"foo": "bar",
				},
				Output: map[string]string{
					keyOutputApprover: example,
				},
				happenedEvents: make([]entity.NodeEvent, 0),
				State: &ApproverData{
					Type: script.ApproverTypeFromSchema,
					Approvers: map[string]struct{}{
						loginFromSlice0: {},
						loginFromSlice1: {},
					},
					Decision:           nil,
					Comment:            nil,
					ActualApprover:     nil,
					AutoAction:         nil,
					ApprovementRule:    script.AnyOfApprovementRequired,
					ApproverLog:        make([]ApproverLogEntry, 0),
					FormsAccessibility: make([]script.FormAccessibility, 0),
					ActionList: []Action{
						{
							ID:    DefaultSocketID,
							Title: script.DefaultSocketTitle,
						},
						{
							ID:    rejectedSocketID,
							Title: script.RejectSocketTitle,
						},
					},
					WorkType: string(sla.WorkTypeN85),
				},
				Sockets: entity.ConvertSocket(next),
			},
			wantErr: false,
		},
		{
			name: "acceptance test",
			args: args{
				name: example,
				runCtx: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						Storage: myStorage,
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
					},
				},
				ef: &entity.EriusFunc{
					BlockType:  BlockGoApproverID,
					Title:      title,
					ShortTitle: shortTitle,
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
							keyOutputApprover: {
								Type:   "string",
								Global: example,
							},
						},
					},
					Params: func() []byte {
						r, _ := json.Marshal(&script.ApproverParams{
							Type:               script.ApproverTypeUser,
							Approver:           login,
							FormsAccessibility: make([]script.FormAccessibility, 0),
						})

						return r
					}(),
					Sockets: next,
				},
			},
			want: &GoApproverBlock{
				Name:      example,
				ShortName: shortTitle,
				Title:     title,
				Input: map[string]string{
					"foo": "bar",
				},
				Output: map[string]string{
					keyOutputApprover: example,
				},
				happenedEvents: make([]entity.NodeEvent, 0),
				State: &ApproverData{
					Type: script.ApproverTypeUser,
					Approvers: map[string]struct{}{
						login: {},
					},
					Decision:           nil,
					Comment:            nil,
					ActualApprover:     nil,
					AutoAction:         nil,
					ApprovementRule:    script.AnyOfApprovementRequired,
					ApproverLog:        make([]ApproverLogEntry, 0),
					FormsAccessibility: make([]script.FormAccessibility, 0),
					ActionList: []Action{
						{
							ID:    DefaultSocketID,
							Title: script.DefaultSocketTitle,
						},
						{
							ID:    rejectedSocketID,
							Title: script.RejectSocketTitle,
						},
					},
					WorkType: string(sla.WorkTypeN85),
				},
				Sockets: entity.ConvertSocket(next),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			got, _, err := createGoApproverBlock(ctx, tt.args.name, tt.args.ef, tt.args.runCtx, nil)
			if got != nil {
				got.RunContext = nil
			}

			assert.Equalf(t, tt.wantErr, err != nil, "createGoApproverBlock(%v, %v, %v)", tt.args.name, tt.args.ef, nil)
			assert.Equalf(t, tt.want, got, "createGoApproverBlock(%v, %v, %v)", tt.args.name, tt.args.ef, nil)
		})
	}
}

func TestGoApproverBlock_Update(t *testing.T) {
	stepID := uuid.New()
	exampleApprover := "example"
	secondExampleApprover := "example2"
	stepName := "appr"

	type fields struct {
		Name         string
		Title        string
		Input        map[string]string
		Output       map[string]string
		NextStep     []script.Socket
		RunContext   *BlockRunContext
		ApproverData *ApproverData
	}

	type args struct {
		ctx  context.Context
		data *script.BlockUpdateData
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		want    interface{}
		wantErr bool
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

							res.On("GetTaskStepById",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								stepID,
							).Return(
								nil, errors.New("unknown error"),
							)

							return res
						}(),
					},
				},
			},
			args: args{
				ctx:  context.Background(),
				data: nil,
			},
			wantErr: true,
		},
		{
			name: "any of approvers",
			fields: fields{
				Name: stepName,
				ApproverData: &ApproverData{
					Type: script.ApproverTypeUser,
					Approvers: map[string]struct{}{
						exampleApprover:       {},
						secondExampleApprover: {},
					},
					ApprovementRule: script.AnyOfApprovementRequired,
					ActionList: []Action{
						{
							ID: ApproverActionApprove,
						},
					},
				},
				RunContext: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						ServiceDesc: func() *servicedesc.Service {
							sdMock := servicedesc.Service{
								SdURL: "",
							}
							httpClient := http.DefaultClient
							mockTransport := serviceDeskMocks.RoundTripper{}
							fResponse := func(*http.Request) *http.Response {
								b, _ := json.Marshal(servicedesc.SsoPerson{})
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
							sdMock.Cli = httpClient

							return &sdMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("GetTaskStepById",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								stepID,
							).Return(
								&entity.Step{
									Time: time.Time{},
									Type: BlockGoApproverID,
									Name: stepName,
									State: map[string]json.RawMessage{
										stepName: func() []byte {
											r, _ := json.Marshal(&ApproverData{
												Type: script.ApproverTypeUser,
												Approvers: map[string]struct{}{
													exampleApprover:       {},
													secondExampleApprover: {},
												},
												ApprovementRule: script.AnyOfApprovementRequired,
											})

											return r
										}(),
									},
									Errors:      nil,
									Steps:       nil,
									BreakPoints: nil,
									HasError:    false,
									Status:      "",
								}, nil,
							)

							res.On("UpdateStepContext",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
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
				ctx: context.Background(),
				data: &script.BlockUpdateData{
					ByLogin: exampleApprover,
					Action:  string(entity.TaskUpdateActionApprovement),
					//nolint:goconst // не нужно здесь константы чекать
					Parameters: []byte(`{"decision":"` + ApproverActionApprove + `"}`),
				},
			},
			wantErr: false,
		},
		{
			name: "any of approvers",
			fields: fields{
				Name: stepName,
				ApproverData: &ApproverData{
					Type: script.ApproverTypeUser,
					Approvers: map[string]struct{}{
						exampleApprover:       {},
						secondExampleApprover: {},
					},
					ApprovementRule: script.AnyOfApprovementRequired,
					ActionList: []Action{
						{
							ID: ApproverActionApprove,
						},
					},
				},
				RunContext: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						ServiceDesc: func() *servicedesc.Service {
							sdMock := servicedesc.Service{
								SdURL: "",
							}
							httpClient := http.DefaultClient
							mockTransport := serviceDeskMocks.RoundTripper{}
							fResponse := func(*http.Request) *http.Response {
								b, _ := json.Marshal(servicedesc.SsoPerson{})
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
							sdMock.Cli = httpClient

							return &sdMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("GetTaskStepById",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								stepID,
							).Return(
								&entity.Step{
									Time: time.Time{},
									Type: BlockGoApproverID,
									Name: stepName,
									State: map[string]json.RawMessage{
										stepName: func() []byte {
											r, _ := json.Marshal(&ApproverData{
												Type: script.ApproverTypeUser,
												Approvers: map[string]struct{}{
													exampleApprover:       {},
													secondExampleApprover: {},
												},
												ApprovementRule: script.AnyOfApprovementRequired,
											})

											return r
										}(),
									},
									Errors:      nil,
									Steps:       nil,
									BreakPoints: nil,
									HasError:    false,
									Status:      "",
								}, nil,
							)

							res.On("UpdateStepContext",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
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
				ctx: context.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    exampleApprover,
					Action:     string(entity.TaskUpdateActionApprovement),
					Parameters: []byte(`{"decision":"` + ApproverActionApprove + `"}`),
				},
			},
			wantErr: false,
		},
		{
			name: "acceptance test",
			fields: fields{
				Name: stepName,
				ApproverData: &ApproverData{
					Type: script.ApproverTypeUser,
					Approvers: map[string]struct{}{
						exampleApprover: {},
					},
					ActionList: []Action{
						{
							ID: ApproverActionApprove,
						},
					},
					WorkType: "8/5",
				},
				RunContext: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						ServiceDesc: func() *servicedesc.Service {
							sdMock := servicedesc.Service{
								SdURL: "",
							}
							httpClient := http.DefaultClient
							mockTransport := serviceDeskMocks.RoundTripper{}
							fResponse := func(*http.Request) *http.Response {
								b, _ := json.Marshal(servicedesc.SsoPerson{})
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
							sdMock.Cli = httpClient

							return &sdMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("GetTaskStepById",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								stepID,
							).Return(
								&entity.Step{
									Time: time.Time{},
									Type: BlockGoApproverID,
									Name: stepName,
									State: map[string]json.RawMessage{
										stepName: func() []byte {
											r, _ := json.Marshal(&ApproverData{
												Type: script.ApproverTypeUser,
												Approvers: map[string]struct{}{
													exampleApprover: {},
												},
											})

											return r
										}(),
									},
									Errors:      nil,
									Steps:       nil,
									BreakPoints: nil,
									HasError:    false,
									Status:      "",
								}, nil,
							)

							res.On("UpdateStepContext",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
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
				ctx: context.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    exampleApprover,
					Action:     string(entity.TaskUpdateActionApprovement),
					Parameters: []byte(`{"decision":"` + ApproverActionApprove + `"}`),
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gb := &GoApproverBlock{
				Name:       tt.fields.Name,
				Title:      tt.fields.Title,
				Input:      tt.fields.Input,
				Output:     tt.fields.Output,
				Sockets:    tt.fields.NextStep,
				State:      tt.fields.ApproverData,
				RunContext: tt.fields.RunContext,
			}
			tt.fields.RunContext.UpdateData = tt.args.data
			_, err := gb.Update(tt.args.ctx)
			assert.Equalf(t, tt.wantErr, err != nil, fmt.Sprintf("Update(%v, %v)", tt.args.ctx, tt.args.data))
		})
	}
}

func TestGoApproverBlock_Actions(t *testing.T) {
	exampleApprover := "example"
	secondExampleApprover := "example2"
	stepName := "appr"
	login := "user1"
	delLogin1 := "delLogin1"

	type fields struct {
		Name         string
		Title        string
		Input        map[string]string
		Output       map[string]string
		NextStep     []script.Socket
		RunContext   *BlockRunContext
		ApproverData *ApproverData
	}

	type args struct {
		ctx  context.Context
		data *script.BlockUpdateData
	}

	tests := []struct {
		name        string
		fields      fields
		args        args
		want        interface{}
		wantActions []MemberAction
		wantErr     bool
	}{
		{
			name: "empty forms accessibility",
			fields: fields{
				Name: stepName,
				ApproverData: &ApproverData{
					Type: script.ApproverTypeUser,
					Approvers: map[string]struct{}{
						exampleApprover:       {},
						secondExampleApprover: {},
					},
					ApprovementRule: script.AnyOfApprovementRequired,
					ActionList: []Action{
						{
							ID: ApproverActionApprove,
						},
					},
				},
				RunContext: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
				},
			},
			args: args{
				ctx:  context.Background(),
				data: nil,
			},
			wantActions: []MemberAction{
				{ID: "approve", Type: "", Params: map[string]interface{}(nil)},
				{ID: "add_approvers", Type: "other", Params: map[string]interface{}(nil)},
				{ID: "request_add_info", Type: "other", Params: map[string]interface{}(nil)}},
		},
		{
			name: "one form (ReadWrite)",
			fields: fields{
				Name: stepName,
				ApproverData: &ApproverData{
					Type: script.ApproverTypeUser,
					Approvers: map[string]struct{}{
						exampleApprover:       {},
						secondExampleApprover: {},
					},
					FormsAccessibility: []script.FormAccessibility{
						{
							Name:        "Форма",
							NodeID:      "form_0",
							AccessType:  "ReadWrite",
							Description: "форма",
						},
					},
					ApprovementRule: script.AnyOfApprovementRequired,
					ActionList: []Action{
						{
							ID: ApproverActionApprove,
						},
					},
				},
				RunContext: &BlockRunContext{
					skipNotifications: true,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						s.State = map[string]json.RawMessage{
							"form_0": []byte{},
						}

						return s
					}(), Services: RunContextServices{},
				},
			},

			args: args{
				ctx: context.Background(),
			},
			wantActions: []MemberAction{
				{ID: "approve", Type: "", Params: map[string]interface{}(nil)},
				{ID: "fill_form", Type: "custom", Params: map[string]interface{}{"form_name": []string{"form_0"}}},
				{ID: "add_approvers", Type: "other", Params: map[string]interface{}(nil)},
				{ID: "request_add_info", Type: "other", Params: map[string]interface{}(nil)}},
		},
		{
			name: "Two forms ReadWrite",
			fields: fields{
				Name: stepName,
				ApproverData: &ApproverData{
					Type: script.ApproverTypeUser,
					Approvers: map[string]struct{}{
						exampleApprover:       {},
						secondExampleApprover: {},
					},
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
					ApprovementRule: script.AnyOfApprovementRequired,
					ActionList: []Action{
						{
							ID: ApproverActionApprove,
						},
					},
				},
				RunContext: &BlockRunContext{
					skipNotifications: true,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						s.State = map[string]json.RawMessage{
							"form_0": []byte{},
							"form_1": []byte{},
						}

						return s
					}(), Services: RunContextServices{},
				},
			},
			args: args{
				ctx: context.Background(),
				data: &script.BlockUpdateData{
					ByLogin: exampleApprover,
					Action:  string(entity.TaskUpdateActionApprovement),
					//nolint:goconst // не нужно здесь константы чекать
					Parameters: []byte(`{"decision":"` + ApproverActionApprove + `"}`),
				},
			},
			wantActions: []MemberAction{
				{ID: "approve", Type: "", Params: map[string]interface{}(nil)},
				{ID: "fill_form", Type: "custom", Params: map[string]interface{}{"form_name": []string{"form_0", "form_1"}}},
				{ID: "add_approvers", Type: "other", Params: map[string]interface{}(nil)},
				{ID: "request_add_info", Type: "other", Params: map[string]interface{}(nil)}},
		},
		{
			name: "Two forms - not exist ChangeLog (ReadWrite && RequiredFill)",
			fields: fields{
				Name: stepName,
				ApproverData: &ApproverData{
					Type: script.ApproverTypeUser,
					Approvers: map[string]struct{}{
						login:                 {},
						secondExampleApprover: {},
					},
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
					ApprovementRule: script.AnyOfApprovementRequired,
					ActionList: []Action{
						{
							ID: ApproverActionApprove,
						},
					},
				},
				RunContext: &BlockRunContext{
					skipNotifications: true,
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
							}()}
						return s
					}(), Services: RunContextServices{
						HumanTasks: func() *humanTasks.Service {
							ht := humanTasks.Service{}
							htMock := htMocks.DelegationServiceClient{}

							htMock.On("GetDelegationsFromLogin", context.Background(), "users1").Return(nil, humanTasks.Delegations{})

							req := &delegationht.GetDelegationsRequest{
								FilterBy:  "fromLogin",
								FromLogin: login,
							}

							htMock.On("getDelegationsInternal", context.Background(), req).Return(humanTasks.Delegations{
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
							htMock.On("GetDelegates", "users1").Return([]string{})

							ht = humanTasks.Service{
								Cli: &htMock,
								C:   nil,
							}

							return &ht
						}(),
					},
				},
			},

			args: args{
				ctx: context.Background(),
			},
			wantActions: []MemberAction{
				{ID: "approve", Type: "", Params: map[string]interface{}{"disabled": true}},
				{ID: "fill_form", Type: "custom", Params: map[string]interface{}{"form_name": []string{"form_0", "form_1"}}},
				{ID: "add_approvers", Type: "other", Params: map[string]interface{}(nil)},
				{ID: "request_add_info", Type: "other", Params: map[string]interface{}(nil)}},
		},
		{
			name: "Two forms - ok (ReadWrite && RequiredFill)",
			fields: fields{
				Name: stepName,
				ApproverData: &ApproverData{
					Type: script.ApproverTypeUser,
					Approvers: map[string]struct{}{
						login:                 {},
						secondExampleApprover: {},
					},
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
					ApprovementRule: script.AnyOfApprovementRequired,
					ActionList: []Action{
						{
							ID: ApproverActionApprove,
						},
					},
				},
				RunContext: &BlockRunContext{
					skipNotifications: true,
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
					}(), Services: RunContextServices{
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							return res
						}(),
						HumanTasks: func() *humanTasks.Service {
							ht := humanTasks.Service{}
							htMock := htMocks.DelegationServiceClient{}

							htMock.On("GetDelegationsFromLogin", context.Background(), "users1").Return(humanTasks.Delegations{}, nil)

							req := &delegationht.GetDelegationsRequest{
								FilterBy:  "fromLogin",
								FromLogin: login,
							}

							htMock.On("getDelegationsInternal", context.Background(), req).Return(humanTasks.Delegations{}, nil)
							htMock.On("FilterByType", "users1").Return(delegationht.GetDelegationsResponse{
								Delegations: []*delegationht.Delegation{},
							})
							htMock.On("GetDelegates", "users1").Return([]string{})

							ht = humanTasks.Service{
								Cli: &htMock,
								C:   nil,
							}

							return &ht
						}(),
					},
				},
			},

			args: args{
				ctx: context.Background(),
			},
			wantActions: []MemberAction{
				{ID: "approve", Type: "", Params: map[string]interface{}(nil)},
				{ID: "fill_form", Type: "custom", Params: map[string]interface{}{"form_name": []string{"form_0", "form_1"}}},
				{ID: "add_approvers", Type: "other", Params: map[string]interface{}(nil)},
				{ID: "request_add_info", Type: "other", Params: map[string]interface{}(nil)}},
		},
		{
			name: "Required Fill - false filled",
			fields: fields{
				Name: stepName,
				ApproverData: &ApproverData{
					Type: script.ApproverTypeUser,
					Approvers: map[string]struct{}{
						exampleApprover:       {},
						secondExampleApprover: {},
					},
					FormsAccessibility: []script.FormAccessibility{
						{
							Name:        "Форма",
							NodeID:      "form_0",
							AccessType:  "RequiredFill",
							Description: "форма",
						},
					},
					ApprovementRule: script.AnyOfApprovementRequired,
					ActionList: []Action{
						{
							ID: ApproverActionApprove,
						},
					},
				},
				RunContext: &BlockRunContext{
					skipNotifications: true,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						s.State = map[string]json.RawMessage{
							"form_0": func() []byte {
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
					}(), Services: RunContextServices{
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							return res
						}(),
						HumanTasks: func() *humanTasks.Service {
							ht := humanTasks.Service{}
							htMock := htMocks.DelegationServiceClient{}

							htMock.On("GetDelegationsFromLogin", context.Background(), "users1").Return(humanTasks.Delegations{}, nil)

							req := &delegationht.GetDelegationsRequest{
								FilterBy:  "fromLogin",
								FromLogin: login,
							}

							htMock.On("getDelegationsInternal", context.Background(), req).Return(humanTasks.Delegations{}, nil)
							htMock.On("FilterByType", "users1").Return(delegationht.GetDelegationsResponse{
								Delegations: []*delegationht.Delegation{},
							})
							htMock.On("GetDelegates", "users1").Return([]string{})

							ht = humanTasks.Service{
								Cli: &htMock,
								C:   nil,
							}

							return &ht
						}(),
					},
				},
			},

			args: args{
				ctx: context.Background(),
			},
			wantActions: []MemberAction{
				{ID: "approve", Type: "", Params: map[string]interface{}{"disabled": true}},
				{ID: "fill_form", Type: "custom", Params: map[string]interface{}{"form_name": []string{"form_0"}}},
				{ID: "add_approvers", Type: "other", Params: map[string]interface{}(nil)},
				{ID: "request_add_info", Type: "other", Params: map[string]interface{}(nil)}},
		},
		{
			name: "Two forms - ok (RequiredFill && RequiredFill)",
			fields: fields{
				Name: stepName,
				ApproverData: &ApproverData{
					Type: script.ApproverTypeUser,
					Approvers: map[string]struct{}{
						login:                 {},
						secondExampleApprover: {},
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
					ApprovementRule: script.AnyOfApprovementRequired,
					ActionList: []Action{
						{
							ID: ApproverActionApprove,
						},
					},
				},
				RunContext: &BlockRunContext{
					skipNotifications: true,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						s.State = map[string]json.RawMessage{
							"form_0": func() []byte {
								marshalForm, _ := json.Marshal(FormData{
									IsFilled: true,
									Executors: map[string]struct{}{
										"user1": {},
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
									IsFilled: true,
									Executors: map[string]struct{}{
										"user1": {},
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
					}(), Services: RunContextServices{
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							return res
						}(),
						HumanTasks: func() *humanTasks.Service {
							ht := humanTasks.Service{}
							htMock := htMocks.DelegationServiceClient{}

							htMock.On("GetDelegationsFromLogin", context.Background(), "users1").Return(humanTasks.Delegations{}, nil)

							req := &delegationht.GetDelegationsRequest{
								FilterBy:  "fromLogin",
								FromLogin: login,
								ToLogin:   delLogin1,
							}

							htMock.On("getDelegationsInternal", context.Background(), req).Return(humanTasks.Delegations{}, nil)
							htMock.On("FilterByType", "users1").Return(delegationht.GetDelegationsResponse{
								Delegations: []*delegationht.Delegation{},
							})

							delegates := []string{delLogin1}
							htMock.On("GetDelegates", "users1").Return(delegates)

							ht = humanTasks.Service{
								Cli: &htMock,
								C:   nil,
							}

							return &ht
						}(),
					},
				},
			},

			args: args{
				ctx: context.Background(),
			},
			wantActions: []MemberAction{
				{ID: "approve", Type: "", Params: map[string]interface{}(nil)},
				{ID: "fill_form", Type: "custom", Params: map[string]interface{}{"form_name": []string{"form_0", "form_1"}}},
				{ID: "add_approvers", Type: "other", Params: map[string]interface{}(nil)},
				{ID: "request_add_info", Type: "other", Params: map[string]interface{}(nil)}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gb := &GoApproverBlock{
				Name:       tt.fields.Name,
				Title:      tt.fields.Title,
				Input:      tt.fields.Input,
				Output:     tt.fields.Output,
				Sockets:    tt.fields.NextStep,
				State:      tt.fields.ApproverData,
				RunContext: tt.fields.RunContext,
			}
			tt.fields.RunContext.UpdateData = tt.args.data
			actions := gb.approvementBaseActions(login)

			assert.Equalf(t, tt.wantActions, actions, fmt.Sprintf("AddBaseAction(%v)", login))
		})
	}
}
