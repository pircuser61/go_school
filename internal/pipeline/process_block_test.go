package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc"

	"github.com/google/uuid"

	"github.com/stretchr/testify/mock"

	"github.com/iancoleman/orderedmap"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db/mocks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	human_tasks "gitlab.services.mts.ru/jocasta/pipeliner/internal/humantasks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	serviceDeskMocks "gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc/mocks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sla"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type TestUpdateData struct {
	BlockName    string
	UpdateParams script.BlockUpdateData
}

func makeStorage() *mocks.MockedDatabase {
	res := &mocks.MockedDatabase{}

	res.On("GetTaskStatus",
		mock.MatchedBy(func(ctx context.Context) bool { return true }),
		uuid.UUID{},
	).Return(1, nil)

	res.On("GetTaskStatusWithReadableString",
		mock.MatchedBy(func(ctx context.Context) bool { return true }),
		uuid.UUID{},
	).Return(1, "running", nil)

	res.On("UpdateTaskStatus",
		mock.MatchedBy(func(ctx context.Context) bool { return true }),
		uuid.UUID{},
		mock.MatchedBy(func(taskStatus int) bool { return true }),
		mock.MatchedBy(func(comment string) bool { return true }),
		mock.MatchedBy(func(author string) bool { return true }),
	).Return(nil)

	res.On("UpdateTaskHumanStatus",
		mock.MatchedBy(func(ctx context.Context) bool { return true }),
		uuid.UUID{},
		mock.MatchedBy(func(status string) bool { return true }),
		mock.MatchedBy(func(comment string) bool { return true }),
	).Return(nil, nil)

	res.On("SaveStepContext",
		mock.MatchedBy(func(ctx context.Context) bool { return true }),
		mock.MatchedBy(func(data *db.SaveStepRequest) bool { return true }),
	).Return(uuid.UUID{}, time.Now(), nil)

	res.On("StopTaskBlocks",
		mock.MatchedBy(func(ctx context.Context) bool { return true }),
		mock.MatchedBy(func(id uuid.UUID) bool { return true }),
	).Return(nil)

	res.On("GetTaskRunContext",
		mock.MatchedBy(func(ctx context.Context) bool { return true }),
		mock.MatchedBy(func(workNumber string) bool { return true }),
	).Return(entity.TaskRunContext{
		InitialApplication: entity.InitialApplication{
			Description:     "",
			ApplicationBody: orderedmap.OrderedMap{},
		},
	}, nil)

	res.On("GetMergedVariableStorage",
		mock.MatchedBy(func(ctx context.Context) bool { return true }),
		mock.MatchedBy(func(workNumber uuid.UUID) bool { return true }),
		mock.MatchedBy(func(blockIds []string) bool { return true }),
	).Return(store.NewStore(), nil)

	res.On("GetVersionByWorkNumber",
		mock.MatchedBy(func(ctx context.Context) bool { return true }),
		mock.MatchedBy(func(workNumber string) bool { return true }),
	).Return(&entity.EriusScenario{}, nil)

	res.On("GetSlaVersionSettings",
		mock.MatchedBy(func(ctx context.Context) bool { return true }),
		mock.MatchedBy(func(versionId string) bool { return true }),
	).Return(entity.SLAVersionSettings{
		Author:   "voronin",
		WorkType: "8/5",
		SLA:      8,
	}, nil)

	res.On("GetCanceledTaskSteps",
		mock.MatchedBy(func(ctx context.Context) bool { return true }),
		mock.MatchedBy(func(taskID uuid.UUID) bool { return true }),
	).Return(nil, nil)

	res.On("GetParentTaskStepByName",
		mock.MatchedBy(func(ctx context.Context) bool { return true }),
		mock.MatchedBy(func(taskID uuid.UUID) bool { return true }),
		mock.MatchedBy(func(string) bool { return true }),
	).Return(&entity.Step{State: map[string]json.RawMessage{}}, nil)

	return res
}

func didMeetBlocks(src, dest []string) bool {
	for _, b1 := range src {
		var found bool

		for _, b2 := range dest {
			if b1 == b2 {
				found = true
			}
		}

		if !found {
			return false
		}
	}

	return true
}

func TestProcessBlock(t *testing.T) {
	const shortTitle = "Нода"

	type fields struct {
		Entrypoint string
		RunContext *BlockRunContext
		Updates    []TestUpdateData
	}

	sdAppParams, err := json.Marshal(script.SdApplicationParams{BlueprintID: "123"})
	if err != nil {
		t.Fatal(err)
	}

	approveParams, err := json.Marshal(script.ApproverParams{
		Type:     script.ApproverTypeUser,
		SLA:      1,
		Approver: "tester",
	})
	if err != nil {
		t.Fatal(err)
	}

	workType := "24/7"

	execParams, err := json.Marshal(script.ExecutionParams{
		Type:      script.ExecutionTypeUser,
		SLA:       1,
		Executors: "tester",
		WorkType:  &workType,
	})
	if err != nil {
		t.Fatal(err)
	}

	approveUpdParams, err := json.Marshal(approverUpdateParams{
		Decision: ApproverActionApprove,
	})
	if err != nil {
		t.Fatal(err)
	}

	executeUpdParams, err := json.Marshal(ExecutionUpdateParams{
		Decision: ExecutionDecisionExecuted,
	})
	if err != nil {
		t.Fatal(err)
	}

	var (
		metBlocks   []string
		latestBlock string
	)

	tests := []struct {
		name   string
		fields fields
	}{
		{
			name: "start-application-end",
			fields: fields{
				Entrypoint: "start_0",
				RunContext: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
					Services: RunContextServices{

						Storage: func() db.Database {
							res := makeStorage()

							res.On("UpdateStepContext",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								mock.MatchedBy(func(data *db.UpdateStepRequest) bool { return true }),
							).Run(func(args mock.Arguments) {
								req := args.Get(1).(*db.UpdateStepRequest)
								if req.Status == string(StatusFinished) {
									latestBlock = req.StepName
								}
							}).Return(nil)

							res.On("GetBlockDataFromVersion",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								mock.MatchedBy(func(workNumber string) bool { return true }),
								"start_0",
							).Return(
								&entity.EriusFunc{
									TypeID:     BlockGoStartID,
									BlockType:  script.TypeGo,
									Title:      BlockGoStartTitle,
									ShortTitle: shortTitle,
									Sockets: []entity.Socket{
										{
											ID:           DefaultSocketID,
											NextBlockIds: []string{"servicedesk_application_0"},
										},
									},
								}, nil,
							)

							res.On("GetBlockDataFromVersion",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								mock.MatchedBy(func(workNumber string) bool { return true }),
								"servicedesk_application_0",
							).Return(
								&entity.EriusFunc{
									TypeID:     BlockGoSdApplicationID,
									BlockType:  script.TypeGo,
									Title:      BlockGoSdApplicationTitle,
									ShortTitle: shortTitle,
									Sockets: []entity.Socket{
										{
											ID:           DefaultSocketID,
											NextBlockIds: []string{"end_0"},
										},
									},
									Params: sdAppParams,
								}, nil,
							)

							res.On("GetBlockDataFromVersion",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								mock.MatchedBy(func(workNumber string) bool { return true }),
								"end_0",
							).Return(
								&entity.EriusFunc{
									TypeID:     BlockGoEndID,
									BlockType:  script.TypeGo,
									Title:      BlockGoEndTitle,
									ShortTitle: shortTitle,
								}, nil,
							)

							res.On("CheckIsArchived",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								uuid.Nil,
							).Return(false, nil)

							return res
						}(),
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
							fError := func(*http.Request) error {
								return nil
							}
							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, fError)
							httpClient.Transport = &mockTransport
							sdMock.Cli = httpClient

							return &sdMock
						}(),
					},
				},
			},
		},
		{
			name: "start-application-startparallel-approve,execute-endparallel-end",
			fields: fields{
				Entrypoint: "start_0",
				RunContext: &BlockRunContext{
					skipNotifications: true,
					VarStore:          store.NewStore(),
					Services: RunContextServices{
						Storage: func() db.Database {
							res := makeStorage()

							res.On("UpdateStepContext",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								mock.MatchedBy(func(data *db.UpdateStepRequest) bool { return true }),
							).Run(func(args mock.Arguments) {
								req := args.Get(1).(*db.UpdateStepRequest)
								if req.Status == string(StatusFinished) {
									latestBlock = req.StepName
									metBlocks = append(metBlocks, req.StepName)
								}
							}).Return(nil)

							res.On("ParallelIsFinished",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								mock.MatchedBy(func(workNumber string) bool { return true }),
								mock.MatchedBy(func(blockName string) bool { return true }),
							).Return(
								false, nil,
							)
							currCall := res.ExpectedCalls[len(res.ExpectedCalls)-1]
							currCall = currCall.Run(func(args mock.Arguments) {
								currCall.ReturnArguments[0] =
									didMeetBlocks([]string{"approver_0", "execution_0"}, metBlocks)
							})

							res.ExpectedCalls[len(res.ExpectedCalls)-1] = currCall

							res.On("GetTaskStepsToWait",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								mock.MatchedBy(func(workNumber string) bool { return true }),
								mock.MatchedBy(func(name string) bool { return true }),
							).Return(
								[]string{"approver_0", "execution_0"}, nil,
							)

							res.On("GetBlockDataFromVersion",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								mock.MatchedBy(func(workNumber string) bool { return true }),
								"start_0",
							).Return(
								&entity.EriusFunc{
									TypeID:     BlockGoStartID,
									BlockType:  script.TypeGo,
									Title:      BlockGoStartTitle,
									ShortTitle: shortTitle,
									Sockets: []entity.Socket{
										{
											ID:           DefaultSocketID,
											NextBlockIds: []string{"servicedesk_application_0"},
										},
									},
								}, nil,
							)

							res.On("GetBlockDataFromVersion",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								mock.MatchedBy(func(workNumber string) bool { return true }),
								"servicedesk_application_0",
							).Return(
								&entity.EriusFunc{
									TypeID:     BlockGoSdApplicationID,
									BlockType:  script.TypeGo,
									Title:      BlockGoSdApplicationTitle,
									ShortTitle: shortTitle,
									Sockets: []entity.Socket{
										{
											ID:           DefaultSocketID,
											NextBlockIds: []string{"start_parallel_0"},
										},
									},
									Params: sdAppParams,
								}, nil,
							)

							res.On("GetBlockDataFromVersion",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								mock.MatchedBy(func(workNumber string) bool { return true }),
								"start_parallel_0",
							).Return(
								&entity.EriusFunc{
									TypeID:     BlockGoBeginParallelTaskID,
									BlockType:  script.TypeGo,
									Title:      BlockGoBeginParallelTaskTitle,
									ShortTitle: shortTitle,
									Sockets: []entity.Socket{
										{
											ID:           DefaultSocketID,
											NextBlockIds: []string{"approver_0", "execution_0"},
										},
									},
								}, nil,
							)

							res.On("GetBlockDataFromVersion",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								mock.MatchedBy(func(workNumber string) bool { return true }),
								"approver_0",
							).Return(
								&entity.EriusFunc{
									TypeID:     BlockGoApproverID,
									ShortTitle: shortTitle,
									BlockType:  script.TypeGo,
									Sockets: []entity.Socket{
										{
											ID:           "approve",
											NextBlockIds: []string{"end_parallel_0"},
										},
									},
									Params: approveParams,
								}, nil,
							)

							res.On("GetBlockDataFromVersion",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								mock.MatchedBy(func(workNumber string) bool { return true }),
								"execution_0",
							).Return(
								&entity.EriusFunc{
									TypeID:     BlockGoExecutionID,
									ShortTitle: shortTitle,
									BlockType:  script.TypeGo,
									Sockets: []entity.Socket{
										{
											ID:           executedSocketID,
											NextBlockIds: []string{"end_parallel_0"},
										},
									},
									Params: execParams,
								}, nil,
							)

							res.On("GetBlockDataFromVersion",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								mock.MatchedBy(func(workNumber string) bool { return true }),
								"end_parallel_0",
							).Return(
								&entity.EriusFunc{
									TypeID:     BlockWaitForAllInputsID,
									ShortTitle: shortTitle,
									BlockType:  script.TypeGo,
									Title:      BlockGoWaitForAllInputsTitle,
									Sockets: []entity.Socket{
										{
											ID:           DefaultSocketID,
											NextBlockIds: []string{"end_0"},
										},
									},
								}, nil,
							)

							res.On("GetBlockDataFromVersion",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								mock.MatchedBy(func(workNumber string) bool { return true }),
								"end_0",
							).Return(
								&entity.EriusFunc{
									TypeID:     BlockGoEndID,
									ShortTitle: shortTitle,
									BlockType:  script.TypeGo,
									Title:      BlockGoEndTitle,
								}, nil,
							)

							res.On("CheckIsArchived",
								mock.MatchedBy(func(ctx context.Context) bool { return true }),
								uuid.Nil,
							).Return(false, nil)

							return res
						}(),
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
							fError := func(*http.Request) error {
								return nil
							}
							mockTransport.On("RoundTrip", mock.Anything).Return(fResponse, fError)
							httpClient.Transport = &mockTransport
							sdMock.Cli = httpClient

							return &sdMock
						}(),
						SLAService: func() sla.Service {
							slaMock := sla.NewSLAService(nil)

							return slaMock
						}(),
						HumanTasks: func() *human_tasks.Service {
							service, _ := human_tasks.NewService(human_tasks.Config{})

							return service
						}(),
					},
					Delegations: func() human_tasks.Delegations {
						return human_tasks.Delegations{}
					}(),
				},
				Updates: []TestUpdateData{
					{
						BlockName: "approver_0",
						UpdateParams: script.BlockUpdateData{
							ByLogin:    "tester",
							Action:     string(entity.TaskUpdateActionApprovement),
							Parameters: approveUpdParams,
						},
					},
					{
						BlockName: "execution_0",
						UpdateParams: script.BlockUpdateData{
							ByLogin:    "tester",
							Action:     string(entity.TaskUpdateActionExecutorStartWork),
							Parameters: executeUpdParams,
						},
					},
					{
						BlockName: "execution_0",
						UpdateParams: script.BlockUpdateData{
							ByLogin:    "tester",
							Action:     string(entity.TaskUpdateActionExecution),
							Parameters: executeUpdParams,
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		metBlocks = metBlocks[:0]
		latestBlock = ""

		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			entrypointData, blockErr := tt.fields.RunContext.Services.Storage.GetBlockDataFromVersion(
				ctx, "", tt.fields.Entrypoint)
			if blockErr != nil {
				t.Fatal(blockErr)
			}

			if procErr := ProcessBlockWithEndMapping(context.Background(), tt.fields.Entrypoint, entrypointData,
				tt.fields.RunContext, false); procErr != nil {
				t.Fatal(procErr)
			}

			for i := range tt.fields.Updates {
				blockData, updateErr := tt.fields.RunContext.Services.Storage.GetBlockDataFromVersion(ctx, "", tt.fields.Updates[i].BlockName)
				if updateErr != nil {
					t.Fatal(updateErr)
				}

				tt.fields.RunContext.UpdateData = &tt.fields.Updates[i].UpdateParams
				if procErr := ProcessBlockWithEndMapping(context.Background(), tt.fields.Updates[i].BlockName, blockData,
					tt.fields.RunContext, true); procErr != nil {
					t.Fatal(procErr)
				}
			}

			if latestBlock != "end_0" {
				t.Fatalf("Didn't reach the end, reached %s instead", latestBlock)
			}
		})
	}
}
