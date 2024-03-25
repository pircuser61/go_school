package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db/mocks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

func TestBlockFunction_Update(t *testing.T) {
	var (
		workID   = uuid.New()
		stepName = "test_step"
	)

	type fields struct {
		Name    string
		Title   string
		Input   map[string]string
		Output  map[string]string
		Sockets []script.Socket
		State   *ExecutableFunction
		RunURL  string

		RunContext *BlockRunContext
	}

	type args struct {
		ctx  context.Context
		data *script.BlockUpdateData
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "test without update data",
			fields: fields{
				Name: stepName,
				State: &ExecutableFunction{
					Mapping: script.JSONSchemaProperties{
						"mktu": script.JSONSchemaPropertiesValue{
							Type:  "array",
							Value: "servicedesk_application_0.application_body.klassi_mktu",
						},
						"name": script.JSONSchemaPropertiesValue{
							Type:  "string",
							Value: "servicedesk_application_0.application_body.neiming",
						},
						"username": script.JSONSchemaPropertiesValue{
							Type:  "string",
							Value: "servicedesk_application_0.application_body.recipient.fullname",
						},
					},
					Function: script.FunctionParam{
						Input: `{
								"mktu":{"type":"array", "items":{"type":"string"},"title":"mktu"},
								"name":{"type":"string","title":"имя"},
								"username":{"type":"string","title": "username"}
							}`,
						RequiredInput: []string{"name", "username", "mktu"},
					},
				},
				RunContext: &BlockRunContext{
					TaskID:            workID,
					skipNotifications: true,
					skipProduce:       true,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						s.SetValue("servicedesk_application_0.application_body", map[string]interface{}{
							"klassi_mktu": []string{"1", "2", "3"},
							"neiming":     "test name",
							"recipient": map[string]interface{}{
								"fullname": "Egor Jopov",
							},
						})

						return s
					}(),
					Services: RunContextServices{
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("GetTaskStepByName",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								workID,
								stepName,
							).Return(
								&entity.Step{
									ID: uuid.New(),
								}, nil,
							)

							return res
						}(),
					},
				},
			},
			args: args{
				ctx:  context.Background(),
				data: nil,
			},
			wantErr: false,
		},
		{
			name: "test with required input and required",
			fields: fields{
				Name: stepName,
				State: &ExecutableFunction{
					Mapping: script.JSONSchemaProperties{
						"param1": script.JSONSchemaPropertiesValue{
							Type:        "string",
							Description: "param1",
							Value:       "servicedesk_application_0.application_body.params1",
						},
						"param2": script.JSONSchemaPropertiesValue{
							Type:        "boolean",
							Description: "param2",
							Value:       "servicedesk_application_0.application_body.params2",
						},
						"param3": script.JSONSchemaPropertiesValue{
							Type:        "number",
							Description: "param3",
							Value:       "servicedesk_application_0.application_body.params3",
						},
						"param4": script.JSONSchemaPropertiesValue{
							Type:        "object",
							Description: "param4",
							Value:       "servicedesk_application_0.application_body.params4",
						},
					},
					Function: script.FunctionParam{
						Input: `{
								"param1":{"type":"string", "title":"param1"},
								"param2":{"type":"boolean","title":"param2"},
								"param3":{"type":"number","title": "param3"},
								"param4":{"type":"object",
									"properties":{
										"param4.1":{"description":"param4.1","type":"string"},
										"param4.2":{"description":"param4.2","type":"string"}
									},
									"required":["param4.1"]
								}
							}`,
						RequiredInput: []string{"param1", "param2", "param3", "param4"},
					},
				},
				RunContext: &BlockRunContext{
					TaskID:            workID,
					skipNotifications: true,
					skipProduce:       true,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						s.SetValue("servicedesk_application_0.application_body", map[string]interface{}{
							"params1": "param-1",
							"params2": false,
							"params3": 3,
							"params4": map[string]interface{}{
								"param4.1": "param4.1",
								"param4.2": "param4.2",
							},
						})

						return s
					}(),
					Services: RunContextServices{
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("GetTaskStepByName",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								workID,
								stepName,
							).Return(
								&entity.Step{
									ID: uuid.New(),
								}, nil,
							)

							return res
						}(),
					},
				},
			},
			args: args{
				ctx:  context.Background(),
				data: nil,
			},
			wantErr: false,
		},
		{
			name: "bad test with required input and required",
			fields: fields{
				Name: stepName,
				State: &ExecutableFunction{
					Mapping: script.JSONSchemaProperties{
						"param1": script.JSONSchemaPropertiesValue{
							Type:        "string",
							Description: "param1",
							Value:       "servicedesk_application_0.application_body.params1",
						},
						"param3": script.JSONSchemaPropertiesValue{
							Type:        "number",
							Description: "param3",
							Value:       "servicedesk_application_0.application_body.params3",
						},
						"param4": script.JSONSchemaPropertiesValue{
							Type:        "object",
							Description: "param4",
							Value:       "servicedesk_application_0.application_body.params4",
						},
					},
					Function: script.FunctionParam{
						Input: `{
								"param1":{"type":"string", "title":"param1"},
								"param3":{"type":"number","title": "param3"},
								"param4":{"type":"object",
									"properties":{
										"param4.2":{"description":"param4.2","type":"string"}
									},
									"required":["param4.1"]
								}
							}`,
						RequiredInput: []string{"param1", "param2", "param3", "param4"},
					},
				},
				RunContext: &BlockRunContext{
					TaskID:            workID,
					skipNotifications: true,
					skipProduce:       true,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						s.SetValue("servicedesk_application_0.application_body", map[string]interface{}{
							"params1": "param-1",
							"params3": 3,
							"params4": map[string]interface{}{
								"param4.1": "param4.1",
								"param4.2": "param4.2",
							},
						})

						return s
					}(),
					Services: RunContextServices{
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("GetTaskStepByName",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								workID,
								stepName,
							).Return(
								&entity.Step{
									ID: uuid.New(),
								}, nil,
							)

							return res
						}(),
					},
				},
			},
			args: args{
				ctx:  context.Background(),
				data: nil,
			},
			wantErr: true,
		},
		{
			name: "ok test with required input and required",
			fields: fields{
				Name: stepName,
				State: &ExecutableFunction{
					Mapping: script.JSONSchemaProperties{
						"param1": script.JSONSchemaPropertiesValue{
							Type:        "string",
							Description: "param1",
							Value:       "servicedesk_application_0.application_body.params1",
						},
						"param2": script.JSONSchemaPropertiesValue{
							Type:        "boolean",
							Description: "param2",
							Value:       "servicedesk_application_0.application_body.params2",
						},
						"param3": script.JSONSchemaPropertiesValue{
							Type:        "number",
							Description: "param3",
							Value:       "servicedesk_application_0.application_body.params3",
						},
						"param4": script.JSONSchemaPropertiesValue{
							Type:        "object",
							Description: "param4",
							Value:       "servicedesk_application_0.application_body.params4",
						},
					},
					Function: script.FunctionParam{
						Input: `{
								"param1":{"type":"string", "title":"param1"},
								"param2":{"type":"boolean","title":"param2"},
								"param3":{"type":"number","title": "param3"},
								"param4":{"type":"object",
									"properties":{
										"param4.1":{"description":"param4.1","type":"string"},
										"param4.2":{"description":"param4.2","type":"string"}
									},
									"required":["param4.1"]
								}
							}`,
						RequiredInput: []string{"param1", "param2", "param4"},
					},
				},
				RunContext: &BlockRunContext{
					TaskID:            workID,
					skipNotifications: true,
					skipProduce:       true,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						s.SetValue("servicedesk_application_0.application_body", map[string]interface{}{
							"params1": "param-1",
							"params2": false,
							"params3": 3,
							"params4": map[string]interface{}{
								"param4.1": "param4.1",
								"param4.2": "param4.2",
							},
						})

						return s
					}(),
					Services: RunContextServices{
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("GetTaskStepByName",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								workID,
								stepName,
							).Return(
								&entity.Step{
									ID: uuid.New(),
								}, nil,
							)

							return res
						}(),
					},
				},
			},
			args: args{
				ctx:  context.Background(),
				data: nil,
			},
			wantErr: false,
		},

		{
			name: "test without update data type error",
			fields: fields{
				Name: stepName,
				State: &ExecutableFunction{
					Mapping: script.JSONSchemaProperties{
						"mktu": script.JSONSchemaPropertiesValue{
							Type:  "array",
							Value: "servicedesk_application_0.application_body.klassi_mktu",
						},
						"name": script.JSONSchemaPropertiesValue{
							Type:  "number",
							Value: "servicedesk_application_0.application_body.neiming",
						},
						"username": script.JSONSchemaPropertiesValue{
							Type:  "string",
							Value: "servicedesk_application_0.application_body.recipient.fullname",
						},
					},
				},
				RunContext: &BlockRunContext{
					TaskID:            workID,
					skipNotifications: true,
					skipProduce:       true,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						s.SetValue("servicedesk_application_0.application_body", map[string]interface{}{
							"klassi_mktu": []string{"1", "2", "3"},
							"neiming":     "test name",
							"recipient": map[string]interface{}{
								"fullname": "Egor Jopov",
							},
						})

						return s
					}(),
					Services: RunContextServices{
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("GetTaskStepByName",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								workID,
								stepName,
							).Return(
								&entity.Step{
									ID: uuid.New(),
								}, nil,
							)

							return res
						}(),
					},
				},
			},
			args: args{
				ctx:  context.Background(),
				data: nil,
			},
			wantErr: true,
		},
		{
			name: "test with update data",
			fields: fields{
				RunContext: &BlockRunContext{
					TaskID:            workID,
					skipNotifications: true,
					skipProduce:       true,
					VarStore:          store.NewStore(),
				},
				State: &ExecutableFunction{
					HasResponse: true,
					Function: script.FunctionParam{
						Output: `{"mktu": {"type": "array", "items": {"type": "number"}}, "name": {"type": "string"}, "obj": {"type": "object", "properties": {"k1": {"type": "string"}}}}`,
					},
				},
			},
			args: args{
				data: &script.BlockUpdateData{
					ByLogin: "example",
					Action:  string(entity.TaskUpdateActionExecution),
					Parameters: json.RawMessage(`{
						"mapping": {
							"mktu": [1, 2, 3],
							"name": "example",
							"obj": {"k2": "v2", "k1": "v1"}
						}
					}`),
				},
				ctx: context.Background(),
			},
		},
		{
			name: "test with update data type error",
			fields: fields{
				RunContext: &BlockRunContext{
					TaskID:            workID,
					skipNotifications: true,
					skipProduce:       true,
					VarStore:          store.NewStore(),
				},
				State: &ExecutableFunction{
					HasResponse: true,
					Function: script.FunctionParam{
						Output: `{"mktu": {"type": "array"}, "name": {"type": "number"}}`,
					},
				},
			},
			args: args{
				data: &script.BlockUpdateData{
					ByLogin: "example",
					Action:  string(entity.TaskUpdateActionExecution),
					Parameters: json.RawMessage(`{
						"mapping": {
							"mktu": [1, 2, 3],
							"name": "example"
						}
					}`),
				},
				ctx: context.Background(),
			},
			wantErr: true,
		},
		{
			name: "test with update data missing key",
			fields: fields{
				RunContext: &BlockRunContext{
					TaskID:            workID,
					skipNotifications: true,
					skipProduce:       true,
					VarStore:          store.NewStore(),
				},
				State: &ExecutableFunction{
					HasResponse: true,
					Function: script.FunctionParam{
						Output: `{"mktu": {"type": "array"}, "name": {"type": "string"}, "obj": {"type": "object", "properties": {"k1": {"type": "string"}}}}`,
					},
				},
			},
			args: args{
				data: &script.BlockUpdateData{
					ByLogin: "example",
					Action:  string(entity.TaskUpdateActionExecution),
					Parameters: json.RawMessage(`{
						"mapping": {
							"mktu": [1, 2, 3],
							"name": "example",
							"obj": {"k2": "v2"}
						}
					}`),
				},
				ctx: context.Background(),
			},
			wantErr: true,
		},
		{
			name: "test use value from constant",
			fields: fields{
				Name: stepName,
				State: &ExecutableFunction{
					Mapping: script.JSONSchemaProperties{
						"mktu.code": script.JSONSchemaPropertiesValue{
							Type:  "string",
							Value: "servicedesk_application_0.application_body.class_mktu_code",
						},
						"name": script.JSONSchemaPropertiesValue{
							Type:  "string",
							Value: "servicedesk_application_0.application_body.short_name",
						},
					},
					Constants: map[string]interface{}{
						"mktu.code": "code_from_constant",
					},
					Function: script.FunctionParam{
						Input:         `{"name":{"type":"string","title":"имя"}}`,
						RequiredInput: []string{"name"},
					},
				},
				RunContext: &BlockRunContext{
					TaskID:            workID,
					skipNotifications: true,
					skipProduce:       true,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						s.SetValue("servicedesk_application_0.application_body", map[string]interface{}{
							"short_name": "test name",
							"recipient": map[string]interface{}{
								"fullname": "Egor Jopov",
							},
						})

						return s
					}(),
					Services: RunContextServices{
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("GetTaskStepByName",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								workID,
								stepName,
							).Return(
								&entity.Step{
									ID: uuid.New(),
								}, nil,
							)

							return res
						}(),
					},
				},
			},
			args: args{
				ctx:  context.Background(),
				data: nil,
			},
			wantErr: false,
		},
		{
			name: "test response with do reply true and exceeded max replay",
			fields: fields{
				RunContext: &BlockRunContext{
					TaskID:            workID,
					skipNotifications: true,
					skipProduce:       true,
					VarStore:          store.NewStore(),
				},
				State: &ExecutableFunction{
					RetryPolicy:        "simple",
					RetryCount:         1,
					CurrRetryCount:     2,
					RetryCountExceeded: true,
					HasResponse:        false,
					Function: script.FunctionParam{
						Output: `{"mktu": {"type": "array"}, "name": {"type": "number"}}`,
					},
				},
			},
			args: args{
				data: &script.BlockUpdateData{
					ByLogin: "example",
					Parameters: json.RawMessage(`{
						"do_retry":true,
						"mapping": {
							"mktu": [1, 2, 3],
							"name": "example"
						}
					}`),
				},
				ctx: context.Background(),
			},
			wantErr: false,
		},
		{
			name: "test response with do reply false and error",
			fields: fields{
				RunContext: &BlockRunContext{
					TaskID:            workID,
					skipNotifications: true,
					skipProduce:       true,
					VarStore:          store.NewStore(),
				},
				State: &ExecutableFunction{
					HasResponse: false,
					Function: script.FunctionParam{
						Output: `{"mktu": {"type": "array"}, "name": {"type": "number"}}`,
					},
				},
			},
			args: args{
				data: &script.BlockUpdateData{
					ByLogin: "example",
					Parameters: json.RawMessage(`{
						"do_retry":false,
						"err":"test error",
						"mapping": {
							"mktu": [1, 2, 3],
							"name": "example"
						}
					}`),
				},
				ctx: context.Background(),
			},
			wantErr: true,
		},
		{
			name: "test with response do reply true and not exceeded max replay",
			fields: fields{
				RunContext: &BlockRunContext{
					TaskID:            workID,
					skipNotifications: true,
					skipProduce:       true,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						s.SetValue("servicedesk_application_0.application_body", map[string]interface{}{
							"short_name": "test name",
							"recipient": map[string]interface{}{
								"fullname": "Egor Jopov",
							},
						})

						return s
					}(),
					Services: RunContextServices{
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("GetTaskStepByName",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								workID,
								stepName,
							).Return(
								&entity.Step{
									ID: uuid.New(),
								}, nil,
							)

							return res
						}(),
					},
				},
				State: &ExecutableFunction{
					RetryPolicy:        "simple",
					RetryCount:         1,
					RetryCountExceeded: false,
					HasResponse:        false,
					Function: script.FunctionParam{
						Output: `{"mktu": {"type": "array"}, "name": {"type": "number"}}`,
					},
				},
			},
			args: args{
				data: &script.BlockUpdateData{
					ByLogin: "example",
					Parameters: json.RawMessage(`{
			           "do_retry":true,
			           "mapping": {
				          "mktu": [1, 2, 3],
				          "name": "example"
			            }
		            }`),
				},
				ctx: context.Background(),
			},
			wantErr: false,
		},
		{
			name: "test action reply",
			fields: fields{
				Name: stepName,
				State: &ExecutableFunction{
					RetryPolicy:        "simple",
					RetryCount:         1,
					CurrRetryTimeout:   3,
					RetryCountExceeded: false,
					HasResponse:        false,
					Function: script.FunctionParam{
						Input:         `{"name":{"type":"string","title":"имя"}}`,
						RequiredInput: []string{},
						Output:        `{"mktu": {"type": "array"}, "name": {"type": "number"}}`,
					},
				},
				RunContext: &BlockRunContext{
					TaskID:            workID,
					skipNotifications: true,
					skipProduce:       true,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						s.SetValue("servicedesk_application_0.application_body", map[string]interface{}{
							"short_name": "test name",
							"recipient": map[string]interface{}{
								"fullname": "Egor Jopov",
							},
						})

						return s
					}(),
					Services: RunContextServices{
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("GetTaskStepByName",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								workID,
								stepName,
							).Return(
								&entity.Step{
									ID: uuid.New(),
								}, nil,
							)

							return res
						}(),
					},
				},
			},
			args: args{
				ctx: context.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    "example",
					Action:     string(entity.TaskUpdateActionRetry),
					Parameters: json.RawMessage(`{}`),
				},
			},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			efb := &ExecutableFunctionBlock{
				Name:       test.fields.Name,
				Title:      test.fields.Title,
				Input:      test.fields.Input,
				Output:     test.fields.Output,
				Sockets:    test.fields.Sockets,
				State:      test.fields.State,
				RunURL:     test.fields.RunURL,
				RunContext: test.fields.RunContext,
			}
			test.fields.RunContext.UpdateData = test.args.data
			_, err := efb.Update(test.args.ctx)

			assert.Equalf(t, test.wantErr, err != nil, fmt.Sprintf("Update(%+v)", test.args.ctx))
		})
	}
}

func TestExecutableFunctionBlock_restoreMapStructure(t *testing.T) {
	tests := []struct {
		name      string
		variables map[string]interface{}
		want      map[string]interface{}
	}{
		{
			name: "success case",
			variables: map[string]interface{}{
				"start_0.application_body.param1": "some_string",
				"start_0.application_body.param2": map[string]interface{}{
					"field1": 4,
					"field2": "string_value",
				},
				"form_0": map[string]interface{}{
					"application_body": map[string]interface{}{
						"A": 111,
					},
				},
				"param3": 123,
			},
			want: map[string]interface{}{
				"start_0": map[string]interface{}{
					"application_body": map[string]interface{}{
						"param1": "some_string",
						"param2": map[string]interface{}{
							"field1": 4,
							"field2": "string_value",
						},
					},
				},
				"form_0": map[string]interface{}{
					"application_body": map[string]interface{}{
						"A": 111,
					},
				},
				"param3": 123,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, script.RestoreMapStructure(tt.variables), "restoreMapStructure(%v)", tt.variables)
		})
	}
}
