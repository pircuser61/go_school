package pipeline

import (
	"context"

	"github.com/google/uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db/mocks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"

	"testing"
)

func TestGoExecutionBlock_DebugRun(t *testing.T) {
	stepId := uuid.New()

	type fields struct {
		Name     string
		Title    string
		Input    map[string]string
		Output   map[string]string
		NextStep map[string][]string
		State    *ExecutionData
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
		wantErr     bool
		wantStorage *store.VariableStore
	}{
		{
			name: "can't get work id from variable store",
			fields: fields{
				Name:     stepName,
				Title:    "",
				Input:    nil,
				Output:   nil,
				NextStep: map[string][]string{},
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
				NextStep: map[string][]string{},
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
				NextStep: map[string][]string{},
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gb := &GoExecutionBlock{
				Name:    tt.fields.Name,
				Title:   tt.fields.Title,
				Input:   tt.fields.Input,
				Output:  tt.fields.Output,
				Nexts:   tt.fields.NextStep,
				Storage: tt.fields.Storage,
			}
			if err := gb.DebugRun(tt.args.ctx, tt.args.runCtx); (err != nil) != tt.wantErr {
				t.Errorf("execution.DebugRun() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantStorage != nil {
				assert.Equal(t, tt.wantStorage, tt.args.runCtx,
					"execution.DebugRun() storage = %v, want %v", tt.args.runCtx, tt.wantStorage)
			}
		})
	}
}
