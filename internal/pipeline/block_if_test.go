package pipeline

import (
	"context"
	"github.com/stretchr/testify/assert"
	"testing"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

func TestIF_DebugRun(t *testing.T) {
	const checkKey = "foo"

	type fields struct {
		Name          string
		FunctionName  string
		FunctionInput map[string]string
		Result        bool
		OnTrue        string
		OnFalse       string
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
		{
			name:   "error - no such key",
			fields: fields{},
			args: args{
				ctx:    context.Background(),
				runCtx: store.NewStore(),
			},
			wantErr: true,
		},
		{
			name: "error - value not a bool",
			fields: fields{
				FunctionInput: map[string]string{
					keyIf: checkKey,
				},
			},
			args: args{
				ctx: context.Background(),
				runCtx: func() *store.VariableStore {
					res := store.NewStore()
					res.SetValue(checkKey, "bar")

					return res
				}(),
			},
			wantErr: true,
		},
		{
			name: "acceptance test",
			fields: fields{
				FunctionInput: map[string]string{
					keyIf: checkKey,
				},
			},
			args: args{
				ctx: context.Background(),
				runCtx: func() *store.VariableStore {
					res := store.NewStore()
					res.SetValue(checkKey, true)

					return res
				}(),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &IF{
				Name:          tt.fields.Name,
				FunctionName:  tt.fields.FunctionName,
				FunctionInput: tt.fields.FunctionInput,
				Result:        tt.fields.Result,
				OnTrue:        tt.fields.OnTrue,
				OnFalse:       tt.fields.OnFalse,
			}

			if err := e.DebugRun(tt.args.ctx, tt.args.runCtx); (err != nil) != tt.wantErr {
				t.Errorf("DebugRun() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIF_Next(t *testing.T) {
	const checkKey = "foo"

	type fields struct {
		Name          string
		FunctionName  string
		FunctionInput map[string]string
		Result        bool
		OnTrue        string
		OnFalse       string
	}
	type args struct {
		runCtx *store.VariableStore
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []string
		ok     bool
	}{
		{
			name:   "error - no such key",
			fields: fields{},
			args: args{
				runCtx: store.NewStore(),
			},
			ok:   false,
			want: []string{},
		},
		{
			want: []string{},
			name: "error - value not a bool",
			fields: fields{
				FunctionInput: map[string]string{
					keyIf: checkKey,
				},
			},
			args: args{
				runCtx: func() *store.VariableStore {
					res := store.NewStore()
					res.SetValue(checkKey, "bar")

					return res
				}(),
			},
			ok: false,
		},
		{
			name: "onTrue",
			fields: fields{
				FunctionInput: map[string]string{
					keyIf: checkKey,
				},
				OnTrue: "onTrue",
			},
			args: args{
				runCtx: func() *store.VariableStore {
					res := store.NewStore()
					res.SetValue(checkKey, true)

					return res
				}(),
			},
			want: []string{"onTrue"},
			ok:   true,
		},
		{
			name: "onFalse",
			fields: fields{
				FunctionInput: map[string]string{
					keyIf: checkKey,
				},
				OnFalse: "onFalse",
			},
			args: args{
				runCtx: func() *store.VariableStore {
					res := store.NewStore()
					res.SetValue(checkKey, false)

					return res
				}(),
			},
			want: []string{"onFalse"},
			ok:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &IF{
				Name:          tt.fields.Name,
				FunctionName:  tt.fields.FunctionName,
				FunctionInput: tt.fields.FunctionInput,
				Result:        tt.fields.Result,
				OnTrue:        tt.fields.OnTrue,
				OnFalse:       tt.fields.OnFalse,
			}
			got, _ := e.Next(tt.args.runCtx)
			assert.Equal(t, tt.want, got)
		})
	}
}
