package pipeline

import (
	"bytes"
	c "context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"golang.org/x/net/context"

	"github.com/google/uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db/mocks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc"
	serviceDeskMocks "gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc/mocks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

func TestExecution_Next(t *testing.T) {
	type fields struct {
		Name  string
		Nexts []script.Socket
		State *ExecutionData
	}

	type args struct {
		runCtx *store.VariableStore
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		want   []string
	}{
		{
			name: "default",
			fields: fields{
				Nexts: []script.Socket{script.DefaultSocket},
				State: &ExecutionData{},
			},
			args: args{
				runCtx: store.NewStore(),
			},
			want: []string(nil),
		},
		{
			name: "test executed",
			fields: fields{
				Nexts: []script.Socket{script.NewSocket("executed", []string{"test-next"})},
				State: &ExecutionData{
					Decision: func() *ExecutionDecision {
						res := ExecutionDecisionExecuted
						return &res
					}(),
				},
			},
			args: args{
				runCtx: store.NewStore(),
			},
			want: []string{"test-next"},
		},
		{
			name: "test edit app",
			fields: fields{
				Nexts: []script.Socket{script.NewSocket("executor_send_edit_app", []string{"test-next"})},
				State: &ExecutionData{
					Decision: func() *ExecutionDecision {
						res := ExecutionDecisionSentEdit
						return &res
					}(),
					EditingApp: nil,
				},
			},
			args: args{
				runCtx: store.NewStore(),
			},
			want: []string{"test-next"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			block := &GoExecutionBlock{
				Name:    test.fields.Name,
				Sockets: test.fields.Nexts,
				State:   test.fields.State,
			}
			got, _ := block.Next(test.args.runCtx)

			assert.Equal(t, test.want, got)
		})
	}
}

func TestGoExecutionBlock_createGoExecutionBlock(t *testing.T) {
	const (
		example             = "example"
		title               = "title"
		shortTitle          = "Нода Исполнение"
		executorsFromSchema = "form_0.user.username;form_1.user.username"
		executorFromSchema  = "form_0.user.username"
	)
	myStorage := makeStorage()

	varStore := store.NewStore()

	varStore.SetValue("form_0.user", map[string]interface{}{
		"username": "test",
		"fullname": "test test test",
	})
	varStore.SetValue("form_1.user", map[string]interface{}{
		"username": "test2",
		"fullname": "test2 test test",
	})

	next := []entity.Socket{
		{
			Id:           DefaultSocketID,
			Title:        script.DefaultSocketTitle,
			NextBlockIds: []string{"next_0"},
		},
		{
			Id:           rejectedSocketID,
			Title:        script.RejectSocketTitle,
			NextBlockIds: []string{"next_1"},
		},
	}

	type args struct {
		name   string
		ef     *entity.EriusFunc
		runCtx *BlockRunContext
	}

	tests := []struct {
		name string
		args args
		want *GoExecutionBlock
	}{
		{
			name: "no execution params",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType:  BlockGoExecutionID,
					Sockets:    next,
					Input:      nil,
					Output:     nil,
					Params:     nil,
					Title:      title,
					ShortTitle: shortTitle,
				},
				runCtx: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
				},
			},
			want: nil,
		},
		{
			name: "invalid execution params",
			args: args{
				name: example,
				ef: &entity.EriusFunc{
					BlockType:  BlockGoExecutionID,
					Sockets:    next,
					Input:      nil,
					Output:     nil,
					Params:     []byte("{}"),
					Title:      title,
					ShortTitle: shortTitle,
				},
				runCtx: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
				},
			},
			want: nil,
		},
		{
			name: "executors from schema",
			args: args{
				name: example,
				runCtx: &BlockRunContext{
					WorkNumber:        "J001",
					skipNotifications: true,
					VarStore:          varStore,
					Services: RunContextServices{
						Storage: myStorage,
					},
				},
				ef: &entity.EriusFunc{
					BlockType:  BlockGoExecutionID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
					Input: []entity.EriusFunctionValue{
						{
							Name:   "foo",
							Type:   "string",
							Global: "bar",
						},
					},
					Output: &script.JSONSchema{
						Type: "object",
						Properties: map[string]script.JSONSchemaPropertiesValue{
							"foo": {
								Type:   "string",
								Global: "bar",
							},
						},
					},
					Params: func() []byte {
						r, _ := json.Marshal(&script.ExecutionParams{
							Type:               script.ExecutionTypeFromSchema,
							Executors:          executorsFromSchema,
							SLA:                8,
							FormsAccessibility: make([]script.FormAccessibility, 1),
						})
						return r
					}(),
				},
			},
			want: &GoExecutionBlock{
				Name:      example,
				ShortName: shortTitle,
				Title:     title,
				Input: map[string]string{
					"foo": "bar",
				},
				Output: map[string]string{
					"foo": "bar",
				},
				happenedEvents: make([]entity.NodeEvent, 0),
				Sockets:        entity.ConvertSocket(next),
				RunContext: &BlockRunContext{
					Services: RunContextServices{
						Storage: myStorage,
					},
					WorkNumber:        "J001",
					skipNotifications: true,
					VarStore:          varStore,
				},
				State: &ExecutionData{
					WorkType:           "8/5",
					ExecutionType:      script.ExecutionTypeFromSchema,
					Executors:          map[string]struct{}{"test": {}, "test2": {}},
					SLA:                8,
					FormsAccessibility: make([]script.FormAccessibility, 1),
					InitialExecutors:   map[string]struct{}{"test": {}, "test2": {}},
				},
			},
		},
		{
			name: "executor from schema",
			args: args{
				name: example,
				runCtx: &BlockRunContext{
					WorkNumber:        "J001",
					skipNotifications: true,
					VarStore:          varStore,
					Services: RunContextServices{
						Storage: myStorage,
					},
				},
				ef: &entity.EriusFunc{
					BlockType:  BlockGoExecutionID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
					Input: []entity.EriusFunctionValue{
						{
							Name:   "foo",
							Type:   "string",
							Global: "bar",
						},
					},
					Output: &script.JSONSchema{
						Type: "object",
						Properties: map[string]script.JSONSchemaPropertiesValue{
							"foo": {
								Type:   "string",
								Global: "bar",
							},
						},
					},
					Params: func() []byte {
						r, _ := json.Marshal(&script.ExecutionParams{
							Type:               script.ExecutionTypeFromSchema,
							Executors:          executorFromSchema,
							SLA:                8,
							FormsAccessibility: make([]script.FormAccessibility, 1),
						})
						return r
					}(),
				},
			},
			want: &GoExecutionBlock{
				Name:      example,
				ShortName: shortTitle,
				Title:     title,
				Input: map[string]string{
					"foo": "bar",
				},
				Output: map[string]string{
					"foo": "bar",
				},
				happenedEvents: make([]entity.NodeEvent, 0),
				Sockets:        entity.ConvertSocket(next),
				RunContext: &BlockRunContext{
					Services: RunContextServices{
						Storage: myStorage,
					},
					WorkNumber:        "J001",
					skipNotifications: true,
					VarStore:          varStore,
				},
				State: &ExecutionData{
					WorkType:           "8/5",
					ExecutionType:      script.ExecutionTypeFromSchema,
					Executors:          map[string]struct{}{"test": {}},
					SLA:                8,
					FormsAccessibility: make([]script.FormAccessibility, 1),
					IsTakenInWork:      false,
					InitialExecutors:   map[string]struct{}{"test": {}},
				},
			},
		},
		{
			name: "load execution state",
			args: args{
				name: example,
				runCtx: &BlockRunContext{
					skipNotifications: true,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						r, _ := json.Marshal(&ExecutionData{
							ExecutionType: script.ExecutionTypeUser,
							Executors: map[string]struct{}{
								"tester": {},
							},
							SLA:                1,
							FormsAccessibility: make([]script.FormAccessibility, 1),
						})
						s.State = map[string]json.RawMessage{
							example: r,
						}
						return s
					}(),
				},
				ef: &entity.EriusFunc{
					BlockType:  BlockGoExecutionID,
					Title:      title,
					ShortTitle: shortTitle,
					Sockets:    next,
					Input: []entity.EriusFunctionValue{
						{
							Name:   "foo",
							Type:   "string",
							Global: "bar",
						},
					},
					Output: &script.JSONSchema{
						Type: "object",
						Properties: map[string]script.JSONSchemaPropertiesValue{
							"foo": {
								Type:   "string",
								Global: "bar",
							},
						},
					},
					Params: func() []byte {
						r, _ := json.Marshal(&script.ExecutionParams{
							Type:               script.ExecutionTypeUser,
							Executors:          "tester",
							SLA:                1,
							FormsAccessibility: make([]script.FormAccessibility, 1),
						})
						return r
					}(),
				},
			},
			want: &GoExecutionBlock{
				Name:      example,
				ShortName: shortTitle,
				Title:     title,
				Input: map[string]string{
					"foo": "bar",
				},
				Output: map[string]string{
					"foo": "bar",
				},
				happenedEvents: make([]entity.NodeEvent, 0),
				Sockets:        entity.ConvertSocket(next),
				RunContext: &BlockRunContext{
					skipNotifications: true,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						r, _ := json.Marshal(&ExecutionData{
							ExecutionType: script.ExecutionTypeUser,
							Executors: map[string]struct{}{
								"tester": {},
							},
							SLA:                1,
							FormsAccessibility: make([]script.FormAccessibility, 1),
						})
						s.State = map[string]json.RawMessage{
							example: r,
						}
						s.Steps = []string{example}
						return s
					}(),
				},
				State: &ExecutionData{
					ExecutionType: script.ExecutionTypeUser,
					Executors: map[string]struct{}{
						"tester": {},
					},
					SLA:                 1,
					FormsAccessibility:  make([]script.FormAccessibility, 1),
					DecisionAttachments: make([]entity.Attachment, 0),
					IsTakenInWork:       false,
					InitialExecutors: map[string]struct{}{
						"tester": {},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := c.Background()
			got, _, _ := createGoExecutionBlock(ctx, test.args.name, test.args.ef, test.args.runCtx, nil)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestGoExecutionBlock_Update(t *testing.T) {
	stepId := uuid.New()
	const (
		exampleExecutor       = "example"
		secondExampleExecutor = "example1"
		stepName              = "exec"
	)

	type fields struct {
		Name          string
		Title         string
		Input         map[string]string
		Output        map[string]string
		NextStep      []script.Socket
		ExecutionData *ExecutionData
		RunContext    *BlockRunContext
	}
	type args struct {
		ef   *entity.EriusFunc
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
			name: "empty data",
			fields: fields{
				Name: stepName,
				RunContext: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}
							res.On("GetTaskStepById",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								stepId,
							).Return(
								&entity.Step{
									Time: time.Time{},
									Type: BlockGoExecutionID,
									Name: stepName,
									State: map[string]json.RawMessage{
										stepName: func() []byte {
											r, _ := json.Marshal(&ExecutionData{
												ExecutionType: script.ExecutionTypeUser,
												Executors: map[string]struct{}{
													exampleExecutor: {},
												},
											})

											return r
										}(),
									},
									Errors:      nil,
									Steps:       nil,
									BreakPoints: nil,
									HasError:    false,
									Status:      "",
								}, nil,
							)
							res.On("UpdateStepContext",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								mock.AnythingOfType("*db.UpdateStepRequest"),
							).Return(
								nil,
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
			name: "one executor with not taken in work",
			fields: fields{
				Name: stepName,
				ExecutionData: &ExecutionData{
					IsTakenInWork: false,
					ExecutionType: script.ExecutionTypeUser,
					Executors: map[string]struct{}{
						exampleExecutor: {},
					},
				},
				RunContext: &BlockRunContext{
					skipNotifications: false,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("GetTaskStepById",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								stepId,
							).Return(
								&entity.Step{
									Time: time.Time{},
									Type: BlockGoExecutionID,
									Name: stepName,
									State: map[string]json.RawMessage{
										stepName: func() []byte {
											r, _ := json.Marshal(&ExecutionData{
												ExecutionType: script.ExecutionTypeUser,
												Executors: map[string]struct{}{
													exampleExecutor: {},
												},
											})

											return r
										}(),
									},
									Errors:      nil,
									Steps:       nil,
									BreakPoints: nil,
									HasError:    false,
									Status:      "",
								}, nil,
							)

							res.On("UpdateStepContext",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								mock.AnythingOfType("*db.UpdateStepRequest"),
							).Return(
								nil,
							)

							return res
						}(),
					},
				},
			},

			args: args{
				ctx: context.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    exampleExecutor,
					Action:     string(entity.TaskUpdateActionExecution),
					Parameters: []byte(`{"decision":"` + ExecutionDecisionExecuted + `"}`),
				},
			},
			wantErr: true,
		},
		{
			name: "Nil executors",
			fields: fields{
				Name: stepName,
				ExecutionData: &ExecutionData{
					IsTakenInWork: true,
					ExecutionType: script.ExecutionTypeUser,
					Executors:     map[string]struct{}{},
				},
				RunContext: &BlockRunContext{
					skipNotifications: false,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("GetTaskStepById",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								stepId,
							).Return(
								&entity.Step{
									Time: time.Time{},
									Type: BlockGoExecutionID,
									Name: stepName,
									State: map[string]json.RawMessage{
										stepName: func() []byte {
											r, _ := json.Marshal(&ExecutionData{
												ExecutionType: script.ExecutionTypeUser,
												Executors:     map[string]struct{}{},
											})

											return r
										}(),
									},
									Errors:      nil,
									Steps:       nil,
									BreakPoints: nil,
									HasError:    false,
									Status:      "",
								}, nil,
							)

							res.On("UpdateStepContext",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								mock.AnythingOfType("*db.UpdateStepRequest"),
							).Return(
								nil,
							)

							return res
						}(),
					},
				},
			},

			args: args{
				ctx: context.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    exampleExecutor,
					Action:     string(entity.TaskUpdateActionExecution),
					Parameters: []byte(`{"decision":"` + ExecutionDecisionExecuted + `"}`),
				},
			},
			wantErr: true,
		},
		{
			name: "one executor send edit",
			fields: fields{
				Name: stepName,
				ExecutionData: &ExecutionData{
					IsTakenInWork: true,
					ExecutionType: script.ExecutionTypeUser,
					Executors: map[string]struct{}{
						exampleExecutor: {},
					},
				},
				RunContext: &BlockRunContext{
					skipNotifications: false,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("GetTaskStepById",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								stepId,
							).Return(
								&entity.Step{
									Time: time.Time{},
									Type: BlockGoExecutionID,
									Name: stepName,
									State: map[string]json.RawMessage{
										stepName: func() []byte {
											r, _ := json.Marshal(&ExecutionData{
												ExecutionType: script.ExecutionTypeUser,
												Executors: map[string]struct{}{
													exampleExecutor: {},
												},
											})

											return r
										}(),
									},
									Errors:      nil,
									Steps:       nil,
									BreakPoints: nil,
									HasError:    false,
									Status:      "",
								}, nil,
							)

							res.On("UpdateStepContext",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								mock.AnythingOfType("*db.UpdateStepRequest"),
							).Return(
								nil,
							)

							return res
						}(),
					},
				},
			},

			args: args{
				ctx: context.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    exampleExecutor,
					Action:     string(entity.TaskUpdateActionExecution),
					Parameters: []byte(`{"decision":"` + ExecutionDecisionSentEdit + `"}`),
				},
			},
			wantErr: true,
		},
		{
			name: "one executor rejected",
			fields: fields{
				Name: stepName,
				ExecutionData: &ExecutionData{
					IsTakenInWork: true,
					ExecutionType: script.ExecutionTypeUser,
					Executors: map[string]struct{}{
						exampleExecutor:       {},
						secondExampleExecutor: {},
					},
				},
				RunContext: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						ServiceDesc: func() *servicedesc.Service {
							sdMock := servicedesc.Service{
								SdURL: "",
							}
							httpClient := http.DefaultClient
							mockTransport := serviceDeskMocks.RoundTripper{}
							fResponse := func(*http.Request) *http.Response {
								b, _ := json.Marshal(servicedesc.SsoPerson{})
								body := io.NopCloser(bytes.NewReader(b))
								defer body.Close()
								return &http.Response{
									Status:     http.StatusText(http.StatusOK),
									StatusCode: http.StatusOK,
									Body:       body,
								}
							}
							f_error := func(*http.Request) error {
								return nil
							}
							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, f_error)
							httpClient.Transport = &mockTransport
							sdMock.Cli = httpClient

							return &sdMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("GetTaskStepById",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								stepId,
							).Return(
								&entity.Step{
									Time: time.Time{},
									Type: BlockGoExecutionID,
									Name: stepName,
									State: map[string]json.RawMessage{
										stepName: func() []byte {
											r, _ := json.Marshal(&ExecutionData{
												ExecutionType: script.ExecutionTypeUser,
												Executors: map[string]struct{}{
													exampleExecutor:       {},
													secondExampleExecutor: {},
												},
											})

											return r
										}(),
									},
									Errors:      nil,
									Steps:       nil,
									BreakPoints: nil,
									HasError:    false,
									Status:      "",
								}, nil,
							)

							res.On("UpdateStepContext",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								mock.AnythingOfType("*db.UpdateStepRequest"),
							).Return(
								nil,
							)

							return res
						}(),
					},
				},
			},

			args: args{
				ctx: context.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    exampleExecutor,
					Action:     string(entity.TaskUpdateActionExecution),
					Parameters: []byte(`{"decision":"` + ExecutionDecisionExecuted + `"}`),
				},
			},
			wantErr: false,
		},
		{
			name: "one executor executed",
			fields: fields{
				Name: stepName,
				ExecutionData: &ExecutionData{
					IsTakenInWork: true,
					ExecutionType: script.ExecutionTypeUser,
					Executors: map[string]struct{}{
						exampleExecutor:       {},
						secondExampleExecutor: {},
					},
				},
				RunContext: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						ServiceDesc: func() *servicedesc.Service {
							sdMock := servicedesc.Service{
								SdURL: "",
							}
							httpClient := http.DefaultClient
							mockTransport := serviceDeskMocks.RoundTripper{}
							fResponse := func(*http.Request) *http.Response {
								b, _ := json.Marshal(servicedesc.SsoPerson{})
								body := io.NopCloser(bytes.NewReader(b))
								defer body.Close()
								return &http.Response{
									Status:     http.StatusText(http.StatusOK),
									StatusCode: http.StatusOK,
									Body:       body,
								}
							}
							f_error := func(*http.Request) error {
								return nil
							}
							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, f_error)
							httpClient.Transport = &mockTransport
							sdMock.Cli = httpClient

							return &sdMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("GetTaskStepById",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								stepId,
							).Return(
								&entity.Step{
									Time: time.Time{},
									Type: BlockGoExecutionID,
									Name: stepName,
									State: map[string]json.RawMessage{
										stepName: func() []byte {
											r, _ := json.Marshal(&ExecutionData{
												ExecutionType: script.ExecutionTypeUser,
												Executors: map[string]struct{}{
													exampleExecutor:       {},
													secondExampleExecutor: {},
												},
											})

											return r
										}(),
									},
									Errors:      nil,
									Steps:       nil,
									BreakPoints: nil,
									HasError:    false,
									Status:      "",
								}, nil,
							)

							res.On("UpdateStepContext",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								mock.AnythingOfType("*db.UpdateStepRequest"),
							).Return(
								nil,
							)

							return res
						}(),
					},
				},
			},

			args: args{
				ctx: context.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    exampleExecutor,
					Action:     string(entity.TaskUpdateActionExecution),
					Parameters: []byte(`{"decision":"` + ExecutionDecisionExecuted + `"}`),
				},
			},
			wantErr: false,
		},
		{
			name: "second executor",
			fields: fields{
				Name: stepName,
				ExecutionData: &ExecutionData{
					IsTakenInWork: true,
					ExecutionType: script.ExecutionTypeUser,
					Executors: map[string]struct{}{
						secondExampleExecutor: {},
					},
				},
				RunContext: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						ServiceDesc: func() *servicedesc.Service {
							sdMock := servicedesc.Service{
								SdURL: "",
							}
							httpClient := http.DefaultClient
							mockTransport := serviceDeskMocks.RoundTripper{}
							fResponse := func(*http.Request) *http.Response {
								b, _ := json.Marshal(servicedesc.SsoPerson{})
								body := io.NopCloser(bytes.NewReader(b))
								defer body.Close()
								return &http.Response{
									Status:     http.StatusText(http.StatusOK),
									StatusCode: http.StatusOK,
									Body:       body,
								}
							}
							f_error := func(*http.Request) error {
								return nil
							}
							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, f_error)
							httpClient.Transport = &mockTransport
							sdMock.Cli = httpClient

							return &sdMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("GetTaskStepById",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								stepId,
							).Return(
								&entity.Step{
									Time: time.Time{},
									Type: BlockGoExecutionID,
									Name: stepName,
									State: map[string]json.RawMessage{
										stepName: func() []byte {
											r, _ := json.Marshal(&ExecutionData{
												ExecutionType: script.ExecutionTypeUser,
												Executors: map[string]struct{}{
													secondExampleExecutor: {},
												},
											})

											return r
										}(),
									},
									Errors:      nil,
									Steps:       nil,
									BreakPoints: nil,
									HasError:    false,
									Status:      "",
								}, nil,
							)

							res.On("UpdateStepContext",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								mock.AnythingOfType("*db.UpdateStepRequest"),
							).Return(
								nil,
							)

							return res
						}(),
					},
				},
			},

			args: args{
				ctx: context.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    secondExampleExecutor,
					Action:     string(entity.TaskUpdateActionExecution),
					Parameters: []byte(`{"decision":"` + ExecutionDecisionExecuted + `"}`),
				},
			},
			wantErr: false,
		},
		{
			name: "any of executors",
			fields: fields{
				Name: stepName,
				ExecutionData: &ExecutionData{
					IsTakenInWork: true,
					ExecutionType: script.ExecutionTypeUser,
					Executors: map[string]struct{}{
						exampleExecutor:       {},
						secondExampleExecutor: {},
					},
				},
				RunContext: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						ServiceDesc: func() *servicedesc.Service {
							sdMock := servicedesc.Service{
								SdURL: "",
							}
							httpClient := http.DefaultClient
							mockTransport := serviceDeskMocks.RoundTripper{}
							fResponse := func(*http.Request) *http.Response {
								b, _ := json.Marshal(servicedesc.SsoPerson{})
								body := io.NopCloser(bytes.NewReader(b))
								defer body.Close()
								return &http.Response{
									Status:     http.StatusText(http.StatusOK),
									StatusCode: http.StatusOK,
									Body:       body,
								}
							}
							f_error := func(*http.Request) error {
								return nil
							}
							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, f_error)
							httpClient.Transport = &mockTransport
							sdMock.Cli = httpClient

							return &sdMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("GetTaskStepById",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								stepId,
							).Return(
								&entity.Step{
									Time: time.Time{},
									Type: BlockGoExecutionID,
									Name: stepName,
									State: map[string]json.RawMessage{
										stepName: func() []byte {
											r, _ := json.Marshal(&ExecutionData{
												ExecutionType: script.ExecutionTypeUser,
												Executors: map[string]struct{}{
													exampleExecutor:       {},
													secondExampleExecutor: {},
												},
											})

											return r
										}(),
									},
									Errors:      nil,
									Steps:       nil,
									BreakPoints: nil,
									HasError:    false,
									Status:      "",
								}, nil,
							)

							res.On("UpdateStepContext",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								mock.AnythingOfType("*db.UpdateStepRequest"),
							).Return(
								nil,
							)

							return res
						}(),
					},
				},
			},

			args: args{
				ctx: context.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    exampleExecutor,
					Action:     string(entity.TaskUpdateActionExecution),
					Parameters: []byte(`{"decision":"` + ExecutionDecisionExecuted + `"}`),
				},
			},
			wantErr: false,
		},
		{
			name: "acceptance test",
			fields: fields{
				Name: stepName,
				ExecutionData: &ExecutionData{
					IsTakenInWork: true,
					ExecutionType: script.ExecutionTypeUser,
					Executors: map[string]struct{}{
						exampleExecutor: {},
					},
				},
				RunContext: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						ServiceDesc: func() *servicedesc.Service {
							sdMock := servicedesc.Service{
								SdURL: "",
							}
							httpClient := http.DefaultClient
							mockTransport := serviceDeskMocks.RoundTripper{}
							fResponse := func(*http.Request) *http.Response {
								b, _ := json.Marshal(servicedesc.SsoPerson{})
								body := io.NopCloser(bytes.NewReader(b))
								defer body.Close()
								return &http.Response{
									Status:     http.StatusText(http.StatusOK),
									StatusCode: http.StatusOK,
									Body:       body,
								}
							}
							f_error := func(*http.Request) error {
								return nil
							}
							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, f_error)
							httpClient.Transport = &mockTransport
							sdMock.Cli = httpClient

							return &sdMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("GetTaskStepById",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								stepId,
							).Return(
								&entity.Step{
									Time: time.Time{},
									Type: BlockGoExecutionID,
									Name: stepName,
									State: map[string]json.RawMessage{
										stepName: func() []byte {
											r, _ := json.Marshal(&ExecutionData{
												ExecutionType: script.ExecutionTypeUser,
												Executors: map[string]struct{}{
													exampleExecutor: {},
												},
											})

											return r
										}(),
									},
									Errors:      nil,
									Steps:       nil,
									BreakPoints: nil,
									HasError:    false,
									Status:      "",
								}, nil,
							)

							res.On("UpdateStepContext",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								mock.AnythingOfType("*db.UpdateStepRequest"),
							).Return(
								nil,
							)

							return res
						}(),
					},
				},
			},

			args: args{
				ctx: context.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    exampleExecutor,
					Action:     string(entity.TaskUpdateActionExecution),
					Parameters: []byte(`{"decision":"` + ExecutionDecisionExecuted + `"}`),
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gb := &GoExecutionBlock{
				Name:       tt.fields.Name,
				Title:      tt.fields.Title,
				Input:      tt.fields.Input,
				Output:     tt.fields.Output,
				Sockets:    tt.fields.NextStep,
				State:      tt.fields.ExecutionData,
				RunContext: tt.fields.RunContext,
			}
			tt.fields.RunContext.UpdateData = tt.args.data
			_, err := gb.Update(tt.args.ctx)
			assert.Equalf(t, tt.wantErr, err != nil, fmt.Sprintf("Update(%v, %v)", tt.args.ctx, tt.args.data))
		})
	}
}
