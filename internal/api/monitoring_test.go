package api

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db/mocks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

func TestGetNodesToSkip(t *testing.T) {
	type skipParams struct {
		nextNodes  map[string][]string
		workNumber string
		steps      map[string]bool
	}

	tests := []struct {
		name   string
		params skipParams
		result []string
	}{
		{
			name: "one simple node",
			params: skipParams{
				nextNodes: map[string][]string{
					"default": {"servicedesk_application_0"},
				},
				workNumber: "J1",
				steps: map[string]bool{
					"servicedesk_application_0": true,
				},
			},
			result: []string{"servicedesk_application_0"},
		},

		{
			name: "many nodes",
			params: skipParams{
				nextNodes: map[string][]string{
					"default": {"servicedesk_application_0"},
				},
				workNumber: "J2",
				steps: map[string]bool{
					"servicedesk_application_0": true,
					"execution_0":               true,
					"execution_2":               true,
				},
			},
			result: []string{"servicedesk_application_0", "execution_0", "execution_2"},
		},
	}

	ae := &Env{
		DB: func() db.Database {
			res := &mocks.MockedDatabase{}

			res.On("GetBlockDataFromVersion",
				mock.MatchedBy(func(ctx context.Context) bool { return true }),
				mock.MatchedBy(func(workNumber string) bool { return true }),
				"start_0",
			).Return(
				&entity.EriusFunc{
					TypeID:    pipeline.BlockGoStartID,
					BlockType: script.TypeGo,
					Title:     pipeline.BlockGoStartTitle,
					Next: map[string][]string{
						"default": {"servicedesk_application_0"},
					},
				}, nil,
			)

			res.On("GetBlockDataFromVersion",
				mock.MatchedBy(func(ctx context.Context) bool { return true }),
				mock.MatchedBy(func(workNumber string) bool { return true }),
				"servicedesk_application_0",
			).Return(
				&entity.EriusFunc{
					TypeID:    pipeline.BlockGoSdApplicationID,
					BlockType: script.TypeGo,
					Title:     pipeline.BlockGoSdApplicationTitle,

					Next: map[string][]string{
						"default": {"execution_0"},
					},
				}, nil,
			)

			res.On("GetBlockDataFromVersion",
				mock.MatchedBy(func(ctx context.Context) bool { return true }),
				mock.MatchedBy(func(workNumber string) bool { return true }),
				"execution_0",
			).Return(
				&entity.EriusFunc{
					TypeID:    pipeline.BlockGoExecutionID,
					BlockType: script.TypeGo,
					Title:     pipeline.BlockGoExecutionID,

					Next: map[string][]string{
						"default":  {"execution_1"},
						"rejected": {"execution_2"},
					},
				}, nil,
			)

			res.On("GetBlockDataFromVersion",
				mock.MatchedBy(func(ctx context.Context) bool { return true }),
				mock.MatchedBy(func(workNumber string) bool { return true }),
				"execution_1",
			).Return(
				&entity.EriusFunc{
					TypeID:    pipeline.BlockGoExecutionID,
					BlockType: script.TypeGo,
					Title:     pipeline.BlockGoExecutionID,

					Next: map[string][]string{
						"default": {"end_0"},
					},
				}, nil,
			)

			res.On("GetBlockDataFromVersion",
				mock.MatchedBy(func(ctx context.Context) bool { return true }),
				mock.MatchedBy(func(workNumber string) bool { return true }),
				"execution_2",
			).Return(
				&entity.EriusFunc{
					TypeID:    pipeline.BlockGoExecutionID,
					BlockType: script.TypeGo,
					Title:     pipeline.BlockGoExecutionID,

					Next: map[string][]string{
						"default": {"end_0"},
					},
				}, nil,
			)

			res.On("GetBlockDataFromVersion",
				mock.MatchedBy(func(ctx context.Context) bool { return true }),
				mock.MatchedBy(func(workNumber string) bool { return true }),
				"end_0",
			).Return(
				&entity.EriusFunc{
					TypeID:    pipeline.BlockGoEndID,
					BlockType: script.TypeGo,
					Title:     pipeline.BlockGoEndTitle,
				}, nil,
			)

			return res
		}(),
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			nodes, err := ae.getNodesToSkip(ctx, tt.params.nextNodes, tt.params.workNumber, tt.params.steps)
			if err != nil {
				t.Fatal(err)
			}
			if !assert.ElementsMatch(t, nodes, tt.result) {
				t.Fatalf("Didn't matched the patten nodes %s , got %s ", nodes, tt.result)
			}
		})
	}
}

func Test_toMonitoringTaskResponse(t *testing.T) {
	type args struct {
		nodes  []entity.MonitoringTaskNode
		events []entity.TaskEvent
	}
	tests := []struct {
		name string
		args args
		want *MonitoringTask
	}{
		{
			name: "success",
			args: args{
				nodes: []entity.MonitoringTaskNode{
					{
						WorkNumber:    "J666",
						VersionID:     "6969",
						IsPaused:      true,
						BlockIsPaused: true,
						Author:        "lohundra",
						ScenarioName:  "ebanina",
						CreationTime:  "6.6.6",
					},
				},
				events: []entity.TaskEvent{
					{
						ID:        "1",
						EventType: "start",
					},
					{
						ID:        "2",
						EventType: "edit",
					},
					{
						ID:        "3",
						EventType: "pause",
					},
					{
						ID:        "4",
						EventType: "start",
					},
					{
						ID:        "5",
						EventType: "other action",
					},
				},
			},
			want: &MonitoringTask{
				IsPaused: true,
				ScenarioInfo: MonitoringScenarioInfo{
					Author:       "lohundra",
					CreationTime: "6.6.6",
					ScenarioName: "ebanina",
				},
				TaskRuns: []MonitoringTaskRun{
					{
						StartEventId: "1",
						EndEventId:   "3",
						Index:        float32(1),
					},
					{
						StartEventId: "4",
						EndEventId:   "",
						Index:        float32(2),
					},
				},
				History: []MonitoringHistory{
					{
						IsPaused: true,
						Status: "running",
					},
				},
				VersionId:  "6969",
				WorkNumber: "J666",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toMonitoringTaskResponse(tt.args.nodes, tt.args.events); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("toMonitoringTaskResponse() = %v, want %v", got, tt.want)
			}
		})
	}
}
