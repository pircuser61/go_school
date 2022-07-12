package pipeline

import (
	"context"
	"testing"

	"encoding/json"

	"github.com/stretchr/testify/assert"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

func TestIF_Next(t *testing.T) {
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
			fields: fields{
				Nexts: map[string][]string{DefaultSocket: []string{""}},
			},
			args: args{
				runCtx: func() *store.VariableStore {
					res := store.NewStore()
					res.SetValue("chosenGroup", "")
					return res
				}(),
			},
			ok:   true,
			want: []string{""},
		},
		{
			name: "test chosen group",
			fields: fields{
				Nexts: map[string][]string{"test-group-1": []string{""}},
			},
			args: args{
				runCtx: func() *store.VariableStore {
					res := store.NewStore()
					res.SetValue("chosenGroup", "test-group-1")
					return res
				}(),
			},
			ok:   true,
			want: []string{""},
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

func TestIF_DebugRun(t *testing.T) {
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
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
				},
				ctx: context.Background(),
				runCtx: func() *store.VariableStore {
					res := store.NewStore()
					return res
				}(),
			},
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
									Conditions: []script.Condition{
										{
											LeftOperand: &script.VariableOperand{
												OperandBase: script.OperandBase{
													Type: "string",
												},
												VariableRef: "testStringVariable1",
											},
											RightOperand: &script.VariableOperand{
												OperandBase: script.OperandBase{
													Type: "string",
												},
												VariableRef: "testStringVariable2",
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
			name:    "compare string inside object variable",
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
									Name:            "test-group-1",
									LogicalOperator: "or",
									Conditions: []script.Condition{
										{
											LeftOperand: &script.VariableOperand{
												OperandBase: script.OperandBase{
													Type: "string",
												},
												VariableRef: "superMap.sub.sub",
											},
											RightOperand: &script.VariableOperand{
												OperandBase: script.OperandBase{
													Type: "string",
												},
												VariableRef: "nestedTest",
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

					var sub = make(map[string]interface{})
					var subSub = sub["sub"]

					subSub = "nestedTest"

					res.SetValue("superMap", subSub)

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
									Name:            "test-group-1",
									LogicalOperator: "or",
									Conditions: []script.Condition{
										{
											LeftOperand: &script.ValueOperand{
												OperandBase: script.OperandBase{
													Type: "string",
												},
												Value: "test",
											},
											RightOperand: &script.ValueOperand{
												OperandBase: script.OperandBase{
													Type: "string",
												},
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
			name:    "compare string value with variable",
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
									Name:            "test-group-1",
									LogicalOperator: "or",
									Conditions: []script.Condition{
										{
											LeftOperand: &script.VariableOperand{
												OperandBase: script.OperandBase{
													Type: "string",
												},
												VariableRef: "testStringVariable",
											},
											RightOperand: &script.ValueOperand{
												OperandBase: script.OperandBase{
													Type: "string",
												},
												Value: "test",
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
					res.SetValue("testStringVariable", "test")

					return res
				}(),
			},
		},
		{
			name:    "valid OR condition",
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
									Name:            "test-group-1",
									LogicalOperator: "or",
									Conditions: []script.Condition{
										{
											LeftOperand: &script.ValueOperand{
												OperandBase: script.OperandBase{
													Type: "string",
												},
												Value: "test",
											},
											RightOperand: &script.ValueOperand{
												OperandBase: script.OperandBase{
													Type: "string",
												},
												Value: "test2",
											},
											Operator: "equal",
										},
										{
											LeftOperand: &script.ValueOperand{
												OperandBase: script.OperandBase{
													Type: "string",
												},
												Value: "test",
											},
											RightOperand: &script.ValueOperand{
												OperandBase: script.OperandBase{
													Type: "string",
												},
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
			name:    "valid AND condition",
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
									Name:            "test-group-1",
									LogicalOperator: "and",
									Conditions: []script.Condition{
										{
											LeftOperand: &script.ValueOperand{
												OperandBase: script.OperandBase{
													Type: "string",
												},
												Value: "test",
											},
											RightOperand: &script.ValueOperand{
												OperandBase: script.OperandBase{
													Type: "string",
												},
												Value: "test",
											},
											Operator: "equal",
										},
										{
											LeftOperand: &script.ValueOperand{
												OperandBase: script.OperandBase{
													Type: "string",
												},
												Value: "test",
											},
											RightOperand: &script.ValueOperand{
												OperandBase: script.OperandBase{
													Type: "string",
												},
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
			name:    "second group conditions is valid",
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
									Name:            "test-group-1",
									LogicalOperator: "and",
									Conditions: []script.Condition{
										{
											LeftOperand: &script.ValueOperand{
												OperandBase: script.OperandBase{
													Type: "string",
												},
												Value: "test",
											},
											RightOperand: &script.ValueOperand{
												OperandBase: script.OperandBase{
													Type: "string",
												},
												Value: "test2",
											},
											Operator: "equal",
										},
										{
											LeftOperand: &script.ValueOperand{
												OperandBase: script.OperandBase{
													Type: "string",
												},
												Value: "test",
											},
											RightOperand: &script.ValueOperand{
												OperandBase: script.OperandBase{
													Type: "string",
												},
												Value: "test2",
											},
											Operator: "notEqual",
										},
									},
								},
								{
									Name:            "test-group-2",
									LogicalOperator: "or",
									Conditions: []script.Condition{
										{
											LeftOperand: &script.ValueOperand{
												OperandBase: script.OperandBase{
													Type: "string",
												},
												Value: "testAbc",
											},
											RightOperand: &script.ValueOperand{
												OperandBase: script.OperandBase{
													Type: "string",
												},
												Value: "testAbc",
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
					return res
				}(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			goBlock, err := createGoIfBlock(tt.args.name, tt.args.ef)

			if goBlock != nil {
				if err = goBlock.DebugRun(tt.args.ctx, nil, tt.args.runCtx); (err != nil) != tt.wantErr {
					t.Errorf("DebugRun() error = %v, wantErr %v", err, tt.wantErr)
				}
			} else {
				t.Errorf("GoIfBlock is nil, error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
