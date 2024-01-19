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
