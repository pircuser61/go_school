package pipeline

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db/mocks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

func TestGoApproverBlock_DebugRun(t *testing.T) {
	const (
		stepName    = "approver1"
		decisionKey = "decision1"
	)

	var (
		login    = "example"
		decision = ApproverDecisionApproved
		comment  = "blah blah blah"
	)

	stepId := uuid.New()

	type fields struct {
		Name     string
		Title    string
		Input    map[string]string
		Output   map[string]string
		NextStep string
		Storage  db.Database
	}
	type args struct {
		ctx    context.Context
		runCtx *store.VariableStore
	}
	tests := []struct {
		name        string
		fields      fields
		args        args
		wantStorage *store.VariableStore
		wantErr     bool
	}{
		{
			name: "can't get work id from variable store",
			fields: fields{
				Name:     stepName,
				Title:    "",
				Input:    nil,
				Output:   nil,
				NextStep: "",
				Storage:  nil,
			},
			args: args{
				ctx:    context.Background(),
				runCtx: store.NewStore(),
			},
			wantStorage: nil,
			wantErr:     true,
		},
		{
			name: "can't assert type of work id",
			fields: fields{
				Name:     stepName,
				Title:    "",
				Input:    nil,
				Output:   nil,
				NextStep: "",
				Storage:  nil,
			},
			args: args{
				ctx: context.Background(),
				runCtx: func() *store.VariableStore {
					res := store.NewStore()

					res.SetValue(getWorkIdKey(stepName), "foo")

					return res
				}(),
			},
			wantStorage: nil,
			wantErr:     true,
		},
		{
			name: "unknown error from database",
			fields: fields{
				Name:     stepName,
				Title:    "",
				Input:    nil,
				Output:   nil,
				NextStep: "",
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
			args: args{
				ctx: context.Background(),
				runCtx: func() *store.VariableStore {
					res := store.NewStore()

					res.SetValue(getWorkIdKey(stepName), stepId)

					return res
				}(),
			},
			wantErr: true,
		},
		{
			name: "invalid format of go-approver-block state",
			fields: fields{
				Name:     stepName,
				Title:    "",
				Input:    nil,
				Output:   nil,
				NextStep: "",
				Storage: func() db.Database {
					res := &mocks.MockedDatabase{}

					res.On("GetTaskStepById",
						mock.MatchedBy(func(ctx context.Context) bool { return true }),
						stepId,
					).Return(
						&entity.Step{
							Time: time.Time{},
							Type: BlockGoApprover,
							Name: stepName,
							Storage: map[string]interface{}{
								stepName: "invalid",
							},
							Errors:      nil,
							Steps:       nil,
							BreakPoints: nil,
							HasError:    false,
							IsFinished:  false,
						}, nil,
					)

					return res
				}(),
			},
			args: args{
				ctx: context.Background(),
				runCtx: func() *store.VariableStore {
					res := store.NewStore()

					res.SetValue(getWorkIdKey(stepName), stepId)

					return res
				}(),
			},
			wantErr: true,
		},
		{
			name: "context canceled",
			fields: fields{
				Name:     stepName,
				Title:    "",
				Input:    nil,
				Output:   nil,
				NextStep: "",
				Storage: func() db.Database {
					res := &mocks.MockedDatabase{}

					res.On("GetTaskStepById",
						mock.MatchedBy(func(ctx context.Context) bool { return true }),
						stepId,
					).Return(
						nil, nil,
					)

					return res
				}(),
			},
			args: args{
				ctx: func() context.Context {
					ctx, cancel := context.WithCancel(context.Background())
					cancel()
					return ctx
				}(),
				runCtx: func() *store.VariableStore {
					res := store.NewStore()

					res.SetValue(getWorkIdKey(stepName), stepId)

					return res
				}(),
			},
			wantErr: true,
		},
		{
			name: "approved case",
			fields: fields{
				Name:  stepName,
				Title: "",
				Input: nil,
				Output: map[string]string{
					keyApproverDecision: decisionKey,
				},
				NextStep: "",
				Storage: func() db.Database {
					res := &mocks.MockedDatabase{}

					res.On("GetTaskStepById",
						mock.MatchedBy(func(ctx context.Context) bool { return true }),
						stepId,
					).Return(
						&entity.Step{
							Time: time.Time{},
							Type: BlockGoApprover,
							Name: stepName,
							Storage: map[string]interface{}{
								stepName: &ApproverData{
									Type: ApproverTypeUser,
									Approvers: map[string]struct{}{
										login: {},
									},
									Decision:       &decision,
									Comment:        &comment,
									ActualApprover: &login,
								},
							},
							Errors:      nil,
							Steps:       nil,
							BreakPoints: nil,
							HasError:    false,
							IsFinished:  false,
						}, nil,
					)

					return res
				}(),
			},
			args: args{
				ctx: context.Background(),
				runCtx: func() *store.VariableStore {
					res := store.NewStore()

					res.SetValue(getWorkIdKey(stepName), stepId)

					return res
				}(),
			},
			wantStorage: func() *store.VariableStore {
				res := store.NewStore()

				res.AddStep(stepName)

				res.SetValue(getWorkIdKey(stepName), stepId)
				res.SetValue(decisionKey, &ApproverResult{
					Login:    login,
					Decision: decision,
					Comment:  comment,
				})

				return res
			}(),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gb := &GoApproverBlock{
				Name:     tt.fields.Name,
				Title:    tt.fields.Title,
				Input:    tt.fields.Input,
				Output:   tt.fields.Output,
				NextStep: tt.fields.NextStep,
				Storage:  tt.fields.Storage,
			}
			if err := gb.DebugRun(tt.args.ctx, tt.args.runCtx); (err != nil) != tt.wantErr {
				t.Errorf("DebugRun() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantStorage != nil {
				assert.Equal(t, tt.wantStorage, tt.args.runCtx, "DebugRun() storage = %v, want %v", tt.args.runCtx, tt.wantStorage)
			}
		})
	}
}

func TestApproverData_SetDecision(t *testing.T) {
	const (
		login    = "example"
		decision = ApproverDecisionRejected
		comment  = "blah blah blah"

		invalidLogin = "foobar"
	)

	type fields struct {
		Type           ApproverType
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
				Type: ApproverTypeHead,
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
				Type: ApproverTypeHead,
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
				Type: ApproverTypeHead,
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
				Type: ApproverTypeHead,
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

			if err := a.SetDecision(tt.args.login, tt.args.decision, tt.args.comment); (err != nil) != tt.wantErr {
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

func TestApproverParams_Validate(t *testing.T) {
	type fields struct {
		Type      ApproverType
		Approvers []string
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "approver list is empty",
			fields: fields{
				Type:      ApproverTypeUser,
				Approvers: make([]string, 0),
			},
			wantErr: true,
		},
		{
			name: "unknown approver type",
			fields: fields{
				Type:      ApproverType("unknown"),
				Approvers: make([]string, 1),
			},
			wantErr: true,
		},
		{
			name: "acceptance test",
			fields: fields{
				Type:      ApproverTypeUser,
				Approvers: []string{"example"},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &ApproverParams{
				Type:      tt.fields.Type,
				Approvers: tt.fields.Approvers,
			}
			if err := a.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("%v Validate()", a)
			}
		})
	}
}

func Test_createGoApproverBlock(t *testing.T) {
	const (
		example = "example"
		title   = "title"
		next    = "next"
		login   = "login1"
	)

	type args struct {
		name    string
		ef      *entity.EriusFunc
		storage db.Database
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
					BlockType: BlockGoApprover,
					Title:     title,
					Input:     nil,
					Output:    nil,
					Params:    nil,
					Next:      next,
				},
				storage: nil,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "invalid approver parameters",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoApprover,
					Title:     title,
					Input:     nil,
					Output:    nil,
					Params:    &ApproverParams{},
					Next:      next,
				},
				storage: nil,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "acceptance test",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoApprover,
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
							Name:   keyApproverDecision,
							Type:   "string",
							Global: example,
						},
					},
					Params: &ApproverParams{
						Type:      ApproverTypeUser,
						Approvers: []string{login},
					},
					Next: next,
				},
				storage: nil,
			},
			want: &GoApproverBlock{
				Name:  example,
				Title: title,
				Input: map[string]string{
					"foo": "bar",
				},
				Output: map[string]string{
					keyApproverDecision: example,
				},
				NextStep: next,
				Storage:  nil,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := createGoApproverBlock(tt.args.name, tt.args.ef, tt.args.storage)
			assert.Equalf(t, tt.wantErr, err != nil, "createGoApproverBlock(%v, %v, %v)", tt.args.name, tt.args.ef, tt.args.storage)
			assert.Equalf(t, tt.want, got, "createGoApproverBlock(%v, %v, %v)", tt.args.name, tt.args.ef, tt.args.storage)
		})
	}
}
