package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	delegationht "gitlab.services.mts.ru/jocasta/human-tasks/pkg/proto/gen/proto/go/delegation"

	dbMocks "gitlab.services.mts.ru/jocasta/pipeliner/internal/db/mocks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	humanTasks "gitlab.services.mts.ru/jocasta/pipeliner/internal/human-tasks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/human-tasks/mocks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc"
	serviceDeskMocks "gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc/mocks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

func Test_createGoFormBlock(t *testing.T) {
	const (
		name       = "form_0"
		title      = "Форма"
		global1    = "form_0.executor"
		global2    = "form_0.application_body"
		schemaId   = "c77be97a-f978-46d3-aa03-ab72663f2b74"
		versionId  = "d77be97a-f978-46d3-aa03-ab72663f2b74"
		schemaName = "название формы"
		executor   = "executor"
		workNumber = "J0000001"
		workType   = "8/5"
	)

	timeNow := time.Now()

	next := []entity.Socket{
		{
			Id:           DefaultSocketID,
			Title:        script.DefaultSocketTitle,
			NextBlockIds: []string{"next"},
		},
	}

	ctx := context.Background()
	databaseMock := dbMocks.NewMockedDatabase(t)
	vid, _ := uuid.Parse(versionId)
	databaseMock.On("GetVersionByWorkNumber", ctx, workNumber).
		Return(&entity.EriusScenario{VersionID: vid}, error(nil))
	databaseMock.On("GetSlaVersionSettings", ctx, vid.String()).
		Return(entity.SlaVersionSettings{WorkType: workType}, error(nil))

	type args struct {
		name   string
		ef     *entity.EriusFunc
		runCtx *BlockRunContext
	}
	tests := []struct {
		name    string
		args    args
		want    *GoFormBlock
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "can't get form parameters",
			args: args{
				name: name,
				ef: &entity.EriusFunc{
					BlockType: BlockGoFormID,
					Title:     title,
					Input:     nil,
					Output:    nil,
					Params:    nil,
					Sockets:   next,
				},
				runCtx: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
				},
			},
			want:    nil,
			wantErr: assert.Error,
		},
		{
			name: "invalid form parameters",
			args: args{
				name: name,
				ef: &entity.EriusFunc{
					BlockType: BlockGoFormID,
					Title:     title,
					Input:     nil,
					Output:    nil,
					Params:    []byte("{}"),
					Sockets:   next,
				},
				runCtx: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
				},
			},
			want:    nil,
			wantErr: assert.Error,
		},
		{
			name: "load state error",
			args: args{
				name: name,
				ef: &entity.EriusFunc{
					BlockType: BlockGoFormID,
					Title:     title,
					Input: []entity.EriusFunctionValue{
						{
							Name:   "foo",
							Type:   "string",
							Global: "bar",
						},
					},
					Output: []entity.EriusFunctionValue{
						{
							Name:   keyOutputFormExecutor,
							Type:   "string",
							Global: global1,
						},
						{
							Name:   keyOutputFormBody,
							Type:   "string",
							Global: global2,
						},
					},
					Params: func() []byte {
						r, _ := json.Marshal(&script.FormParams{
							SchemaId:         schemaId,
							SchemaName:       schemaName,
							Executor:         executor,
							FormExecutorType: script.FormExecutorTypeFromSchema,
						})

						return r
					}(),
					Sockets: next,
				},
				runCtx: &BlockRunContext{
					skipNotifications: true,
					VarStore: &store.VariableStore{
						State: map[string]json.RawMessage{
							name: {},
						},
					},
				},
			},
			want:    nil,
			wantErr: assert.Error,
		},
		{
			name: "success case",
			args: args{
				name: name,
				ef: &entity.EriusFunc{
					BlockType: BlockGoFormID,
					Title:     title,
					Input: []entity.EriusFunctionValue{
						{
							Name:   "foo",
							Type:   "string",
							Global: "bar",
						},
					},
					Output: []entity.EriusFunctionValue{
						{
							Name:   keyOutputFormExecutor,
							Type:   "string",
							Global: global1,
						},
						{
							Name:   keyOutputFormBody,
							Type:   "string",
							Global: global2,
						},
					},
					Params: func() []byte {
						r, _ := json.Marshal(&script.FormParams{
							SchemaId:         schemaId,
							SchemaName:       schemaName,
							Executor:         "form.executor",
							FormExecutorType: script.FormExecutorTypeFromSchema,
						})

						return r
					}(),
					Sockets: next,
				},
				runCtx: &BlockRunContext{
					WorkNumber:        workNumber,
					Storage:           databaseMock,
					skipNotifications: true,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						s.SetValue("form.executor", executor)
						return s
					}(),
				},
			},
			want: &GoFormBlock{
				Name:  name,
				Title: title,
				Input: map[string]string{
					"foo": "bar",
				},
				Output: map[string]string{
					keyOutputFormExecutor: global1,
					keyOutputFormBody:     global2,
				},
				State: &FormData{
					FormExecutorType:   script.FormExecutorTypeFromSchema,
					SchemaId:           schemaId,
					SchemaName:         schemaName,
					Executors:          map[string]struct{}{executor: {}},
					ApplicationBody:    map[string]interface{}{},
					IsFilled:           false,
					ActualExecutor:     nil,
					ChangesLog:         []ChangesLogItem{},
					Description:        "",
					FormsAccessibility: nil,
					WorkType:           workType,
					IsTakenInWork:      true,
				},
				Sockets: entity.ConvertSocket(next),
			},
			wantErr: assert.NoError,
		},
		{
			name: "success_auto_fill",
			args: args{
				name: name,
				ef: &entity.EriusFunc{
					BlockType: BlockGoFormID,
					Title:     title,
					Input: []entity.EriusFunctionValue{
						{
							Name:   "foo",
							Type:   "string",
							Global: "bar",
						},
					},
					Output: []entity.EriusFunctionValue{
						{
							Name:   keyOutputFormExecutor,
							Type:   "string",
							Global: global1,
						},
						{
							Name:   keyOutputFormBody,
							Type:   "string",
							Global: global2,
						},
					},
					Params: func() []byte {
						r, _ := json.Marshal(&script.FormParams{
							SchemaId:         schemaId,
							SchemaName:       schemaName,
							Executor:         executor,
							FormExecutorType: script.FormExecutorTypeAutoFillUser,
							Mapping: script.JSONSchemaProperties{
								"a": script.JSONSchemaPropertiesValue{
									Type:  "number",
									Value: "sd.form_0.a",
								},
								"b": script.JSONSchemaPropertiesValue{
									Type:  "number",
									Value: "sd.form_0.b",
								},
							},
							WorkType: utils.GetAddressOfValue("8/5"),
						})

						return r
					}(),
					Sockets: next,
				},
				runCtx: &BlockRunContext{
					skipNotifications: true,
					VarStore: func() *store.VariableStore {
						s := store.NewStore()
						s.SetValue("sd.form_0", map[string]interface{}{
							"a": 100,
							"b": 200,
						})
						return s
					}(),
				},
			},
			want: &GoFormBlock{
				Name:  name,
				Title: title,
				Input: map[string]string{
					"foo": "bar",
				},
				Output: map[string]string{
					keyOutputFormExecutor: global1,
					keyOutputFormBody:     global2,
				},
				State: &FormData{
					FormExecutorType: script.FormExecutorTypeAutoFillUser,
					SchemaId:         schemaId,
					SchemaName:       schemaName,
					Executors:        map[string]struct{}{"auto_fill": {}},
					ApplicationBody: map[string]interface{}{
						"a": 100,
						"b": 200,
					},
					WorkType:      "8/5",
					IsFilled:      true,
					IsTakenInWork: true,
					Mapping: script.JSONSchemaProperties{
						"a": script.JSONSchemaPropertiesValue{
							Type:  "number",
							Value: "sd.form_0.a",
						},
						"b": script.JSONSchemaPropertiesValue{
							Type:  "number",
							Value: "sd.form_0.b",
						},
					},
					ActualExecutor: func(s string) *string {
						return &s
					}("auto_fill"),
					ChangesLog: []ChangesLogItem{
						{
							ApplicationBody: map[string]interface{}{
								"a": 100,
								"b": 200,
							},
							CreatedAt:   timeNow,
							Executor:    "auto_fill",
							DelegateFor: "",
						},
					},
					Description:        "",
					FormsAccessibility: nil,
				},
				Sockets: entity.ConvertSocket(next),
			},
			wantErr: assert.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			cli := mocks.DelegationServiceClient{}
			cli.On("GetDelegations", mock.Anything, mock.Anything).Return(&delegationht.GetDelegationsResponse{
				Delegations: []*delegationht.Delegation{},
			}, nil)
			tt.args.runCtx.HumanTasks = &humanTasks.Service{
				C:   nil,
				Cli: &cli,
			}

			got, _, err := createGoFormBlock(ctx, tt.args.name, tt.args.ef, tt.args.runCtx)
			if got != nil {
				got.RunContext = nil
				if got.State != nil && len(got.State.ChangesLog) > 0 {
					got.State.ChangesLog[0].CreatedAt = timeNow
				}
			}

			if !tt.wantErr(t, err, "createGoFormBlock(%v, %v, %v)", tt.args.name, tt.args.ef, nil) {
				return
			}

			assert.Equalf(t, tt.want, got, "createGoFormBlock(%v, %v, %v)", tt.args.name, tt.args.ef, nil)
		})
	}
}

func TestGoFormBlock_Update(t *testing.T) {
	const (
		name        = "form_0"
		title       = "Форма"
		global1     = "form_0.executor"
		global2     = "form_0.application_body"
		schemaId    = "c77be97a-f978-46d3-aa03-ab72663f2b74"
		schemaName  = "название формы"
		login       = "login"
		login2      = "login2"
		login3      = "login3"
		blockId     = "form_0"
		blockId2    = "servicedesk_application_0"
		description = "description"
		fieldName   = "field1"
		fieldValue  = "some text"
		newValue    = "some new text"
	)

	timeNow := time.Now()
	taskId1 := uuid.New()

	next := []entity.Socket{
		{
			Id:           DefaultSocketID,
			Title:        script.DefaultSocketTitle,
			NextBlockIds: []string{"next"},
		},
	}

	ctx := context.Background()
	mockedDb := &dbMocks.MockedDatabase{}

	mockedDb.On("CheckUserCanEditForm",
		mock.MatchedBy(func(ctx context.Context) bool { return true }),
		mock.MatchedBy(func(string) bool { return true }),
		mock.MatchedBy(func(string) bool { return true }),
		mock.MatchedBy(func(string) bool { return true }),
	).Return(false, nil)
	currCall := mockedDb.ExpectedCalls[len(mockedDb.ExpectedCalls)-1]
	currCall = currCall.Run(func(args mock.Arguments) {
		switch args.Get(3).(string) {
		case login:
			currCall.ReturnArguments[0] = true
			currCall.ReturnArguments[1] = nil
		case login2:
			currCall.ReturnArguments[0] = false
			currCall.ReturnArguments[1] = nil
		case login3:
			currCall.ReturnArguments[0] = false
			currCall.ReturnArguments[1] = errors.New("mock error")
		}

	})

	type args struct {
		Name       string
		Title      string
		Input      map[string]string
		Output     map[string]string
		Sockets    []script.Socket
		State      *FormData
		RunContext *BlockRunContext
	}
	type ServiceDeskHttpTransportMockDataStruct struct {
		Status     string
		StatusCode int
		Body       any
	}
	type mockDataStruct struct {
		ServiceDeskHttpTransportMockData *ServiceDeskHttpTransportMockDataStruct
	}

	tests := []struct {
		name      string
		args      args
		mockData  *mockDataStruct
		want      interface{}
		wantErr   assert.ErrorAssertionFunc
		wantState *FormData
	}{
		{
			name: "empty data error",
			args: args{
				Name:   name,
				Title:  title,
				Input:  map[string]string{},
				Output: map[string]string{},
				RunContext: &BlockRunContext{
					UpdateData: nil,
					VarStore:   store.NewStore(),
				}},
			want:    nil,
			wantErr: assert.Error,
		},
		{
			name: "login not found error",
			args: args{
				Name:   name,
				Title:  title,
				Input:  map[string]string{},
				Output: map[string]string{},
				State:  &FormData{},
				RunContext: &BlockRunContext{
					UpdateData: &script.BlockUpdateData{
						ByLogin: login,
						Action:  string(entity.TaskUpdateActionRequestFillForm),
						Parameters: json.RawMessage(
							func() []byte {
								r, _ := json.Marshal(&updateFillFormParams{
									BlockId: blockId,
								})

								return r
							}(),
						),
					},
					VarStore: store.NewStore(),
				},
			},
			want:      nil,
			wantState: &FormData{},
			wantErr:   assert.Error,
		},
		{
			name: "fill form success case",
			args: args{
				Name:  name,
				Title: title,
				Input: map[string]string{},
				Output: map[string]string{
					keyOutputFormExecutor: global1,
					keyOutputFormBody:     global2,
				},
				Sockets: entity.ConvertSocket(next),
				State: &FormData{
					FormExecutorType: script.FormExecutorTypeFromSchema,
					SchemaId:         schemaId,
					SchemaName:       schemaName,
					Executors:        map[string]struct{}{login: {}},
					ApplicationBody:  map[string]interface{}{},
					IsFilled:         false,
					ActualExecutor:   nil,
					ChangesLog:       []ChangesLogItem{},
					IsTakenInWork:    true,
				},
				RunContext: &BlockRunContext{
					UpdateData: &script.BlockUpdateData{
						ByLogin: login,
						Action:  string(entity.TaskUpdateActionRequestFillForm),
						Parameters: json.RawMessage(
							func() []byte {
								r, _ := json.Marshal(&updateFillFormParams{
									Description: description,
									ApplicationBody: map[string]interface{}{
										fieldName: fieldValue,
									},
									BlockId: blockId,
								})

								return r
							}(),
						),
					},
					VarStore: store.NewStore(),
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
								Close:      true,
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
				},
			},
			want:    nil,
			wantErr: assert.NoError,
			wantState: &FormData{
				FormExecutorType: script.FormExecutorTypeFromSchema,
				SchemaId:         schemaId,
				SchemaName:       schemaName,
				Executors:        map[string]struct{}{login: {}},
				Description:      description,
				IsTakenInWork:    true,
				ApplicationBody: map[string]interface{}{
					fieldName: fieldValue,
				},
				IsFilled: true,
				ActualExecutor: func() *string {
					l := login
					return &l
				}(),
				ChangesLog: []ChangesLogItem{
					{
						Description: description,
						ApplicationBody: map[string]interface{}{
							fieldName: fieldValue,
						},
						CreatedAt: timeNow,
						Executor:  login,
					},
				},
			},
		},
		{
			name: "edit form success case",
			args: args{
				Name:  name,
				Title: title,
				Input: map[string]string{},
				Output: map[string]string{
					keyOutputFormExecutor: global1,
					keyOutputFormBody:     global2,
				},
				Sockets: entity.ConvertSocket(next),
				State: &FormData{
					FormExecutorType: script.FormExecutorTypeFromSchema,
					SchemaId:         schemaId,
					SchemaName:       schemaName,
					Executors:        map[string]struct{}{login: {}},
					ApplicationBody: map[string]interface{}{
						fieldName: fieldValue,
					},
					IsTakenInWork:  true,
					IsFilled:       true,
					ActualExecutor: getStringAddress(login),
					ChangesLog:     []ChangesLogItem{},
				},
				RunContext: &BlockRunContext{
					UpdateData: &script.BlockUpdateData{
						ByLogin: login,
						Action:  string(entity.TaskUpdateActionRequestFillForm),
						Parameters: json.RawMessage(
							func() []byte {
								r, _ := json.Marshal(&updateFillFormParams{
									Description: description,
									ApplicationBody: map[string]interface{}{
										fieldName: newValue,
									},
									BlockId: blockId,
								})

								return r
							}(),
						),
					},
					VarStore: store.NewStore(),
					ServiceDesc: func() *servicedesc.Service {
						sdMock := servicedesc.Service{
							SdURL: "",
						}
						httpClient := http.DefaultClient
						mockTransport := serviceDeskMocks.RoundTripper{}
						fResponse := func(*http.Request) *http.Response {
							b, _ := json.Marshal(servicedesc.SsoPerson{})
							body := io.NopCloser(bytes.NewReader(b))
							return &http.Response{
								Status:     http.StatusText(http.StatusOK),
								StatusCode: http.StatusOK,
								Body:       body,
								Close:      true,
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
				},
			},
			want:    nil,
			wantErr: assert.NoError,
			wantState: &FormData{
				FormExecutorType: script.FormExecutorTypeFromSchema,
				SchemaId:         schemaId,
				IsTakenInWork:    true,
				SchemaName:       schemaName,
				Executors:        map[string]struct{}{login: {}},
				Description:      description,
				ApplicationBody: map[string]interface{}{
					fieldName: newValue,
				},
				IsFilled: true,
				ActualExecutor: func() *string {
					l := login
					return &l
				}(),
				ChangesLog: []ChangesLogItem{
					{
						Description: description,
						ApplicationBody: map[string]interface{}{
							fieldName: newValue,
						},
						CreatedAt: timeNow,
						Executor:  login,
					},
				},
			},
		},
		{
			name: "edit form, not allowed error",
			args: args{
				Name:   name,
				Title:  title,
				Input:  map[string]string{},
				Output: map[string]string{},
				State: &FormData{
					ApplicationBody: map[string]interface{}{
						fieldName: fieldValue,
					},
					IsFilled: true,
				},
				RunContext: &BlockRunContext{
					UpdateData: &script.BlockUpdateData{
						ByLogin: login2,
						Action:  string(entity.TaskUpdateActionRequestFillForm),
						Parameters: json.RawMessage(
							func() []byte {
								r, _ := json.Marshal(&updateFillFormParams{
									Description: description,
									ApplicationBody: map[string]interface{}{
										fieldName: newValue,
									},
									BlockId: blockId,
								})

								return r
							}(),
						),
					},
					VarStore: store.NewStore(),
				},
			},
			want:    nil,
			wantErr: assert.Error,
			wantState: &FormData{
				ApplicationBody: map[string]interface{}{
					fieldName: fieldValue,
				},
				IsFilled: true,
			},
		},
		{
			name: "edit form, check permission error",
			args: args{
				Name:   name,
				Title:  title,
				Input:  map[string]string{},
				Output: map[string]string{},
				State: &FormData{
					ApplicationBody: map[string]interface{}{
						fieldName: fieldValue,
					},
					IsFilled: true,
				},
				RunContext: &BlockRunContext{
					UpdateData: &script.BlockUpdateData{
						ByLogin: login3,
						Action:  string(entity.TaskUpdateActionRequestFillForm),
						Parameters: json.RawMessage(
							func() []byte {
								r, _ := json.Marshal(&updateFillFormParams{
									Description: description,
									ApplicationBody: map[string]interface{}{
										fieldName: newValue,
									},
									BlockId: blockId,
								})

								return r
							}(),
						),
					},
					VarStore: store.NewStore(),
				},
			},
			want:    nil,
			wantErr: assert.Error,
			wantState: &FormData{
				ApplicationBody: map[string]interface{}{
					fieldName: fieldValue,
				},
				IsFilled: true,
			},
		},
		{
			name: "cancel pipeline case",
			args: args{
				Name:  name,
				Title: title,
				Input: map[string]string{},
				Output: map[string]string{
					keyOutputFormExecutor: global1,
					keyOutputFormBody:     global2,
				},
				Sockets: entity.ConvertSocket(next),
				State: &FormData{
					FormExecutorType: script.FormExecutorTypeFromSchema,
					SchemaId:         schemaId,
					SchemaName:       schemaName,
					Executors:        map[string]struct{}{login: {}},
					ApplicationBody:  map[string]interface{}{},
					IsFilled:         false,
					ActualExecutor:   nil,
					ChangesLog:       []ChangesLogItem{},
				},
				RunContext: &BlockRunContext{
					UpdateData: &script.BlockUpdateData{
						Action:     string(entity.TaskUpdateActionCancelApp),
						Parameters: json.RawMessage{},
					},
					VarStore: store.NewStore(),
					TaskID:   taskId1,
				},
			},
			want:    nil,
			wantErr: assert.NoError,
			wantState: &FormData{
				FormExecutorType: script.FormExecutorTypeFromSchema,
				SchemaId:         schemaId,
				SchemaName:       schemaName,
				Executors:        map[string]struct{}{login: {}},
				ApplicationBody:  map[string]interface{}{},
				ChangesLog:       []ChangesLogItem{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gb := &GoFormBlock{
				Name:       tt.args.Name,
				Title:      tt.args.Title,
				Input:      tt.args.Input,
				Output:     tt.args.Output,
				Sockets:    tt.args.Sockets,
				State:      tt.args.State,
				RunContext: tt.args.RunContext,
			}

			gb.RunContext.skipNotifications = true
			gb.RunContext.Storage = mockedDb

			got, err := gb.Update(ctx)

			if !tt.wantErr(t, err, "Update() method") {
				return
			}
			assert.Equalf(t, tt.want, got, "Update() method. Expect %v, got %v", tt.want, got)

			if gb.State != nil && len(gb.State.ChangesLog) > 0 {
				gb.State.ChangesLog[0].CreatedAt = timeNow
			}
			assert.Equalf(t, tt.wantState, gb.State,
				"Update() method. Expect State %v, got %v", tt.wantState, gb.State)
		})
	}
}

func TestGoFormBlock_Next(t *testing.T) {
	const blockId = "approver_0"

	type args struct {
		Sockets []script.Socket
	}

	tests := []struct {
		name   string
		args   args
		want   []string
		wantOk bool
	}{
		{
			name: "next block not found",
			args: args{
				Sockets: nil,
			},
			want:   nil,
			wantOk: false,
		},
		{
			name: "acceptance test",
			args: args{
				Sockets: []script.Socket{
					{
						Id:           DefaultSocketID,
						Title:        script.DefaultSocketTitle,
						NextBlockIds: []string{blockId},
					},
					script.DefaultSocket,
				},
			},
			want:   []string{blockId},
			wantOk: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gb := &GoFormBlock{
				Sockets: tt.args.Sockets,
			}
			got, ok := gb.Next(&store.VariableStore{})
			assert.Equalf(t, tt.want, got, "Update() method. Expect %v, got %v", tt.want, got)
			assert.Equalf(t, tt.wantOk, ok, "Update() method. Expect Ok %v, got %v", tt.wantOk, ok)
		})
	}
}
