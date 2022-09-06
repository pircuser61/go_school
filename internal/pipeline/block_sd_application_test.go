package pipeline

import (
	"context"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
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
		Sockets []script.Socket
		State   *ApplicationData
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
				Sockets: []script.Socket{},
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
				Sockets: tt.fields.Sockets,
				State:   tt.fields.State,
			}

			if err := gb.DebugRun(tt.args.ctx, nil, tt.args.runCtx); (err != nil) != tt.wantErr {
				t.Errorf("DebugRun() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
