package pipeline

import (
	"context"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
	"testing"
)

func TestGoSdBlock_DebugRun(t *testing.T) {
	type fields struct {
		Name     string
		Title    string
		Input    map[string]string
		Output   map[string]string
		NextStep string
		State    *ApplicationData
		Storage  db.Database
	}
	type args struct {
		ctx    context.Context
		runCtx *store.VariableStore
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gb := &GoSdApplicationBlock{
				Name:     tt.fields.Name,
				Title:    tt.fields.Title,
				Input:    tt.fields.Input,
				Output:   tt.fields.Output,
				NextStep: tt.fields.NextStep,
				State:    tt.fields.State,
				Storage:  tt.fields.Storage,
			}
			if err := gb.DebugRun(tt.args.ctx, tt.args.runCtx); (err != nil) != tt.wantErr {
				t.Errorf("DebugRun() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}