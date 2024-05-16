package pipeline

import (
	"bytes"
	c "context"
	"encoding/json"
	"fmt"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people/nocache"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"golang.org/x/net/context"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db/mocks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/httpclient"
	humanTasks "gitlab.services.mts.ru/jocasta/pipeliner/internal/humantasks"
	mocks2 "gitlab.services.mts.ru/jocasta/pipeliner/internal/humantasks/mocks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc"
	serviceDeskMocks "gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc/mocks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sla"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"

	delegationht "gitlab.services.mts.ru/jocasta/human-tasks/pkg/proto/gen/proto/go/delegation"
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
	workType := "8/5"

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
			ID:           DefaultSocketID,
			Title:        script.DefaultSocketTitle,
			NextBlockIds: []string{"next_0"},
		},
		{
			ID:           rejectedSocketID,
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
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
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
							WorkType:           &workType,
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
				happenedKafkaEvents: []entity.NodeKafkaEvent{
					{
						TaskID:        "00000000-0000-0000-0000-000000000000",
						WorkNumber:    "J001",
						NodeName:      "example",
						NodeShortName: "Нода Исполнение",
						NodeStart:     time.Now().Unix(),
						TaskStatus:    "processing",
						NodeStatus:    "running",
						CreatedAt:     time.Now().Unix(),
						NodeSLA:       -62135546400,
						Action:        "start",
						NodeType:      "execution",
						ActionBody: map[string]interface{}{
							"toAdd": []string{
								"test",
								"test2",
							},
						},
						AvailableActions: []string{},
					},
				},
				Sockets: entity.ConvertSocket(next),
				RunContext: &BlockRunContext{
					Services: RunContextServices{
						Storage: myStorage,
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
					},
					WorkNumber:        "J001",
					skipNotifications: true,
					VarStore:          varStore,
				},
				State: &ExecutionData{
					WorkType:           workType,
					ExecutionType:      script.ExecutionTypeFromSchema,
					Deadline:           time.Date(1, time.January, 1, 14, 0, 0, 0, time.UTC),
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
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
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
							WorkType:           &workType,
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
				happenedKafkaEvents: []entity.NodeKafkaEvent{
					{
						TaskID:        "00000000-0000-0000-0000-000000000000",
						WorkNumber:    "J001",
						NodeName:      "example",
						NodeShortName: "Нода Исполнение",
						NodeStart:     time.Now().Unix(),
						TaskStatus:    "processing",
						NodeStatus:    "running",
						CreatedAt:     time.Now().Unix(),
						NodeSLA:       -62135546400,
						Action:        "start",
						NodeType:      "execution",
						ActionBody: map[string]interface{}{
							"toAdd": []string{
								"test",
							},
						},
						AvailableActions: []string{},
					},
				},
				Sockets: entity.ConvertSocket(next),
				RunContext: &BlockRunContext{
					Services: RunContextServices{
						Storage: myStorage,
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
					},
					WorkNumber:        "J001",
					skipNotifications: true,
					VarStore:          varStore,
				},
				State: &ExecutionData{
					WorkType:           workType,
					ExecutionType:      script.ExecutionTypeFromSchema,
					Deadline:           time.Date(1, time.January, 1, 14, 0, 0, 0, time.UTC),
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
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
					},
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						r, _ := json.Marshal(&ExecutionData{
							ExecutionType: script.ExecutionTypeUser,
							Executors: map[string]struct{}{
								"tester": {},
							},
							SLA:                1,
							WorkType:           workType,
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
							WorkType:           &workType,
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
				happenedKafkaEvents: []entity.NodeKafkaEvent{
					{
						TaskID:        "00000000-0000-0000-0000-000000000000",
						NodeName:      "example",
						NodeShortName: "Нода Исполнение",
						NodeStart:     time.Now().Unix(),
						TaskStatus:    "processing",
						NodeStatus:    "running",
						CreatedAt:     time.Now().Unix(),
						NodeSLA:       -62135571600,
						Action:        "start",
						NodeType:      "execution",
						ActionBody: map[string]interface{}{
							"toAdd": []string{
								"tester",
							},
						},
						AvailableActions: []string{},
					},
				},
				Sockets: entity.ConvertSocket(next),
				RunContext: &BlockRunContext{
					skipNotifications: true,
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
					},
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						r, _ := json.Marshal(&ExecutionData{
							ExecutionType: script.ExecutionTypeUser,
							Executors: map[string]struct{}{
								"tester": {},
							},
							SLA:                1,
							WorkType:           workType,
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
					WorkType:            workType,
					Deadline:            time.Date(1, time.January, 1, 7, 0, 0, 0, time.UTC),
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
	stepID := uuid.New()

	const (
		exampleExecutor       = "example"
		secondExampleExecutor = "example1"
		stepName              = "exec"
		workType              = "8/5"
	)

	var (
		actualExecutor  = "executor"
		actualExecutor2 = "example"
	)

	type (
		fields struct {
			Name          string
			Title         string
			Input         map[string]string
			Output        map[string]string
			NextStep      []script.Socket
			ExecutionData *ExecutionData
			RunContext    *BlockRunContext
		}
		args struct {
			ctx  context.Context
			data *script.BlockUpdateData
		}
	)

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
						Storage: nil,
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
					WorkType:      workType,
					SLA:           8,
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
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						ServiceDesc: func() *servicedesc.Service {
							sdMock := servicedesc.Service{
								SdURL: "",
							}
							httpClient := http.DefaultClient
							retryableHttpClient := httpclient.NewClient(httpClient, nil, 0, 0)

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
							fError := func(*http.Request) error {
								return nil
							}
							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, fError)
							httpClient.Transport = &mockTransport
							sdMock.Cli = retryableHttpClient

							return &sdMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

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
					WorkType:      workType,
					SLA:           8,
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
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						ServiceDesc: func() *servicedesc.Service {
							sdMock := servicedesc.Service{
								SdURL: "",
							}
							httpClient := http.DefaultClient
							retryableHttpClient := httpclient.NewClient(httpClient, nil, 0, 0)

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
							fError := func(*http.Request) error {
								return nil
							}
							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, fError)
							httpClient.Transport = &mockTransport
							sdMock.Cli = retryableHttpClient

							return &sdMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

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
					SLA:      8,
					WorkType: workType,
				},
				RunContext: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						ServiceDesc: func() *servicedesc.Service {
							sdMock := servicedesc.Service{
								SdURL: "",
							}
							httpClient := http.DefaultClient
							retryableHttpClient := httpclient.NewClient(httpClient, nil, 0, 0)

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
							fError := func(*http.Request) error {
								return nil
							}
							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, fError)
							httpClient.Transport = &mockTransport
							sdMock.Cli = retryableHttpClient

							return &sdMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

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
					WorkType: workType,
					SLA:      8,
				},
				RunContext: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						ServiceDesc: func() *servicedesc.Service {
							sdMock := servicedesc.Service{
								SdURL: "",
							}
							httpClient := http.DefaultClient
							retryableHttpClient := httpclient.NewClient(httpClient, nil, 0, 0)

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
							fError := func(*http.Request) error {
								return nil
							}
							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, fError)
							httpClient.Transport = &mockTransport
							sdMock.Cli = retryableHttpClient

							return &sdMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

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
			name: "action TaskUpdateActionSLABreach",
			fields: fields{
				Name: stepName,
				ExecutionData: &ExecutionData{
					IsTakenInWork: true,
					CheckSLA:      true,
					SLA:           1,
					WorkType:      workType,
					ExecutionType: script.ExecutionTypeUser,
					Executors: map[string]struct{}{
						secondExampleExecutor: {},
					},
				},
				RunContext: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						ServiceDesc: func() *servicedesc.Service {
							sdMock := servicedesc.Service{
								SdURL: "",
							}
							httpClient := http.DefaultClient
							retryableHttpClient := httpclient.NewClient(httpClient, nil, 0, 0)

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
							fError := func(*http.Request) error {
								return nil
							}
							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, fError)
							httpClient.Transport = &mockTransport
							sdMock.Cli = retryableHttpClient

							return &sdMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							return res
						}(),
					},
				},
			},

			args: args{
				ctx: context.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    secondExampleExecutor,
					Action:     string(entity.TaskUpdateActionSLABreach),
					Parameters: []byte(`{"decision":"` + ExecutionDecisionExecuted + `"}`),
				},
			},
			wantErr: false,
		},
		{
			name: "action TaskUpdateActionHalfSLABreach",
			fields: fields{
				Name: stepName,
				ExecutionData: &ExecutionData{
					IsTakenInWork: true,
					CheckSLA:      false,
					ExecutionType: script.ExecutionTypeUser,
					Executors: map[string]struct{}{
						secondExampleExecutor: {},
					},
					WorkType: workType,
					SLA:      8,
				},
				RunContext: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						ServiceDesc: func() *servicedesc.Service {
							sdMock := servicedesc.Service{
								SdURL: "",
							}
							httpClient := http.DefaultClient
							retryableHttpClient := httpclient.NewClient(httpClient, nil, 0, 0)

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
							fError := func(*http.Request) error {
								return nil
							}
							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, fError)
							httpClient.Transport = &mockTransport
							sdMock.Cli = retryableHttpClient

							return &sdMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							return res
						}(),
					},
				},
			},

			args: args{
				ctx: context.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    secondExampleExecutor,
					Action:     string(entity.TaskUpdateActionHalfSLABreach),
					Parameters: []byte(`{"decision":"` + ExecutionDecisionExecuted + `"}`),
				},
			},
			wantErr: false,
		},
		{
			name: "case TaskUpdateActionReworkSLABreach false",
			fields: fields{
				Name: stepName,
				ExecutionData: &ExecutionData{
					IsTakenInWork: true,
					ExecutionType: script.ExecutionTypeUser,
					Executors: map[string]struct{}{
						secondExampleExecutor: {},
					},
					CheckReworkSLA: false,
					WorkType:       workType,
					SLA:            8,
				},
				RunContext: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						ServiceDesc: func() *servicedesc.Service {
							sdMock := servicedesc.Service{
								SdURL: "",
							}
							httpClient := http.DefaultClient
							retryableHttpClient := httpclient.NewClient(httpClient, nil, 0, 0)

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
							fError := func(*http.Request) error {
								return nil
							}
							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, fError)
							httpClient.Transport = &mockTransport
							sdMock.Cli = retryableHttpClient

							return &sdMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("GetTaskStepById",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								stepID,
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
					Action:     string(entity.TaskUpdateActionReworkSLABreach),
					Parameters: []byte(`{"decision":"` + ExecutionDecisionSentEdit + `"}`),
				},
			},
			wantErr: false,
		},
		{
			name: "action TaskUpdateActionExecution not take in work",
			fields: fields{
				Name: stepName,
				ExecutionData: &ExecutionData{
					IsTakenInWork: false,
					ExecutionType: script.ExecutionTypeUser,
					Executors: map[string]struct{}{
						secondExampleExecutor: {},
					},
					ActualExecutor: nil,
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
							retryableHttpClient := httpclient.NewClient(httpClient, nil, 0, 0)

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
							fError := func(*http.Request) error {
								return nil
							}
							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, fError)
							httpClient.Transport = &mockTransport
							sdMock.Cli = retryableHttpClient

							return &sdMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

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
			wantErr: true,
		},
		{
			name: "case TaskUpdateActionExecution nil actual executor",
			fields: fields{
				Name: stepName,
				ExecutionData: &ExecutionData{
					IsTakenInWork: true,
					ExecutionType: script.ExecutionTypeUser,
					Executors: map[string]struct{}{
						secondExampleExecutor: {},
					},
					ActualExecutor: nil,
					WorkType:       workType,
					SLA:            8,
				},
				RunContext: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						ServiceDesc: func() *servicedesc.Service {
							sdMock := servicedesc.Service{
								SdURL: "",
							}
							httpClient := http.DefaultClient
							retryableHttpClient := httpclient.NewClient(httpClient, nil, 0, 0)

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
							fError := func(*http.Request) error {
								return nil
							}
							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, fError)
							httpClient.Transport = &mockTransport
							sdMock.Cli = retryableHttpClient

							return &sdMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("GetTaskStepById",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								stepID,
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
			name: "action TaskUpdateActionExecution set actual executor",
			fields: fields{
				Name: stepName,
				ExecutionData: &ExecutionData{
					IsTakenInWork: true,
					ExecutionType: script.ExecutionTypeUser,
					Executors: map[string]struct{}{
						secondExampleExecutor: {},
					},
					ActualExecutor: &actualExecutor,
					WorkType:       workType,
					SLA:            8,
				},
				RunContext: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						ServiceDesc: func() *servicedesc.Service {
							sdMock := servicedesc.Service{
								SdURL: "",
							}
							httpClient := http.DefaultClient
							retryableHttpClient := httpclient.NewClient(httpClient, nil, 0, 0)

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
							fError := func(*http.Request) error {
								return nil
							}
							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, fError)
							httpClient.Transport = &mockTransport
							sdMock.Cli = retryableHttpClient

							return &sdMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

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
			name: "action TaskUpdateActionChangeExecutor not take in work",
			fields: fields{
				Name: stepName,
				ExecutionData: &ExecutionData{
					IsTakenInWork: false,
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
							retryableHttpClient := httpclient.NewClient(httpClient, nil, 0, 0)

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
							fError := func(*http.Request) error {
								return nil
							}
							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, fError)
							httpClient.Transport = &mockTransport
							sdMock.Cli = retryableHttpClient

							return &sdMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							return res
						}(),
					},
				},
			},

			args: args{
				ctx: context.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    secondExampleExecutor,
					Action:     string(entity.TaskUpdateActionChangeExecutor),
					Parameters: []byte(`{"decision":"` + ExecutionDecisionExecuted + `"}`),
				},
			},
			wantErr: true,
		},
		{
			name: "action TaskUpdateActionChangeExecutor take in work",
			fields: fields{
				Name: stepName,
				ExecutionData: &ExecutionData{
					IsTakenInWork:  true,
					ActualExecutor: &actualExecutor2,
					ExecutionType:  script.ExecutionTypeUser,
					Executors: map[string]struct{}{
						exampleExecutor: {},
					},
					WorkType: workType,
					SLA:      8,
				},
				RunContext: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						ServiceDesc: func() *servicedesc.Service {
							sdMock := servicedesc.Service{
								SdURL: "",
							}
							httpClient := http.DefaultClient
							retryableHttpClient := httpclient.NewClient(httpClient, nil, 0, 0)

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
							fError := func(*http.Request) error {
								return nil
							}
							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, fError)
							httpClient.Transport = &mockTransport
							sdMock.Cli = retryableHttpClient

							return &sdMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							return res
						}(),
					},
				},
			},

			args: args{
				ctx: context.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    exampleExecutor,
					Action:     string(entity.TaskUpdateActionChangeExecutor),
					Parameters: []byte(`{"decision":"` + ExecutionDecisionExecuted + `"}`),
				},
			},
			wantErr: false,
		},
		{
			name: "action TaskUpdateActionChangeExecutor not have executors",
			fields: fields{
				Name: stepName,
				ExecutionData: &ExecutionData{
					IsTakenInWork:  true,
					ActualExecutor: &actualExecutor2,
					ExecutionType:  script.ExecutionTypeUser,
					Executors:      map[string]struct{}{},
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
							retryableHttpClient := httpclient.NewClient(httpClient, nil, 0, 0)

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
							fError := func(*http.Request) error {
								return nil
							}
							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, fError)
							httpClient.Transport = &mockTransport
							sdMock.Cli = retryableHttpClient

							return &sdMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							return res
						}(),
					},
				},
			},

			args: args{
				ctx: context.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    exampleExecutor,
					Action:     string(entity.TaskUpdateActionChangeExecutor),
					Parameters: []byte(`{"decision":"` + ExecutionDecisionExecuted + `"}`),
				},
			},
			wantErr: true,
		},
		{
			name: "action TaskUpdateActionRequestExecutionInfo not take in work",
			fields: fields{
				Name: stepName,
				ExecutionData: &ExecutionData{
					IsTakenInWork: false,
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
							retryableHttpClient := httpclient.NewClient(httpClient, nil, 0, 0)

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
							fError := func(*http.Request) error {
								return nil
							}
							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, fError)
							httpClient.Transport = &mockTransport
							sdMock.Cli = retryableHttpClient

							return &sdMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							return res
						}(),
					},
				},
			},

			args: args{
				ctx: context.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    secondExampleExecutor,
					Action:     string(entity.TaskUpdateActionRequestExecutionInfo),
					Parameters: []byte(`{"decision":"` + ExecutionDecisionExecuted + `"}`),
				},
			},
			wantErr: true,
		},
		{
			name: "action TaskUpdateActionExecutorStartWork already work",
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
							retryableHttpClient := httpclient.NewClient(httpClient, nil, 0, 0)

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
							fError := func(*http.Request) error {
								return nil
							}
							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, fError)
							httpClient.Transport = &mockTransport
							sdMock.Cli = retryableHttpClient

							return &sdMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							return res
						}(),
					},
				},
			},

			args: args{
				ctx: context.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    secondExampleExecutor,
					Action:     string(entity.TaskUpdateActionExecutorStartWork),
					Parameters: []byte(`{"decision":"` + ExecutionDecisionExecuted + `"}`),
				},
			},
			wantErr: true,
		},
		{
			name: "action TaskUpdateActionExecutorStartWork not take in work",
			fields: fields{
				Name: stepName,
				ExecutionData: &ExecutionData{
					WorkType:      workType,
					IsTakenInWork: false,
					ExecutionType: script.ExecutionTypeUser,
					Executors: map[string]struct{}{
						secondExampleExecutor: {},
					},
				},
				RunContext: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						ServiceDesc: func() *servicedesc.Service {
							sdMock := servicedesc.Service{
								SdURL: "",
							}
							httpClient := http.DefaultClient
							retryableHttpClient := httpclient.NewClient(httpClient, nil, 0, 0)

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
							fError := func(*http.Request) error {
								return nil
							}
							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, fError)
							httpClient.Transport = &mockTransport
							sdMock.Cli = retryableHttpClient

							return &sdMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							return res
						}(),
					},
				},
			},

			args: args{
				ctx: context.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    secondExampleExecutor,
					Action:     string(entity.TaskUpdateActionExecutorStartWork),
					Parameters: []byte(`{"decision":"` + ExecutionDecisionExecuted + `"}`),
				},
			},
			wantErr: false,
		},
		{
			name: "action TaskUpdateActionExecutorSendEditApp not take in work",
			fields: fields{
				Name: stepName,
				ExecutionData: &ExecutionData{
					IsTakenInWork: false,
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
							retryableHttpClient := httpclient.NewClient(httpClient, nil, 0, 0)

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
							fError := func(*http.Request) error {
								return nil
							}
							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, fError)
							httpClient.Transport = &mockTransport
							sdMock.Cli = retryableHttpClient

							return &sdMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							return res
						}(),
					},
				},
			},

			args: args{
				ctx: context.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    secondExampleExecutor,
					Action:     string(entity.TaskUpdateActionExecutorSendEditApp),
					Parameters: []byte(`{"decision":"` + ExecutionDecisionSentEdit + `"}`),
				},
			},
			wantErr: true,
		},
		{
			name: "action TaskUpdateActionExecutorSendEditApp take in work",
			fields: fields{
				Name: stepName,
				ExecutionData: &ExecutionData{
					IsTakenInWork: true,
					ExecutionType: script.ExecutionTypeUser,
					Executors: map[string]struct{}{
						secondExampleExecutor: {},
					},
					WorkType: workType,
					SLA:      8,
				},
				RunContext: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						ServiceDesc: func() *servicedesc.Service {
							sdMock := servicedesc.Service{
								SdURL: "",
							}
							httpClient := http.DefaultClient
							retryableHttpClient := httpclient.NewClient(httpClient, nil, 0, 0)

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
							fError := func(*http.Request) error {
								return nil
							}
							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, fError)
							httpClient.Transport = &mockTransport
							sdMock.Cli = retryableHttpClient

							return &sdMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							return res
						}(),
					},
				},
			},

			args: args{
				ctx: context.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    secondExampleExecutor,
					Action:     string(entity.TaskUpdateActionExecutorSendEditApp),
					Parameters: []byte(`{"decision":"` + ExecutionDecisionSentEdit + `"}`),
				},
			},
			wantErr: false,
		},
		{
			name: "action TaskUpdateActionExecutorSendEditApp set decision ",
			fields: fields{
				Name: stepName,
				ExecutionData: &ExecutionData{
					IsTakenInWork: true,
					ExecutionType: script.ExecutionTypeUser,
					Executors: map[string]struct{}{
						secondExampleExecutor: {},
					},
					Decision: func() *ExecutionDecision {
						res := ExecutionDecisionSentEdit

						return &res
					}(),
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
							retryableHttpClient := httpclient.NewClient(httpClient, nil, 0, 0)

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
							fError := func(*http.Request) error {
								return nil
							}
							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, fError)
							httpClient.Transport = &mockTransport
							sdMock.Cli = retryableHttpClient

							return &sdMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("UpdateTaskStatus",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								uuid.UUID{},
								mock.MatchedBy(func(taskStatus int) bool { return true }),
								mock.MatchedBy(func(comment string) bool { return true }),
								mock.MatchedBy(func(author string) bool { return true }),
							).Return(nil)

							res.On("StopTaskBlocks",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								mock.MatchedBy(func(id uuid.UUID) bool { return true }),
							).Return(nil)

							res.On("SendTaskToArchive",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								mock.MatchedBy(func(id uuid.UUID) bool { return true }),
							).Return(nil)

							return res
						}(),
					},
				},
			},

			args: args{
				ctx: context.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    secondExampleExecutor,
					Action:     string(entity.TaskUpdateActionExecutorSendEditApp),
					Parameters: []byte(`{"decision":"` + ExecutionDecisionSentEdit + `"}`),
				},
			},
			wantErr: true,
		},
		{
			name: "action TaskUpdateActionDayBeforeSLARequestAddInfo false check",
			fields: fields{
				Name: stepName,
				ExecutionData: &ExecutionData{
					ExecutionType: script.ExecutionTypeUser,
					Executors: map[string]struct{}{
						secondExampleExecutor: {},
					},
					CheckDayBeforeSLARequestInfo: false,
					WorkType:                     workType,
					SLA:                          8,
				},
				RunContext: &BlockRunContext{
					skipNotifications: true,
					Initiator:         secondExampleExecutor,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						People: func() *nocache.Service {
							return &nocache.Service{}
						}(),
						ServiceDesc: func() *servicedesc.Service {
							sdMock := servicedesc.Service{
								SdURL: "",
							}
							httpClient := http.DefaultClient
							retryableHttpClient := httpclient.NewClient(httpClient, nil, 0, 0)

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
							fError := func(*http.Request) error {
								return nil
							}
							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, fError)
							httpClient.Transport = &mockTransport
							sdMock.Cli = retryableHttpClient

							return &sdMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							return res
						}(),
					},
				},
			},

			args: args{
				ctx: context.Background(),
				data: &script.BlockUpdateData{
					ByLogin:    secondExampleExecutor,
					Action:     string(entity.TaskUpdateActionDayBeforeSLARequestAddInfo),
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
					WorkType: workType,
					SLA:      8,
				},
				RunContext: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						ServiceDesc: func() *servicedesc.Service {
							sdMock := servicedesc.Service{
								SdURL: "",
							}
							httpClient := http.DefaultClient
							retryableHttpClient := httpclient.NewClient(httpClient, nil, 0, 0)

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
							fError := func(*http.Request) error {
								return nil
							}
							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, fError)
							httpClient.Transport = &mockTransport
							sdMock.Cli = retryableHttpClient

							return &sdMock
						}(),
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							res.On("GetTaskStepById",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								stepID,
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

func TestGoExecutionActions(t *testing.T) {
	const (
		exampleExecutor = "example"
		stepName        = "exec"
	)

	login := "user1"
	delLogin1 := "delLogin1"

	type (
		fields struct {
			Name          string
			Title         string
			Input         map[string]string
			Output        map[string]string
			NextStep      []script.Socket
			ExecutionData *ExecutionData
			RunContext    *BlockRunContext
		}
		args struct {
			ctx  context.Context
			data *script.BlockUpdateData
		}
	)

	tests := []struct {
		name        string
		fields      fields
		args        args
		wantActions []MemberAction
	}{
		{
			name: "empty form accessibility",
			fields: fields{
				ExecutionData: &ExecutionData{},
				Name:          stepName,
				RunContext: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						Storage: nil,
					},
				},
			},
			args: args{
				ctx:  context.Background(),
				data: nil,
			},
			wantActions: []MemberAction{
				{ID: "executor_start_work", Type: "primary", Params: map[string]interface{}(nil)},
			},
		},
		{
			name: "one form ReadWrite",
			fields: fields{
				Name: stepName,
				ExecutionData: &ExecutionData{
					IsTakenInWork: true,
					ExecutionType: script.ExecutionTypeUser,
					Executors: map[string]struct{}{
						exampleExecutor: {},
					},
					FormsAccessibility: []script.FormAccessibility{
						{
							Name:        "Форма",
							NodeID:      "form_0",
							AccessType:  "ReadWrite",
							Description: "форма",
						},
					},
				},
				RunContext: &BlockRunContext{
					skipNotifications: false,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						s.State = map[string]json.RawMessage{
							"form_0": []byte{},
						}

						return s
					}(),
					Services: RunContextServices{
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

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
			wantActions: []MemberAction{
				{ID: "execution", Type: "primary", Params: map[string]interface{}(nil)},
				{ID: "decline", Type: "secondary", Params: map[string]interface{}(nil)},
				{ID: "change_executor", Type: "other", Params: map[string]interface{}(nil)},
				{ID: "request_execution_info", Type: "other", Params: map[string]interface{}(nil)},
				{ID: "fill_form", Type: "custom", Params: map[string]interface{}{"form_name": []string{"form_0"}}},
			},
		},
		{
			name: "two form (ReadWrite)",
			fields: fields{
				Name: stepName,
				ExecutionData: &ExecutionData{
					IsTakenInWork: true,
					ExecutionType: script.ExecutionTypeUser,
					Executors: map[string]struct{}{
						exampleExecutor: {},
					},
					FormsAccessibility: []script.FormAccessibility{
						{
							Name:        "Форма",
							NodeID:      "form_0",
							AccessType:  "ReadWrite",
							Description: "форма",
						},
						{
							Name:        "Форма",
							NodeID:      "form_1",
							AccessType:  "ReadWrite",
							Description: "форма",
						},
					},
				},
				RunContext: &BlockRunContext{
					skipNotifications: false,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						s.State = map[string]json.RawMessage{
							"form_0": []byte{},
							"form_1": []byte{},
						}

						return s
					}(),
					Services: RunContextServices{
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

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
			wantActions: []MemberAction{
				{ID: "execution", Type: "primary", Params: map[string]interface{}(nil)},
				{ID: "decline", Type: "secondary", Params: map[string]interface{}(nil)},
				{ID: "change_executor", Type: "other", Params: map[string]interface{}(nil)},
				{ID: "request_execution_info", Type: "other", Params: map[string]interface{}(nil)},
				{ID: "fill_form", Type: "custom", Params: map[string]interface{}{"form_name": []string{"form_0", "form_1"}}},
			},
		},
		{
			name: "Two form is filled true - ok (ReadWrite & RequiredFill)",
			fields: fields{
				Name: stepName,
				ExecutionData: &ExecutionData{
					IsTakenInWork: true,
					ExecutionType: script.ExecutionTypeUser,
					Executors: map[string]struct{}{
						login: {},
					},
					FormsAccessibility: []script.FormAccessibility{
						{
							Name:        "Форма",
							NodeID:      "form_0",
							AccessType:  "ReadWrite",
							Description: "форма",
						},
						{
							Name:        "Форма",
							NodeID:      "form_1",
							AccessType:  "RequiredFill",
							Description: "форма",
						},
					},
				},
				RunContext: &BlockRunContext{
					skipNotifications: false,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						s.State = map[string]json.RawMessage{
							"form_0": []byte{},
							"form_1": func() []byte {
								marshalForm, _ := json.Marshal(FormData{
									IsFilled: true,
									Executors: map[string]struct{}{
										login: {},
									},
									ActualExecutor: &login,
									ChangesLog: []ChangesLogItem{
										{
											Executor: login,
										},
									},
								})

								return marshalForm
							}(),
						}
						return s
					}(),
					Services: RunContextServices{
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							return res
						}(),
						HumanTasks: func() *humanTasks.Service {
							ht := humanTasks.Service{}
							htMock := mocks2.DelegationServiceClient{}

							htMock.On("GetDelegationsFromLogin", context.Background(), "users1").Return(nil, humanTasks.Delegations{})

							req := &delegationht.GetDelegationsRequest{
								FilterBy:  "fromLogin",
								FromLogin: login,
							}

							htMock.On("getDelegationsInternal", context.Background(), req).Return(humanTasks.Delegations{
								{
									ToLogin:   delLogin1,
									FromLogin: login,
								},
							}, nil)
							htMock.On("FilterByType", "users1").Return(delegationht.GetDelegationsResponse{
								Delegations: []*delegationht.Delegation{
									{
										FromUser: &delegationht.User{
											Fullname: login,
										},
									},
								},
							})
							htMock.On("GetDelegates", "users1").Return([]string{"a"})

							ht = humanTasks.Service{
								Cli: &htMock,
								C:   nil,
							}

							return &ht
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
			wantActions: []MemberAction{
				{ID: "execution", Type: "primary", Params: map[string]interface{}(nil)},
				{ID: "decline", Type: "secondary", Params: map[string]interface{}(nil)},
				{ID: "change_executor", Type: "other", Params: map[string]interface{}(nil)},
				{ID: "request_execution_info", Type: "other", Params: map[string]interface{}(nil)},
				{ID: "fill_form", Type: "custom", Params: map[string]interface{}{"form_name": []string{"form_0", "form_1"}}},
			},
		},
		{
			name: "Two form is filled true - bad (ReadWrite & RequiredFill)",
			fields: fields{
				Name: stepName,
				ExecutionData: &ExecutionData{
					IsTakenInWork: true,
					ExecutionType: script.ExecutionTypeUser,
					Executors: map[string]struct{}{
						login: {},
					},
					FormsAccessibility: []script.FormAccessibility{
						{
							Name:        "Форма",
							NodeID:      "form_0",
							AccessType:  "ReadWrite",
							Description: "форма",
						},
						{
							Name:        "Форма",
							NodeID:      "form_1",
							AccessType:  "RequiredFill",
							Description: "форма",
						},
					},
				},
				RunContext: &BlockRunContext{
					skipNotifications: false,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						s.State = map[string]json.RawMessage{
							"form_0": []byte{},
							"form_1": func() []byte {
								marshalForm, _ := json.Marshal(FormData{
									IsFilled: true,
									Executors: map[string]struct{}{
										login: {},
									},
									ActualExecutor: &login,
									ChangesLog:     []ChangesLogItem{},
								})

								return marshalForm
							}(),
						}
						return s
					}(),
					Services: RunContextServices{
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							return res
						}(),
						HumanTasks: func() *humanTasks.Service {
							ht := humanTasks.Service{}
							htMock := mocks2.DelegationServiceClient{}

							htMock.On("GetDelegationsFromLogin", context.Background(), "users1").Return(nil, humanTasks.Delegations{})

							req := &delegationht.GetDelegationsRequest{
								FilterBy:  "fromLogin",
								FromLogin: login,
							}

							htMock.On("getDelegationsInternal", context.Background(), req).Return(humanTasks.Delegations{
								{
									ToLogin:   delLogin1,
									FromLogin: login,
								},
							}, nil)
							htMock.On("FilterByType", "users1").Return(delegationht.GetDelegationsResponse{
								Delegations: []*delegationht.Delegation{
									{
										FromUser: &delegationht.User{
											Fullname: login,
										},
									},
								},
							})
							htMock.On("GetDelegates", "users1").Return([]string{"a"})

							ht = humanTasks.Service{
								Cli: &htMock,
								C:   nil,
							}

							return &ht
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
			wantActions: []MemberAction{
				{ID: "execution", Type: "primary", Params: map[string]interface{}{"disabled": true, "hint_description": "Для продолжения работы над заявкой, необходимо {fill_form}"}},
				{ID: "decline", Type: "secondary", Params: map[string]interface{}(nil)},
				{ID: "change_executor", Type: "other", Params: map[string]interface{}(nil)},
				{ID: "request_execution_info", Type: "other", Params: map[string]interface{}(nil)},
				{ID: "fill_form", Type: "custom", Params: map[string]interface{}{"form_name": []string{"form_0", "form_1"}}},
			},
		},
		{
			name: "Two form - is filled false (ReadWrite & RequiredFill)",
			fields: fields{
				Name: stepName,
				ExecutionData: &ExecutionData{
					IsTakenInWork: true,
					ExecutionType: script.ExecutionTypeUser,
					Executors: map[string]struct{}{
						exampleExecutor: {},
					},
					FormsAccessibility: []script.FormAccessibility{
						{
							Name:        "Форма",
							NodeID:      "form_0",
							AccessType:  "ReadWrite",
							Description: "форма",
						},
						{
							Name:        "Форма",
							NodeID:      "form_1",
							AccessType:  "RequiredFill",
							Description: "форма",
						},
					},
				},
				RunContext: &BlockRunContext{
					skipNotifications: false,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						s.State = map[string]json.RawMessage{
							"form_0": []byte{},
							"form_1": func() []byte {
								marshalForm, _ := json.Marshal(FormData{
									IsFilled:       false,
									ActualExecutor: &login,
								})

								return marshalForm
							}(),
						}
						return s
					}(),
					Services: RunContextServices{
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							return res
						}(),
						HumanTasks: func() *humanTasks.Service {
							ht := humanTasks.Service{}
							htMock := mocks2.DelegationServiceClient{}

							htMock.On("GetDelegationsFromLogin", context.Background(), "users1").Return(nil, humanTasks.Delegations{})

							req := &delegationht.GetDelegationsRequest{
								FilterBy:  "fromLogin",
								FromLogin: login,
							}

							htMock.On("getDelegationsInternal", context.Background(), req).Return(humanTasks.Delegations{
								{
									ToLogin:   delLogin1,
									FromLogin: login,
								},
							}, nil)
							htMock.On("FilterByType", "users1").Return(delegationht.GetDelegationsResponse{
								Delegations: []*delegationht.Delegation{
									{
										FromUser: &delegationht.User{
											Fullname: login,
										},
									},
								},
							})
							htMock.On("GetDelegates", "users1").Return([]string{"a"})

							ht = humanTasks.Service{
								Cli: &htMock,
								C:   nil,
							}

							return &ht
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
			wantActions: []MemberAction{
				{ID: "execution", Type: "primary", Params: map[string]interface{}{"disabled": true, "hint_description": "Для продолжения работы над заявкой, необходимо {fill_form}"}},
				{ID: "decline", Type: "secondary", Params: map[string]interface{}(nil)},
				{ID: "change_executor", Type: "other", Params: map[string]interface{}(nil)},
				{ID: "request_execution_info", Type: "other", Params: map[string]interface{}(nil)},
				{ID: "fill_form", Type: "custom", Params: map[string]interface{}{"form_name": []string{"form_0", "form_1"}}},
			},
		},
		{
			name: "Two form is filled (RequiredFill)",
			fields: fields{
				Name: stepName,
				ExecutionData: &ExecutionData{
					IsTakenInWork: true,
					ExecutionType: script.ExecutionTypeUser,
					Executors: map[string]struct{}{
						login: {},
					},
					FormsAccessibility: []script.FormAccessibility{
						{
							Name:        "Форма",
							NodeID:      "form_0",
							AccessType:  "RequiredFill",
							Description: "форма",
						},
						{
							Name:        "Форма",
							NodeID:      "form_1",
							AccessType:  "RequiredFill",
							Description: "форма",
						},
					},
				},
				RunContext: &BlockRunContext{
					skipNotifications: false,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						s.State = map[string]json.RawMessage{
							"form_0": func() []byte {
								marshalForm, _ := json.Marshal(FormData{
									IsFilled: true,
									Executors: map[string]struct{}{
										login: {},
									},
									ActualExecutor: &delLogin1,
									ChangesLog: []ChangesLogItem{
										{
											Executor: login,
										},
									},
								})

								return marshalForm
							}(),
							"form_1": func() []byte {
								marshalForm, _ := json.Marshal(FormData{
									IsFilled: true,
									Executors: map[string]struct{}{
										login: {},
									},
									ActualExecutor: &login,
									ChangesLog: []ChangesLogItem{
										{
											Executor: login,
										},
									},
								})

								return marshalForm
							}(),
						}
						return s
					}(),
					Services: RunContextServices{
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							return res
						}(),
						HumanTasks: func() *humanTasks.Service {
							ht := humanTasks.Service{}
							htMock := mocks2.DelegationServiceClient{}

							htMock.On("GetDelegationsFromLogin", context.Background(), "users1").Return(nil, humanTasks.Delegations{})

							req := &delegationht.GetDelegationsRequest{
								FilterBy:  "fromLogin",
								FromLogin: login,
							}

							htMock.On("getDelegationsInternal", context.Background(), req).Return(humanTasks.Delegations{
								{
									ToLogin:   delLogin1,
									FromLogin: login,
								},
							}, nil)
							htMock.On("FilterByType", "users1").Return(delegationht.GetDelegationsResponse{
								Delegations: []*delegationht.Delegation{
									{
										FromUser: &delegationht.User{
											Fullname: login,
										},
									},
								},
							})
							htMock.On("GetDelegates", "users1").Return([]string{"a"})

							ht = humanTasks.Service{
								Cli: &htMock,
								C:   nil,
							}

							return &ht
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
			wantActions: []MemberAction{
				{ID: "execution", Type: "primary", Params: map[string]interface{}(nil)},
				{ID: "decline", Type: "secondary", Params: map[string]interface{}(nil)},
				{ID: "change_executor", Type: "other", Params: map[string]interface{}(nil)},
				{ID: "request_execution_info", Type: "other", Params: map[string]interface{}(nil)},
				{ID: "fill_form", Type: "custom", Params: map[string]interface{}{"form_name": []string{"form_0", "form_1"}}},
			},
		},
		{
			name: "Two form is filled and not filled (RequiredFill)",
			fields: fields{
				Name: stepName,
				ExecutionData: &ExecutionData{
					IsTakenInWork: true,
					ExecutionType: script.ExecutionTypeUser,
					Executors: map[string]struct{}{
						login: {},
					},
					FormsAccessibility: []script.FormAccessibility{
						{
							Name:        "Форма",
							NodeID:      "form_0",
							AccessType:  "RequiredFill",
							Description: "форма",
						},
						{
							Name:        "Форма",
							NodeID:      "form_1",
							AccessType:  "RequiredFill",
							Description: "форма",
						},
					},
				},
				RunContext: &BlockRunContext{
					skipNotifications: false,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						s.State = map[string]json.RawMessage{
							"form_0": func() []byte {
								marshalForm, _ := json.Marshal(FormData{
									IsFilled: true,
									Executors: map[string]struct{}{
										login: {},
									},
									ActualExecutor: &delLogin1,
									ChangesLog: []ChangesLogItem{
										{
											Executor: login,
										},
									},
								})

								return marshalForm
							}(),
							"form_1": func() []byte {
								marshalForm, _ := json.Marshal(FormData{
									IsFilled: false,
									Executors: map[string]struct{}{
										"user1": {},
									},
									ActualExecutor: &login,
								})

								return marshalForm
							}(),
						}
						return s
					}(),
					Services: RunContextServices{
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							return res
						}(),
						HumanTasks: func() *humanTasks.Service {
							ht := humanTasks.Service{}
							htMock := mocks2.DelegationServiceClient{}

							htMock.On("GetDelegationsFromLogin", context.Background(), "users1").Return(nil, humanTasks.Delegations{})

							req := &delegationht.GetDelegationsRequest{
								FilterBy:  "fromLogin",
								FromLogin: login,
							}

							htMock.On("getDelegationsInternal", context.Background(), req).Return(humanTasks.Delegations{
								{
									ToLogin:   delLogin1,
									FromLogin: login,
								},
							}, nil)
							htMock.On("FilterByType", "users1").Return(delegationht.GetDelegationsResponse{
								Delegations: []*delegationht.Delegation{
									{
										FromUser: &delegationht.User{
											Fullname: login,
										},
									},
								},
							})
							htMock.On("GetDelegates", "users1").Return([]string{"a"})

							ht = humanTasks.Service{
								Cli: &htMock,
								C:   nil,
							}

							return &ht
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
			wantActions: []MemberAction{
				{ID: "execution", Type: "primary", Params: map[string]interface{}{"disabled": true, "hint_description": "Для продолжения работы над заявкой, необходимо {fill_form}"}},
				{ID: "decline", Type: "secondary", Params: map[string]interface{}(nil)},
				{ID: "change_executor", Type: "other", Params: map[string]interface{}(nil)},
				{ID: "request_execution_info", Type: "other", Params: map[string]interface{}(nil)},
				{ID: "fill_form", Type: "custom", Params: map[string]interface{}{"form_name": []string{"form_0", "form_1"}}},
			},
		},
		{
			name: "Two form - not filled (RequiredFill)",
			fields: fields{
				Name: stepName,
				ExecutionData: &ExecutionData{
					IsTakenInWork: true,
					ExecutionType: script.ExecutionTypeUser,
					Executors: map[string]struct{}{
						exampleExecutor: {},
					},
					FormsAccessibility: []script.FormAccessibility{
						{
							Name:        "Форма",
							NodeID:      "form_0",
							AccessType:  "RequiredFill",
							Description: "форма",
						},
						{
							Name:        "Форма",
							NodeID:      "form_1",
							AccessType:  "RequiredFill",
							Description: "форма",
						},
					},
				},
				RunContext: &BlockRunContext{
					skipNotifications: false,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						s.State = map[string]json.RawMessage{
							"form_0": func() []byte {
								marshalForm, _ := json.Marshal(FormData{
									IsFilled: false,
									Executors: map[string]struct{}{
										"user1": {},
									},
									ActualExecutor: &delLogin1,
								})

								return marshalForm
							}(),
							"form_1": func() []byte {
								marshalForm, _ := json.Marshal(FormData{
									IsFilled: false,
									Executors: map[string]struct{}{
										"user1": {},
									},
									ActualExecutor: &login,
								})

								return marshalForm
							}(),
						}
						return s
					}(),
					Services: RunContextServices{
						Storage: func() db.Database {
							res := &mocks.MockedDatabase{}

							return res
						}(),
						HumanTasks: func() *humanTasks.Service {
							ht := humanTasks.Service{}
							htMock := mocks2.DelegationServiceClient{}

							htMock.On("GetDelegationsFromLogin", context.Background(), "users1").Return(nil, humanTasks.Delegations{})

							req := &delegationht.GetDelegationsRequest{
								FilterBy:  "fromLogin",
								FromLogin: login,
							}

							htMock.On("getDelegationsInternal", context.Background(), req).Return(humanTasks.Delegations{
								{
									ToLogin:   delLogin1,
									FromLogin: login,
								},
							}, nil)
							htMock.On("FilterByType", "users1").Return(delegationht.GetDelegationsResponse{
								Delegations: []*delegationht.Delegation{
									{
										FromUser: &delegationht.User{
											Fullname: login,
										},
									},
								},
							})
							htMock.On("GetDelegates", "users1").Return([]string{"a"})

							ht = humanTasks.Service{
								Cli: &htMock,
								C:   nil,
							}

							return &ht
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
			wantActions: []MemberAction{
				{ID: "execution", Type: "primary", Params: map[string]interface{}{"disabled": true, "hint_description": "Для продолжения работы над заявкой, необходимо {fill_form}"}},
				{ID: "decline", Type: "secondary", Params: map[string]interface{}(nil)},
				{ID: "change_executor", Type: "other", Params: map[string]interface{}(nil)},
				{ID: "request_execution_info", Type: "other", Params: map[string]interface{}(nil)},
				{ID: "fill_form", Type: "custom", Params: map[string]interface{}{"form_name": []string{"form_0", "form_1"}}},
			},
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

			actions := gb.executionActions()
			assert.Equal(t, tt.wantActions, actions, fmt.Sprintf("executionActions(%v)", login))
		})
	}
}
