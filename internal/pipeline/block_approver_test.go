package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/pkg/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db/mocks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	humanTasks "gitlab.services.mts.ru/jocasta/pipeliner/internal/human-tasks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

func TestApproverData_SetDecision(t *testing.T) {
	const (
		login                   = "example"
		decision ApproverAction = ApproverActionReject
		comment                 = "blah blah blah"

		invalidLogin = "foobar"
	)

	type fields struct {
		Type           script.ApproverType
		Approvers      map[string]struct{}
		Decision       *ApproverAction
		Comment        *string
		ActualApprover *string
	}
	type args struct {
		login       string
		decision    ApproverAction
		comment     string
		delegations humanTasks.Delegations
	}
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

			if err := a.SetDecision(tt.args.login, tt.args.decision.ToDecision(), tt.args.comment, []string{}, tt.args.delegations); (err != nil) != tt.wantErr {
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
		example             = "example"
		title               = "title"
		login               = "login1"
		approversFromSchema = "a.var1;b.var2;var3"
		approverGroupId     = "uuid13456"
	)

	next := []entity.Socket{
		{
			Id:           DefaultSocketID,
			Title:        script.DefaultSocketTitle,
			NextBlockIds: []string{"next_0"},
		},
		{
			Id:           rejectedSocketID,
			Title:        script.RejectSocketTitle,
			NextBlockIds: []string{"next_1"},
		},
	}

	type args struct {
		name string
		ef   *entity.EriusFunc
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
				ef: &entity.EriusFunc{
					BlockType: BlockGoApproverID,
					Title:     title,
					Input:     nil,
					Output:    nil,
					Params:    nil,
					Sockets:   next,
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid approver parameters",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoApproverID,
					Title:     title,
					Input:     nil,
					Output:    nil,
					Params:    []byte("{}"),
					Sockets:   next,
				},
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid approvement rule for many approvers from schema",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoApproverID,
					Title:     title,
					Input:     nil,
					Output:    nil,
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
				ef: &entity.EriusFunc{
					BlockType: BlockGoApproverID,
					Title:     title,
					Input:     nil,
					Output:    nil,
					Params: func() []byte {
						r, _ := json.Marshal(&script.ApproverParams{
							Type:             script.ApproverTypeGroup,
							ApproversGroupID: approverGroupId,
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
			name: "acceptance test",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoApproverID,
					Title:     title,
					Input: []entity.EriusFunctionValue{
						{
							Name:   "foo",
							Type:   "string",
							Global: "bar",
						},
					},
					Output: []entity.EriusFunctionValue{
						{
							Name:   keyOutputApprover,
							Type:   "string",
							Global: example,
						},
					},
					Params: func() []byte {
						r, _ := json.Marshal(&script.ApproverParams{
							Type:               script.ApproverTypeUser,
							Approver:           login,
							SLA:                1,
							FormsAccessibility: make([]script.FormAccessibility, 0),
						})

						return r
					}(),
					Sockets: next,
				},
			},
			want: &GoApproverBlock{
				Name:  example,
				Title: title,
				Input: map[string]string{
					"foo": "bar",
				},
				Output: map[string]string{
					keyOutputApprover: example,
				},
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
					SLA:                1,
					FormsAccessibility: make([]script.FormAccessibility, 0),
					ActionList: []Action{
						{
							Id:    DefaultSocketID,
							Title: script.DefaultSocketTitle,
						},
						{
							Id:    rejectedSocketID,
							Title: script.RejectSocketTitle,
						},
					},
				},
				Sockets: entity.ConvertSocket(next),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			got, err := createGoApproverBlock(ctx, tt.args.name, tt.args.ef, &BlockRunContext{
				skipNotifications: true,
				VarStore:          store.NewStore(),
			})
			if got != nil {
				got.RunContext = nil
			}
			assert.Equalf(t, tt.wantErr, err != nil, "createGoApproverBlock(%v, %v, %v)", tt.args.name, tt.args.ef, nil)
			assert.Equalf(t, tt.want, got, "createGoApproverBlock(%v, %v, %v)", tt.args.name, tt.args.ef, nil)
		})
	}
}

func TestGoApproverBlock_Update(t *testing.T) {
	stepId := uuid.New()
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
					Storage: func() db.Database {
						res := &mocks.MockedDatabase{}

						res.On("GetTaskStepById",
							mock.MatchedBy(func(ctx context.Context) bool { return true }),
							stepId,
						).Return(
							nil, errors.New("unknown error"),
						)

						return res
					}(),
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
							Id: ApproverActionApprove,
						},
					},
				},
				RunContext: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
					Storage: func() db.Database {
						res := &mocks.MockedDatabase{}

						res.On("GetTaskStepById",
							mock.MatchedBy(func(ctx context.Context) bool { return true }),
							stepId,
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
							Id: ApproverActionApprove,
						},
					},
				},
				RunContext: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
					Storage: func() db.Database {
						res := &mocks.MockedDatabase{}

						res.On("GetTaskStepById",
							mock.MatchedBy(func(ctx context.Context) bool { return true }),
							stepId,
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
							Id: ApproverActionApprove,
						},
					},
				},
				RunContext: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
					Storage: func() db.Database {
						res := &mocks.MockedDatabase{}

						res.On("GetTaskStepById",
							mock.MatchedBy(func(ctx context.Context) bool { return true }),
							stepId,
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
