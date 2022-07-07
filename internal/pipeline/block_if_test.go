package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

/*
func TestIF_DebugRun(t *testing.T) {
	const checkKey = "foo"

	type fields struct {
		Name          string
		FunctionName  string
		FunctionInput map[string]string
		Result        bool
		Nexts         map[string][]string
		State   	  *ConditionsData
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
				Nexts:         tt.fields.Nexts,
				State:		   tt.fields.State,
			}


			if err := e.DebugRun(tt.args.ctx, tt.args.runCtx); (err != nil) != tt.wantErr {
				t.Errorf("DebugRun() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}*/

/*func TestIF_Next(t *testing.T) {
	const checkKey = "foo"

	type fields struct {
		Name          string
		FunctionName  string
		FunctionInput map[string]string
		Result        bool
		Nexts         map[string][]string
		State   	  *ConditionsData
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
				Nexts: map[string][]string{trueSocket: []string{"onTrue"}},
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
				Nexts: map[string][]string{falseSocket: []string{"onFalse"}},
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
				Nexts:         tt.fields.Nexts,
				State:		   tt.fields.State,
			}
			got, _ := e.Next(tt.args.runCtx)
			assert.Equal(t, tt.want, got)
		})
	}
}*/

func TestIF_Next_2(t *testing.T) {
	const checkKey = "foo"

	type fields struct {
		Name          string
		FunctionName  string
		FunctionInput map[string]string
		Result        bool
		Nexts         map[string][]string
		State         *ConditionsData
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
			name: "default socket",
		},
		{
			name: "group socket next",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &IF{
				Name:          tt.fields.Name,
				FunctionName:  tt.fields.FunctionName,
				FunctionInput: tt.fields.FunctionInput,
				Result:        tt.fields.Result,
				Nexts:         tt.fields.Nexts,
				State:         tt.fields.State,
			}
			got, _ := e.Next(tt.args.runCtx)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIF_DebugRun_3(t *testing.T) {
	const (
		example = "example"
		title   = "title"
	)

	type fields struct {
		Name          string
		FunctionName  string
		FunctionInput map[string]string
		Result        bool
		Nexts         map[string][]string
		State         *ConditionsData
	}
	type args struct {
		name   string
		ef     *entity.EriusFunc
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
			name:    "empty groups",
			wantErr: false,
		},
		{
			name:    "compare string variables",
			wantErr: false,
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&script.ConditionParams{
							Type: "conditions",
							ConditionGroups: []script.ConditionGroup{
								{
									AnyOf: []script.Condition{
										{
											LeftOperand: script.Operand{
												Type:  "string",
												Value: "ref#testStringVariable1",
											},
											RightOperand: script.Operand{
												Type:  "string",
												Value: "ref#testStringVariable2",
											},
											Operator: "equal",
										},
									},
								},
							},
						})

						return r
					}(),
				},
				ctx: context.Background(),
				runCtx: func() *store.VariableStore {
					res := store.NewStore()
					res.SetValue("testStringVariable1", "test")
					res.SetValue("testStringVariable2", "test")

					return res
				}(),
			},
		},
		{
			name:    "compare int variables",
			wantErr: false,
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&script.ConditionParams{
							Type: "conditions",
							ConditionGroups: []script.ConditionGroup{
								{
									AnyOf: []script.Condition{
										{
											LeftOperand: script.Operand{
												Type:  "string",
												Value: "ref#testIntVariable1",
											},
											RightOperand: script.Operand{
												Type:  "string",
												Value: "ref#testIntVariable2",
											},
											Operator: "moreThan",
										},
									},
								},
							},
						})

						return r
					}(),
				},
				ctx: context.Background(),
				runCtx: func() *store.VariableStore {
					res := store.NewStore()
					res.SetValue("testIntVariable1", 10)
					res.SetValue("testIntVariable2", 5)

					return res
				}(),
			},
		},
		{
			name:    "compare string values",
			wantErr: false,
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&script.ConditionParams{
							Type: "conditions",
							ConditionGroups: []script.ConditionGroup{
								{
									AnyOf: []script.Condition{
										{
											LeftOperand: script.Operand{
												Type:  "string",
												Value: "test",
											},
											RightOperand: script.Operand{
												Type:  "string",
												Value: "test2",
											},
											Operator: "notEqual",
										},
									},
								},
							},
						})

						return r
					}(),
				},
				ctx: context.Background(),
				runCtx: func() *store.VariableStore {
					res := store.NewStore()
					return res
				}(),
			},
		},
		{
			name:    "compare int values",
			wantErr: false,
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&script.ConditionParams{
							Type: "conditions",
							ConditionGroups: []script.ConditionGroup{
								{
									AnyOf: []script.Condition{
										{
											LeftOperand: script.Operand{
												Type:  "integer",
												Value: 10,
											},
											RightOperand: script.Operand{
												Type:  "integer",
												Value: 15,
											},
											Operator: "lessThan",
										},
									},
								},
							},
						})

						return r
					}(),
				},
				ctx: context.Background(),
				runCtx: func() *store.VariableStore {
					res := store.NewStore()
					return res
				}(),
			},
		},
		{
			name:    "choose anyOf group (group-0)",
			wantErr: false,
		},
		{
			name:    "choose allOf group (group-1)",
			wantErr: false,
		},
		{
			name:    "acceptance test",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := createGoIfBlock(tt.args.name, tt.args.ef)
			fmt.Println(got)
			fmt.Println(err)

			if err := got.DebugRun(tt.args.ctx, tt.args.runCtx); (err != nil) != tt.wantErr {
				t.Errorf("DebugRun() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
