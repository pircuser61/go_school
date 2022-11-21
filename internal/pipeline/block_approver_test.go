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
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

func TestApproverData_SetDecision(t *testing.T) {
	const (
		login    = "example"
		decision = ApproverDecisionRejected
		comment  = "blah blah blah"

		invalidLogin = "foobar"
	)

	type fields struct {
		Type           script.ApproverType
		Approvers      map[string]struct{}
		Decision       *ApproverDecision
		Comment        *string
		ActualApprover *string
	}
	type args struct {
		login    string
		decision ApproverDecision
		comment  string
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
				Type: script.ApproverTypeHead,
				Approvers: map[string]struct{}{
					login: {},
				},
				Decision:       nil,
				Comment:        nil,
				ActualApprover: nil,
			},
			args: args{
				login:    invalidLogin,
				decision: decision,
				comment:  comment,
			},
			wantErr: true,
		},
		{
			name: "decision already set",
			fields: fields{
				Type: script.ApproverTypeHead,
				Approvers: map[string]struct{}{
					login: {},
				},
				Decision: func() *ApproverDecision {
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
				login:    login,
				decision: decision,
				comment:  comment,
			},
			wantErr: true,
		},
		{
			name: "unknown decision",
			fields: fields{
				Type: script.ApproverTypeHead,
				Approvers: map[string]struct{}{
					login: {},
				},
				Decision:       nil,
				Comment:        nil,
				ActualApprover: nil,
			},
			args: args{
				login:    login,
				decision: ApproverDecision("unknown"),
				comment:  comment,
			},
			wantErr: true,
		},
		{
			name: "valid case",
			fields: fields{
				Type: script.ApproverTypeHead,
				Approvers: map[string]struct{}{
					login: {},
				},
				Decision:       nil,
				Comment:        nil,
				ActualApprover: nil,
			},
			args: args{
				login:    login,
				decision: decision,
				comment:  comment,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &ApproverData{
				Type:           tt.fields.Type,
				Approvers:      tt.fields.Approvers,
				Decision:       tt.fields.Decision,
				Comment:        tt.fields.Comment,
				ActualApprover: tt.fields.ActualApprover,
			}

			if err := a.SetDecision(tt.args.login, tt.args.decision, tt.args.comment, []string{}); (err != nil) != tt.wantErr {
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

func Test_createGoApproverBlock(t *testing.T) {
	const (
		example = "example"
		title   = "title"
		login   = "login1"
	)

	next := []entity.Socket{
		{
			Id:           DefaultSocketID,
			Title:        script.DefaultSocketTitle,
			NextBlockIds: []string{"next_0"},
		},
		{
			Id:           rejectedSocketID,
			Title:        script.RejectedSocketTitle,
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
				},
				Sockets: entity.ConvertSocket(next),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			got, err := createGoApproverBlock(ctx, tt.args.name, tt.args.ef, &BlockRunContext{VarStore: store.NewStore()})
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
					VarStore: store.NewStore(),
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
				},
				RunContext: &BlockRunContext{
					VarStore: store.NewStore(),
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
					Parameters: []byte(`{"decision":"` + ApproverDecisionApproved.String() + `"}`),
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
				},
				RunContext: &BlockRunContext{
					VarStore: store.NewStore(),
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
					Parameters: []byte(`{"decision":"` + ApproverDecisionApproved.String() + `"}`),
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
				},
				RunContext: &BlockRunContext{
					VarStore: store.NewStore(),
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
					Parameters: []byte(`{"decision":"` + ApproverDecisionApproved.String() + `"}`),
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
