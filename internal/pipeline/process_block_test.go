package pipeline

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stretchr/testify/mock"

	"github.com/iancoleman/orderedmap"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db/mocks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

func makeStorage() *mocks.MockedDatabase {
	res := &mocks.MockedDatabase{}

	res.On("GetTaskStatus",
		mock.MatchedBy(func(ctx context.Context) bool { return true }),
		uuid.UUID{},
	).Return(1, nil)

	res.On("UpdateTaskStatus",
		mock.MatchedBy(func(ctx context.Context) bool { return true }),
		uuid.UUID{},
		mock.MatchedBy(func(taskStatus int) bool { return true }),
	).Return(nil)

	res.On("UpdateTaskHumanStatus",
		mock.MatchedBy(func(ctx context.Context) bool { return true }),
		uuid.UUID{},
		mock.MatchedBy(func(status string) bool { return true }),
	).Return(nil)

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
	type fields struct {
		Entrypoint string
		RunContext *BlockRunContext
		Updates    map[string]script.BlockUpdateData
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
	execParams, err := json.Marshal(script.ExecutionParams{
		Type:      script.ExecutionTypeUser,
		SLA:       1,
		Executors: "tester",
	})
	if err != nil {
		t.Fatal(err)
	}
	approveUpdParams, err := json.Marshal(approverUpdateParams{
		Decision: ApproverDecisionApproved,
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

	var metBlocks []string
	var latestBlock string

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
								TypeID:    BlockGoStartId,
								BlockType: script.TypeGo,
								Title:     BlockGoStartTitle,
								Sockets: []entity.Socket{
									{
										Id:           DefaultSocketID,
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
								TypeID:    BlockGoSdApplicationID,
								BlockType: script.TypeGo,
								Title:     BlockGoSdApplicationTitle,
								Sockets: []entity.Socket{
									{
										Id:           DefaultSocketID,
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
								TypeID:    BlockGoEndId,
								BlockType: script.TypeGo,
								Title:     BlockGoEndTitle,
							}, nil,
						)
						return res
					}(),
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

						res.On("CheckTaskStepsExecuted",
							mock.MatchedBy(func(ctx context.Context) bool { return true }),
							mock.MatchedBy(func(workNumber string) bool { return true }),
							mock.MatchedBy(func(ids []string) bool { return true }),
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
								TypeID:    BlockGoStartId,
								BlockType: script.TypeGo,
								Title:     BlockGoStartTitle,
								Sockets: []entity.Socket{
									{
										Id:           DefaultSocketID,
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
								TypeID:    BlockGoSdApplicationID,
								BlockType: script.TypeGo,
								Title:     BlockGoSdApplicationTitle,
								Sockets: []entity.Socket{
									{
										Id:           DefaultSocketID,
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
								TypeID:    BlockGoBeginParallelTaskId,
								BlockType: script.TypeGo,
								Title:     BlockGoBeginParallelTaskTitle,
								Sockets: []entity.Socket{
									{
										Id:           DefaultSocketID,
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
								TypeID:    BlockGoApproverID,
								BlockType: script.TypeGo,
								Sockets: []entity.Socket{
									{
										Id:           approvedSocketID,
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
								TypeID:    BlockGoExecutionID,
								BlockType: script.TypeGo,
								Sockets: []entity.Socket{
									{
										Id:           executedSocketID,
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
								TypeID:    BlockWaitForAllInputsId,
								BlockType: script.TypeGo,
								Title:     BlockGoWaitForAllInputsTitle,
								Sockets: []entity.Socket{
									{
										Id:           DefaultSocketID,
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
								TypeID:    BlockGoEndId,
								BlockType: script.TypeGo,
								Title:     BlockGoEndTitle,
							}, nil,
						)
						return res
					}(),
				},
				Updates: map[string]script.BlockUpdateData{
					"approver_0": {
						ByLogin:    "tester",
						Action:     string(entity.TaskUpdateActionApprovement),
						Parameters: approveUpdParams,
					},
					"execution_0": {
						ByLogin:    "tester",
						Action:     string(entity.TaskUpdateActionExecution),
						Parameters: executeUpdParams,
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
			entrypointData, blockErr := tt.fields.RunContext.Storage.GetBlockDataFromVersion(
				ctx, "", tt.fields.Entrypoint)
			if blockErr != nil {
				t.Fatal(blockErr)
			}
			if procErr := ProcessBlock(context.Background(), tt.fields.Entrypoint, entrypointData,
				tt.fields.RunContext, false); procErr != nil {
				t.Fatal(procErr)
			}
			for name, params := range tt.fields.Updates {
				blockData, updateErr := tt.fields.RunContext.Storage.GetBlockDataFromVersion(ctx, "", name)
				if updateErr != nil {
					t.Fatal(updateErr)
				}
				tt.fields.RunContext.UpdateData = &params
				if procErr := ProcessBlock(context.Background(), name, blockData,
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
