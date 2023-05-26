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
		workId   = uuid.New()
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
					TaskID:            workId,
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
					Storage: func() db.Database {
						res := &mocks.MockedDatabase{}

						res.On("GetTaskStepByName",
							mock.MatchedBy(func(ctx context.Context) bool { return true }),
							workId,
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
					TaskID:            workId,
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
					Storage: func() db.Database {
						res := &mocks.MockedDatabase{}

						res.On("GetTaskStepByName",
							mock.MatchedBy(func(ctx context.Context) bool { return true }),
							workId,
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
					TaskID:            workId,
					skipNotifications: true,
					skipProduce:       true,
					VarStore:          store.NewStore(),
				},
				State: &ExecutableFunction{
					HasResponse: true,
					Function: script.FunctionParam{
						Output: `{"mktu": {"type": "array", "items": [{"type": "number"}]}, "name": {"type": "string"}, "obj": {"type": "object", "properties": {"k1": {"type": "string"}}}}`,
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
					TaskID:            workId,
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
					TaskID:            workId,
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
			name: "test constant value priority",
			fields: fields{
				RunContext: &BlockRunContext{
					TaskID:            workId,
					skipNotifications: true,
					skipProduce:       true,
					VarStore:          store.NewStore(),
				},
				State: &ExecutableFunction{
					HasResponse: true,
					Function: script.FunctionParam{
						Output: `{"name": {"type": "string"}}`,
					},
					Constants: map[string]interface{}{"name": "name from constant", "user.userName": "testLogin"},
				},
			},
			args: args{
				data: &script.BlockUpdateData{
					ByLogin: "example",
					Action:  string(entity.TaskUpdateActionExecution),
					Parameters: json.RawMessage(`{
						"mapping": {
							"name": "example"
						}
					}`),
				},
				ctx: context.Background(),
			},
			wantErr: true,
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

			assert.Equalf(t, test.wantErr, err != nil, fmt.Sprintf("Update(%v)", test.args.ctx))
		})
	}
}
