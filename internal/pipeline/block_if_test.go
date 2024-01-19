package pipeline

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	conditions_kit "gitlab.services.mts.ru/jocasta/conditions-kit"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

func TestIF_Next(t *testing.T) {
	type (
		fields struct {
			Name          string
			FunctionName  string
			FunctionInput map[string]string
			Result        bool
			Nexts         []script.Socket
			State         *ConditionsData
		}
		args struct {
			runCtx *store.VariableStore
		}
	)

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
				Nexts: []script.Socket{script.DefaultSocket},
				State: &ConditionsData{},
			},
			args: args{
				runCtx: func() *store.VariableStore {
					res := store.NewStore()
					res.SetValue("chosenGroup", "")
					return res
				}(),
			},
			ok:   true,
			want: []string(nil),
		},
		{
			name: "test chosen group",
			fields: fields{
				Nexts: []script.Socket{script.NewSocket("test-group-1", []string{"test-next"})},
				State: &ConditionsData{ChosenGroupID: "test-group-1"},
			},
			args: args{
				runCtx: func() *store.VariableStore {
					res := store.NewStore()
					res.SetValue("chosenGroup", "test-group-1")
					return res
				}(),
			},
			ok:   true,
			want: []string{"test-next"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &IF{
				Name:          tt.fields.Name,
				FunctionName:  tt.fields.FunctionName,
				FunctionInput: tt.fields.FunctionInput,
				Result:        tt.fields.Result,
				Sockets:       tt.fields.Nexts,
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

	type (
		TestOperand struct {
			OperandType string      `json:"operandType"`
			Value       interface{} `json:"value"`
			VariableRef string      `json:"variableRef"`
			conditions_kit.OperandBase
		}

		fields struct {
			Name          string
			FunctionName  string
			FunctionInput map[string]string
			Result        bool
			Nexts         map[string][]string
			State         *ConditionsData
		}
		args struct {
			name   string
			ef     *entity.EriusFunc
			ctx    context.Context
			runCtx *store.VariableStore
		}
	)

	tests := []struct {
		name          string
		fields        fields
		args          args
		wantErr       bool
		wantedGroupID string
	}{
		{
			name:          "empty groups",
			wantErr:       false,
			wantedGroupID: "",
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
			name:          "compare string values - not equal",
			wantErr:       false,
			wantedGroupID: "test-group-1",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "or",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandBase: conditions_kit.OperandBase{
													DataType: "string",
												},
												OperandType: "valueOperand",
												Value:       "test",
											},
											RightOperand: &TestOperand{
												OperandBase: conditions_kit.OperandBase{
													DataType: "string",
												},
												OperandType: "valueOperand",
												Value:       "test2",
											},
											Operator: "NotEqual",
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
			name:          "compare string variables - equal",
			wantErr:       false,
			wantedGroupID: "test-group-1",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "or",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandBase: conditions_kit.OperandBase{
													DataType: "string",
												},
												OperandType: "variableOperand",
												VariableRef: "data.testStringVariable1",
											},
											RightOperand: &TestOperand{
												OperandBase: conditions_kit.OperandBase{
													DataType: "string",
												},
												OperandType: "variableOperand",
												VariableRef: "data.testStringVariable2",
											},
											Operator: "Equal",
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
					res.SetValue("data.testStringVariable1", "test")
					res.SetValue("data.testStringVariable2", "test")

					return res
				}(),
			},
		},
		{
			name:          "compare string variables - not equal",
			wantErr:       false,
			wantedGroupID: "test-group-1",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id: "test-group-1",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "variableOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "string",
												},
												VariableRef: "data.testStringVariable1",
											},
											RightOperand: &TestOperand{
												OperandType: "variableOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "string",
												},
												VariableRef: "data.testStringVariable2",
											},
											Operator: "NotEqual",
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
					res.SetValue("data.testStringVariable1", "test")
					res.SetValue("data.testStringVariable2", "test1")

					return res
				}(),
			},
		},
		{
			name:          "compare string and bool variables - equal",
			wantErr:       false,
			wantedGroupID: "test-group-1",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id: "test-group-1",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "variableOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "string",
												},
												VariableRef: "testStringVariable",
											},
											RightOperand: &TestOperand{
												OperandType: "variableOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "boolean",
												},
												VariableRef: "testBoolVariable",
											},
											Operator: "Equal",
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
					res.SetValue("testStringVariable", "true")
					res.SetValue("testBoolVariable", true)

					return res
				}(),
			},
		},
		{
			name:          "compare string nested in 2nd level - equal",
			wantErr:       false,
			wantedGroupID: "test-group-1",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "or",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "variableOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "string",
												},
												VariableRef: "level1.level2",
											},
											RightOperand: &TestOperand{
												OperandType: "variableOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "string",
												},
												VariableRef: "data.test",
											},
											Operator: "Equal",
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

					res.SetValue("level1.level2", "test")
					res.SetValue("data.test", "test")

					return res
				}(),
			},
		},
		{
			name:          "compare string nested in 2nd level - not equal",
			wantErr:       false,
			wantedGroupID: "",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "or",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "variableOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "string",
												},
												VariableRef: "level1.level2",
											},
											RightOperand: &TestOperand{
												OperandType: "variableOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "string",
												},
												VariableRef: "test1",
											},
											Operator: "Equal",
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

					res.SetValue("level1.level2", "test")

					return res
				}(),
			},
		},
		{
			name:          "compare string nested in 3rd level - equal",
			wantErr:       false,
			wantedGroupID: "test-group-1",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "or",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "variableOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "string",
												},
												VariableRef: "level1.level2.level3",
											},
											RightOperand: &TestOperand{
												OperandType: "variableOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "string",
												},
												VariableRef: "test",
											},
											Operator: "Equal",
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

					level2 := map[string]interface{}{
						"level3": "test",
					}

					res.SetValue("level1.level2", level2)
					res.SetValue("test", "test")

					return res
				}(),
			},
		},
		{
			name:          "compare string nested in 4th level - equal",
			wantErr:       false,
			wantedGroupID: "test-group-1",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "or",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "variableOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "string",
												},
												VariableRef: "level1.level2.level3.level4",
											},
											RightOperand: &TestOperand{
												OperandType: "variableOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "string",
												},
												VariableRef: "test",
											},
											Operator: "Equal",
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

					level2 := map[string]interface{}{
						"level3": map[string]interface{}{
							"level4": "test",
						},
					}

					res.SetValue("level1.level2", level2)
					res.SetValue("test", "test")

					return res
				}(),
			},
		},
		{
			name:          "compare string nested in 3rd level - not equal",
			wantErr:       false,
			wantedGroupID: "",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "or",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "variableOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "string",
												},
												VariableRef: "level1.level2.level3",
											},
											RightOperand: &TestOperand{
												OperandType: "variableOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "string",
												},
												VariableRef: "level1.level2.level3,1",
											},
											Operator: "Equal",
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

					level2 := map[string]interface{}{
						"level3":   "test",
						"level3,1": "test1",
					}

					res.SetValue("level1.level2", level2)

					return res
				}(),
			},
		},
		{
			name:          "compare string value with variable",
			wantErr:       false,
			wantedGroupID: "test-group-1",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "or",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "variableOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "string",
												},
												VariableRef: "data.testStringVariable",
											},
											RightOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "string",
												},
												Value: "test",
											},
											Operator: "Equal",
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
					res.SetValue("data.testStringVariable", "test")

					return res
				}(),
			},
		},
		{
			name:          "second group conditions is valid",
			wantErr:       false,
			wantedGroupID: "test-group-2",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "and",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "string",
												},
												Value: "test",
											},
											RightOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "string",
												},
												Value: "test2",
											},
											Operator: "Equal",
										},
										{
											LeftOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "string",
												},
												Value: "test",
											},
											RightOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "string",
												},
												Value: "test2",
											},
											Operator: "NotEqual",
										},
									},
								},
								{
									Id:              "test-group-2",
									LogicalOperator: "or",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "string",
												},
												Value: "testAbc",
											},
											RightOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "string",
												},
												Value: "testAbc",
											},
											Operator: "Equal",
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
			name:          "compare with string and integer (integer-string pair)",
			wantErr:       false,
			wantedGroupID: "test-group-1",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "and",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "integer",
												},
												Value: 10,
											},
											RightOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "string",
												},
												Value: "10",
											},
											Operator: "Equal",
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
			name:          "compare with string and integer (string-integer pair)",
			wantErr:       false,
			wantedGroupID: "test-group-1",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "or",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "string",
												},
												Value: "10",
											},
											RightOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "integer",
												},
												Value: 10,
											},
											Operator: "Equal",
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
			name:          "compare with invalid string and integer (string-integer pair)",
			wantErr:       false,
			wantedGroupID: "",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "or",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "string",
												},
												Value: "unable to cast to integer string",
											},
											RightOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "integer",
												},
												Value: 10,
											},
											Operator: "Equal",
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
			name:          "compare with string and number (number-string pair)",
			wantErr:       false,
			wantedGroupID: "test-group-1",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "or",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "number",
												},
												Value: 10.05,
											},
											RightOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "string",
												},
												Value: "10.05",
											},
											Operator: "Equal",
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
			name:          "compare with empty string and number (number-nil string pair)",
			wantErr:       false,
			wantedGroupID: "",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "or",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "number",
												},
												Value: nil,
											},
											RightOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "number",
												},
												Value: "0",
											},
											Operator: "Equal",
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
			name:          "compare with string and number (string-number pair)",
			wantErr:       false,
			wantedGroupID: "test-group-1",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "or",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "string",
												},
												Value: "10.05",
											},
											RightOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "number",
												},
												Value: 10.05,
											},
											Operator: "Equal",
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
			name:          "compare with int and number (int-number pair)",
			wantErr:       false,
			wantedGroupID: "test-group-1",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "or",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "integer",
												},
												Value: 10,
											},
											RightOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "number",
												},
												Value: 10,
											},
											Operator: "Equal",
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
			name:          "compare with int and number (number-int pair)",
			wantErr:       false,
			wantedGroupID: "test-group-1",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "or",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "number",
												},
												Value: 10.05,
											},
											RightOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "integer",
												},
												Value: 10,
											},
											Operator: "Equal",
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
			name:          "compare with string and bool (string-bool pair)",
			wantErr:       false,
			wantedGroupID: "test-group-1",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "or",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "string",
												},
												Value: "false",
											},
											RightOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "boolean",
												},
												Value: false,
											},
											Operator: "Equal",
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
			name:          "compare with string and bool (bool-string pair)",
			wantErr:       false,
			wantedGroupID: "test-group-1",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "or",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "boolean",
												},
												Value: false,
											},
											RightOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "string",
												},
												Value: "false",
											},
											Operator: "Equal",
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
			name:          "compare with int and bool (int-bool pair)",
			wantErr:       false,
			wantedGroupID: "",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "or",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "integer",
												},
												Value: 1,
											},
											RightOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "boolean",
												},
												Value: true,
											},
											Operator: "Equal",
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
			name:          "compare with int and bool (bool-int pair)",
			wantErr:       false,
			wantedGroupID: "test-group-1",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "or",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "boolean",
												},
												Value: true,
											},
											RightOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "integer",
												},
												Value: 1,
											},
											Operator: "Equal",
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
			name:          "compare string variables - contain",
			wantErr:       false,
			wantedGroupID: "test-group-1",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "or",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "variableOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "string",
												},
												VariableRef: "data.testStringVariable1",
											},
											RightOperand: &TestOperand{
												OperandType: "variableOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "string",
												},
												VariableRef: "data.testStringVariable2",
											},
											Operator: "Contain",
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
					res.SetValue("data.testStringVariable1", "heretesthere")
					res.SetValue("data.testStringVariable2", "test")

					return res
				}(),
			},
		},
		{
			name:          "compare string variables - not contain",
			wantErr:       false,
			wantedGroupID: "test-group-1",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "or",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "variableOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "string",
												},
												VariableRef: "data.testStringVariable1",
											},
											RightOperand: &TestOperand{
												OperandType: "variableOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "string",
												},
												VariableRef: "data.testStringVariable2",
											},
											Operator: "NotContain",
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
					res.SetValue("data.testStringVariable1", "Nothing")
					res.SetValue("data.testStringVariable2", "test")

					return res
				}(),
			},
		},
		{
			name:          "compare int variables - less",
			wantErr:       false,
			wantedGroupID: "test-group-1",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "or",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "integer",
												},
												Value: 10,
											},
											RightOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "integer",
												},
												Value: 100,
											},
											Operator: "Less",
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
			name:          "compare int variables - lessOrEqual",
			wantErr:       false,
			wantedGroupID: "test-group-1",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "or",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "integer",
												},
												Value: 10,
											},
											RightOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "integer",
												},
												Value: 100,
											},
											Operator: "LessOrEqual",
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
			name:          "compare int variables - more",
			wantErr:       false,
			wantedGroupID: "test-group-1",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "or",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "integer",
												},
												Value: 100,
											},
											RightOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "integer",
												},
												Value: 10,
											},
											Operator: "More",
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
			name:          "compare int variables - moreOrEqal",
			wantErr:       false,
			wantedGroupID: "test-group-1",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "or",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "integer",
												},
												Value: 100,
											},
											RightOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "integer",
												},
												Value: 10,
											},
											Operator: "MoreOrEqual",
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
			name:          "compare number variables - less",
			wantErr:       false,
			wantedGroupID: "test-group-1",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "or",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "number",
												},
												Value: 10.05,
											},
											RightOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "number",
												},
												Value: 100,
											},
											Operator: "Less",
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
			name:          "compare number variables - lessOrEqual",
			wantErr:       false,
			wantedGroupID: "test-group-1",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "or",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "number",
												},
												Value: 10.05,
											},
											RightOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "number",
												},
												Value: 100,
											},
											Operator: "LessOrEqual",
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
			name:          "compare number variables - more",
			wantErr:       false,
			wantedGroupID: "test-group-1",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "or",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "number",
												},
												Value: 100,
											},
											RightOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "number",
												},
												Value: 10.05,
											},
											Operator: "More",
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
			name:          "compare number variables - moreOrEqal",
			wantErr:       false,
			wantedGroupID: "test-group-1",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "or",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "number",
												},
												Value: 100,
											},
											RightOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "number",
												},
												Value: 10.05,
											},
											Operator: "MoreOrEqual",
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
			name:          "compare date values - not equal",
			wantErr:       false,
			wantedGroupID: "test-group-1",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "or",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "date",
												},
												Value: "11.08.2022",
											},
											RightOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "date",
												},
												Value: "11.09.2022",
											},
											Operator: "NotEqual",
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
			name:          "compare date variables - equal",
			wantErr:       false,
			wantedGroupID: "test-group-1",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "or",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "variableOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "date",
												},
												VariableRef: "data.testStringVariable1",
											},
											RightOperand: &TestOperand{
												OperandType: "variableOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "date",
												},
												VariableRef: "data.testStringVariable2",
											},
											Operator: "Equal",
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
					res.SetValue("data.testStringVariable1", "11.08.2022")
					res.SetValue("data.testStringVariable2", "11.08.2022")

					return res
				}(),
			},
		},
		{
			name:          "compare date variables - less",
			wantErr:       false,
			wantedGroupID: "test-group-1",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "or",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "date",
												},
												Value: "11.08.2022",
											},
											RightOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "date",
												},
												Value: "11.08.2023",
											},
											Operator: "Less",
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
			name:          "compare date variables - lessOrEqual",
			wantErr:       false,
			wantedGroupID: "test-group-1",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "or",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "date",
												},
												Value: "11.08.2022",
											},
											RightOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "date",
												},
												Value: "11.08.2032",
											},
											Operator: "LessOrEqual",
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
			name:          "compare date variables - more",
			wantErr:       false,
			wantedGroupID: "test-group-1",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "or",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "date",
												},
												Value: "11.08.2022",
											},
											RightOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "date",
												},
												Value: "11.08.2012",
											},
											Operator: "More",
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
			name:          "compare date variables - moreOrEqal",
			wantErr:       false,
			wantedGroupID: "test-group-1",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "or",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "date",
												},
												Value: "11.08.2022",
											},
											RightOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "date",
												},
												Value: "11.08.2012",
											},
											Operator: "MoreOrEqual",
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
			name:          "compare with string and date (string-date pair)",
			wantErr:       false,
			wantedGroupID: "test-group-1",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "or",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "string",
												},
												Value: "11.08.2022",
											},
											RightOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "date",
												},
												Value: "11.08.2022",
											},
											Operator: "Equal",
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
			name:          "compare with string and date (date-string pair)",
			wantErr:       false,
			wantedGroupID: "test-group-1",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "or",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "date",
												},
												Value: "11.08.2022",
											},
											RightOperand: &TestOperand{
												OperandType: "valueOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "string",
												},
												Value: "11.08.2022",
											},
											Operator: "Equal",
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
			name:          "compare date variable and nil - equal",
			wantErr:       false,
			wantedGroupID: "",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "or",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "variableOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "date",
												},
												VariableRef: "data.testStringVariable1",
											},
											RightOperand: &TestOperand{
												OperandType: "variableOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "date",
												},
												VariableRef: "data.testStringVariable3",
											},
											Operator: "Equal",
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
					res.SetValue("data.testStringVariable1", "11.08.2022")
					res.SetValue("data.testStringVariable2", "11.08.2022")

					return res
				}(),
			},
		},
		{
			name:          "variable - exists",
			wantErr:       false,
			wantedGroupID: "test-group-1",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "or",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "variableOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "date",
												},
												VariableRef: "data.testStringVariable1",
											},
											RightOperand: &TestOperand{},
											Operator:     "Exists",
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
					res.SetValue("data.testStringVariable1", "11.08.2022")
					res.SetValue("data.testStringVariable2", "11.08.2022")

					return res
				}(),
			},
		},
		{
			name:          "variable - not exists",
			wantErr:       false,
			wantedGroupID: "test-group-1",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType: BlockGoIfID,
					Title:     title,
					Params: func() []byte {
						r, _ := json.Marshal(&conditions_kit.ConditionParams{
							Type: "conditions",
							ConditionGroups: []conditions_kit.ConditionGroup{
								{
									Id:              "test-group-1",
									LogicalOperator: "or",
									Conditions: []conditions_kit.Condition{
										{
											LeftOperand: &TestOperand{
												OperandType: "variableOperand",
												OperandBase: conditions_kit.OperandBase{
													DataType: "date",
												},
												VariableRef: "data.testStringVariable3",
											},
											RightOperand: &TestOperand{},
											Operator:     "NotExists",
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
					res.SetValue("data.testStringVariable1", "11.08.2022")
					res.SetValue("data.testStringVariable2", "11.08.2022")

					return res
				}(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			goBlock, _, err := createGoIfBlock(context.Background(), tt.args.name, tt.args.ef,
				&BlockRunContext{VarStore: tt.args.runCtx}, nil)

			if goBlock != nil {
				if _, err = goBlock.Update(tt.args.ctx); (err != nil) != tt.wantErr {
					t.Errorf("DebugRun() error = %v, wantErr %v", err, tt.wantErr)
				}

				if goBlock.State.ChosenGroupID != tt.wantedGroupID {
					t.Errorf("Unwanted group name. wantedGroupID = %v", tt.wantedGroupID)
				}
			} else {
				t.Errorf("GoIfBlock is nil, error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
