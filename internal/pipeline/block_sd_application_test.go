package pipeline

import (
	"context"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
	"testing"
)

const stepName = "servicedesk_application"

func TestGoSdBlock_DebugRun(t *testing.T) {
	type fields struct {
		Name    string
		Title   string
		Input   map[string]string
		Output  map[string]string
		Nexts   map[string][]string
		State   *ApplicationData
		Storage db.Database
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
			name: "can't find application data in context",
			fields: fields{
				Name:    stepName,
				Title:   "",
				Input:   nil,
				Output:  nil,
				Nexts:   map[string][]string{},
				Storage: nil,
			},
			args: args{
				ctx:    context.Background(),
				runCtx: store.NewStore(),
			},
			wantStorage: nil,
			wantErr:     true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gb := &GoSdApplicationBlock{
				Name:    tt.fields.Name,
				Title:   tt.fields.Title,
				Input:   tt.fields.Input,
				Output:  tt.fields.Output,
				Nexts:   tt.fields.Nexts,
				State:   tt.fields.State,
				Storage: tt.fields.Storage,
			}

			if err := gb.DebugRun(tt.args.ctx, tt.args.runCtx); (err != nil) != tt.wantErr {
				t.Errorf("DebugRun() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
