package api

import (
	"bytes"
	"context"
	"encoding/json"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/httpclient"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"github.com/hrishin/httpmock"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc"
)

func TestValidation_EndExists(t *testing.T) {
	log := logger.GetLogger(context.TODO())

	tests := []struct {
		Name      string
		Ef        entity.EriusScenario
		WantValid bool
	}{
		{
			Name: "test valid blocks with end block",
			Ef: entity.EriusScenario{
				Pipeline: entity.PipelineType{
					Blocks: entity.BlocksType{
						"start_0": {
							TypeID: "start",
						},
						"end_0": {
							TypeID: "end",
						},
					},
				},
			},
			WantValid: true,
		},
		{
			Name: "test invalid block without end block",
			Ef: entity.EriusScenario{
				Pipeline: entity.PipelineType{
					Blocks: entity.BlocksType{
						"start_0": {
							TypeID: "start",
						},
						"approver_0": {
							TypeID: "approver",
						},
					},
				},
			},
			WantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			if tt.Ef.Pipeline.Blocks.EndExists(log) != tt.WantValid {
				t.Errorf("unexpected invalid %+v", tt.Ef.Pipeline.Blocks)
			}
		})
	}
}

func TestValidation_SocketFilled(t *testing.T) {
	log := logger.GetLogger(context.TODO())

	tests := []struct {
		Name      string
		Ef        entity.EriusScenario
		WantValid bool
	}{
		{
			Name: "test socket filled all sockets filled",
			Ef: entity.EriusScenario{
				Pipeline: entity.PipelineType{
					Blocks: entity.BlocksType{
						"start_0": {
							TypeID: "start",
							Sockets: []entity.Socket{
								{
									ID: "approved",
								},
							},
							Next: map[string][]string{
								"approved": {"end_0"},
							},
						},
						"end_0": {
							TypeID: "end",
						},
					},
				},
			},
			WantValid: true,
		},
		{
			Name: "test socket filled missing next field",
			Ef: entity.EriusScenario{
				Pipeline: entity.PipelineType{
					Blocks: entity.BlocksType{
						"start_0": {
							TypeID: "start",
							Sockets: []entity.Socket{
								{
									ID: "approved",
								},
								{
									ID: "rejected",
								},
							},
							Next: map[string][]string{
								"approved": {"end_0"},
							},
						},
						"end_0": {
							TypeID: "end",
						},
					},
				},
			},
			WantValid: false,
		},
		{
			Name: "test socket filled missing socket field",
			Ef: entity.EriusScenario{
				Pipeline: entity.PipelineType{
					Blocks: entity.BlocksType{
						"start_0": {
							TypeID: "start",
							Sockets: []entity.Socket{
								{
									ID: "approved",
								},
							},
							Next: map[string][]string{
								"approved": {"end_0"},
								"rejected": {"start_0"},
							},
						},
						"end_0": {
							TypeID: "end",
						},
					},
				},
			},
			WantValid: false,
		},
		{
			Name: "test socket filled empty next array",
			Ef: entity.EriusScenario{
				Pipeline: entity.PipelineType{
					Blocks: entity.BlocksType{
						"start_0": {
							TypeID: "start",
							Sockets: []entity.Socket{
								{
									ID: "approved",
								},
							},
							Next: map[string][]string{
								"approved": {},
							},
						},
						"end_0": {
							TypeID: "end",
						},
					},
				},
			},
			WantValid: false,
		},
		{
			Name: "test socket filled empty next",
			Ef: entity.EriusScenario{
				Pipeline: entity.PipelineType{
					Blocks: entity.BlocksType{
						"start_0": {
							TypeID: "start",
							Sockets: []entity.Socket{
								{
									ID: "approved",
								},
							},
							Next: map[string][]string{},
						},
						"end_0": {
							TypeID: "end",
						},
					},
				},
			},
			WantValid: false,
		},
		{
			Name: "test socket filled empty sockets",
			Ef: entity.EriusScenario{
				Pipeline: entity.PipelineType{
					Blocks: entity.BlocksType{
						"start_0": {
							TypeID:  "start",
							Sockets: []entity.Socket{},
							Next: map[string][]string{
								"approved": {"end_0"},
							},
						},
						"end_0": {
							TypeID: "end",
						},
					},
				},
			},
			WantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			isValid, _ := tt.Ef.Pipeline.Blocks.IsSocketsFilled(log)
			if isValid != tt.WantValid {
				t.Errorf("unexpected invalid %+v", tt.Ef.Pipeline.Blocks)
			}
		})
	}
}

func TestValidation_SdBlueprintFilled(t *testing.T) {
	mockResponse := httpmock.Response{
		URI:        "/api/herald/v1/schema/blueprint/59d1a7e6-011d-11ed-b7f9-baa4bc97ef20",
		StatusCode: 200,
		Body:       "bar response",
	}
	tests := []struct {
		Name      string
		Ef        entity.EriusScenario
		WantValid bool
	}{
		{
			Name: "test sd blueprint id filled id filled",
			Ef: entity.EriusScenario{
				Pipeline: entity.PipelineType{
					Blocks: entity.BlocksType{
						"servicedesk_application_0": {
							TypeID: "servicedesk_application",
							Params: func() json.RawMessage {
								r, _ := json.Marshal(&script.SdApplicationParams{
									BlueprintID: "59d1a7e6-011d-11ed-b7f9-baa4bc97ef20",
								})

								return r
							}(),
						},
					},
				},
			},
			WantValid: true,
		},
		{
			Name: "test sd blueprint id filled id not filled",
			Ef: entity.EriusScenario{
				Pipeline: entity.PipelineType{
					Blocks: entity.BlocksType{
						"servicedesk_application_0": {
							TypeID: "servicedesk_application",
							Params: func() json.RawMessage {
								r, _ := json.Marshal(&script.SdApplicationParams{
									BlueprintID: "",
								})

								return r
							}(),
						},
					},
				},
			},
			WantValid: false,
		},
	}

	retryableHttpClient := httpclient.HTTPClientWithRetries(httpmock.Client(&mockResponse), nil, 0, 0)

	sdAPI := &servicedesc.Service{
		Cli:   retryableHttpClient,
		SdURL: "https://dev.servicedesk.mts.ru",
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			if tt.Ef.Pipeline.Blocks.IsSdBlueprintFilled(context.Background(), sdAPI) != tt.WantValid {
				t.Errorf("unexpected invalid %+v", tt.Ef.Pipeline.Blocks)
			}
		})
	}
}

func TestValidation_ParallelNodes(t *testing.T) {
	tests := []struct {
		Name      string
		Ef        entity.EriusScenario
		WantValid bool
	}{
		{
			Name: "positive case",
			Ef: entity.EriusScenario{
				Pipeline: entity.PipelineType{
					Blocks: entity.BlocksType{
						"start_0": {
							TypeID: "start",
							Next:   map[string][]string{"default": {"begin_parallel_task_0"}},
							Sockets: []entity.Socket{
								{
									ID:           script.DefaultSocketID,
									Title:        script.DefaultSocketTitle,
									NextBlockIds: []string{"begin_parallel_task_0"},
								},
							},
						},
						"begin_parallel_task_0": {
							TypeID: "begin_parallel_task",
							Next:   map[string][]string{"default": {"approver_1", "approver_2"}},
							Sockets: []entity.Socket{
								{
									ID:           script.DefaultSocketID,
									Title:        script.DefaultSocketTitle,
									NextBlockIds: []string{"approver_1", "approver_2"},
								},
							},
						},
						"approver_1": {
							TypeID: "approver",
							Next: map[string][]string{
								"approve": {"wait_for_all_inputs_0"},
								"reject":  {"wait_for_all_inputs_0"},
							},
							Sockets: []entity.Socket{
								{
									ID:           "approve",
									Title:        "Согласовать",
									NextBlockIds: []string{"wait_for_all_inputs_0"},
								},
								{
									ID:           "reject",
									Title:        "Отклонить",
									NextBlockIds: []string{"wait_for_all_inputs_0"},
								},
							},
						},
						"approver_2": {
							TypeID: "approver",
							Next: map[string][]string{
								"approve": {"approver_3"},
								"reject":  {"approver_3"},
							},
							Sockets: []entity.Socket{
								{
									ID:           "approve",
									Title:        "Согласовать",
									NextBlockIds: []string{"approver_3"},
								},
								{
									ID:           "reject",
									Title:        "Отклонить",
									NextBlockIds: []string{"approver_3"},
								},
							},
						},
						"approver_3": {
							TypeID: "approver",
							Next: map[string][]string{
								"approve": {"approver_2"},
								"reject":  {"wait_for_all_inputs_0"},
							},
							Sockets: []entity.Socket{
								{
									ID:           "approve",
									Title:        "Согласовать",
									NextBlockIds: []string{"approver_2"},
								},
								{
									ID:           "reject",
									Title:        "Отклонить",
									NextBlockIds: []string{"wait_for_all_inputs_0"},
								},
							},
						},
						"wait_for_all_inputs_0": {
							TypeID: "wait_for_all_inputs",
							Next:   map[string][]string{"default": {"end_0"}},
							Sockets: []entity.Socket{
								{
									ID:           script.DefaultSocketID,
									Title:        script.DefaultSocketTitle,
									NextBlockIds: []string{"end_0"},
								},
							},
						},
						"end_0": {
							TypeID: "end",
							Sockets: []entity.Socket{
								{
									ID:           script.DefaultSocketID,
									Title:        script.DefaultSocketTitle,
									NextBlockIds: []string{},
								},
							},
						},
					},
				},
			},
			WantValid: true,
		},
		{
			Name: "error case, more than one parallel end",
			Ef: entity.EriusScenario{
				Pipeline: entity.PipelineType{
					Blocks: entity.BlocksType{
						"start_0": {
							TypeID: "start",
							Next:   map[string][]string{"default": {"begin_parallel_task_0"}},
							Sockets: []entity.Socket{
								{
									ID:           script.DefaultSocketID,
									Title:        script.DefaultSocketTitle,
									NextBlockIds: []string{"begin_parallel_task_0"},
								},
							},
						},
						"begin_parallel_task_0": {
							TypeID: "begin_parallel_task",
							Next:   map[string][]string{"default": {"approver_1", "approver_2"}},
							Sockets: []entity.Socket{
								{
									ID:           script.DefaultSocketID,
									Title:        script.DefaultSocketTitle,
									NextBlockIds: []string{"approver_1", "approver_2"},
								},
							},
						},
						"approver_1": {
							TypeID: "approver",
							Next: map[string][]string{
								"approve": {"wait_for_all_inputs_0"},
								"reject":  {"wait_for_all_inputs_1"},
							},
							Sockets: []entity.Socket{
								{
									ID:           "approve",
									Title:        "Согласовать",
									NextBlockIds: []string{"wait_for_all_inputs_0"},
								},
								{
									ID:           "reject",
									Title:        "Отклонить",
									NextBlockIds: []string{"wait_for_all_inputs_1"},
								},
							},
						},
						"approver_2": {
							TypeID: "approver",
							Next: map[string][]string{
								"approve": {"approver_3"},
								"reject":  {"approver_3"},
							},
							Sockets: []entity.Socket{
								{
									ID:           "approve",
									Title:        "Согласовать",
									NextBlockIds: []string{"approver_3"},
								},
								{
									ID:           "reject",
									Title:        "Отклонить",
									NextBlockIds: []string{"approver_3"},
								},
							},
						},
						"approver_3": {
							TypeID: "approver",
							Next: map[string][]string{
								"approve": {"approver_2"},
								"reject":  {"wait_for_all_inputs_0"},
							},
							Sockets: []entity.Socket{
								{
									ID:           "approve",
									Title:        "Согласовать",
									NextBlockIds: []string{"approver_2"},
								},
								{
									ID:           "reject",
									Title:        "Отклонить",
									NextBlockIds: []string{"wait_for_all_inputs_0"},
								},
							},
						},
						"wait_for_all_inputs_0": {
							TypeID: "wait_for_all_inputs",
							Next:   map[string][]string{"default": {"end_0"}},
							Sockets: []entity.Socket{
								{
									ID:           script.DefaultSocketID,
									Title:        script.DefaultSocketTitle,
									NextBlockIds: []string{"end_0"},
								},
							},
						},
						"wait_for_all_inputs_1": {
							TypeID: "wait_for_all_inputs",
							Next:   map[string][]string{"default": {"end_0"}},
							Sockets: []entity.Socket{
								{
									ID:           script.DefaultSocketID,
									Title:        script.DefaultSocketTitle,
									NextBlockIds: []string{"end_0"},
								},
							},
						},
						"end_0": {
							TypeID: "end",
							Sockets: []entity.Socket{
								{
									ID:           script.DefaultSocketID,
									Title:        script.DefaultSocketTitle,
									NextBlockIds: []string{},
								},
							},
						},
					},
				},
			},
			WantValid: false,
		},
		{
			Name:      "Valid",
			Ef:        *unmarshalFromTestFile(t, "testdata/test_parallel_valid.json"),
			WantValid: true,
		},
		{
			Name:      "outOfParallelEnd",
			Ef:        *unmarshalFromTestFile(t, "testdata/test_parallel_out_of_end.json"),
			WantValid: false,
		},
		{
			Name:      "outOfParallelStart",
			Ef:        *unmarshalFromTestFile(t, "testdata/test_parallel_out_of_start.json"),
			WantValid: false,
		},
		{
			Name:      "cycle returning from parallel",
			Ef:        *unmarshalFromTestFile(t, "testdata/test_parallel_cycle.json"),
			WantValid: false,
		},
		{
			Name:      "intersected branch bad between paralls bad 1",
			Ef:        *unmarshalFromTestFile(t, "testdata/test_parallel_intersected_branches_bad1.json"),
			WantValid: false,
		},
		{
			Name:      "intersected branch valid 1",
			Ef:        *unmarshalFromTestFile(t, "testdata/test_parallel_intersected_branches_valid1.json"),
			WantValid: true,
		},
		{
			Name:      "intersected branch sent_to_edit valid 2",
			Ef:        *unmarshalFromTestFile(t, "testdata/test_parallel_intersected_branches_valid2.json"),
			WantValid: true,
		},
		{
			Name:      "intersected branch inside parall bad 2",
			Ef:        *unmarshalFromTestFile(t, "testdata/test_parallel_intersected_branches_bad2.json"),
			WantValid: false,
		},
		{
			Name:      "intersected branch inside parall bad 3",
			Ef:        *unmarshalFromTestFile(t, "testdata/test_parallel_intersected_branches_bad3.json"),
			WantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			isValid, _ := tt.Ef.Pipeline.Blocks.IsParallelNodesCorrect(nil)
			if isValid != true {
				t.Errorf("unexpected invalid %+v", tt.Ef.Pipeline.Blocks)
			}
		})
	}
}

func unmarshalFromTestFile(t *testing.T, in string) *entity.EriusScenario {
	bytes, err := os.ReadFile(in)
	if err != nil {
		t.Fatal(err)
	}

	var result entity.EriusScenario

	err = json.Unmarshal(bytes, &result)
	if err != nil {
		t.Fatal(err)
	}

	return &result
}

func unmarshalGroupsFromTestFile(t *testing.T, in string) []*entity.NodeGroup {
	bytes, err := os.ReadFile(in)
	if err != nil {
		t.Fatal(err)
	}

	var result []*entity.NodeGroup

	err = json.Unmarshal(bytes, &result)
	if err != nil {
		t.Fatal(err)
	}

	return result
}

func TestValidation_GroupNodes(t *testing.T) {
	tests := []struct {
		Name        string
		Ef          entity.EriusScenario
		WantedGroup []*entity.NodeGroup
	}{
		{
			Name:        "OnlyOneLine",
			Ef:          *unmarshalFromTestFile(t, "testdata/test_groups_one_line.json"),
			WantedGroup: unmarshalGroupsFromTestFile(t, "testdata/test_groups_one_line_result.json"),
		},

		{
			Name:        "OneParallel",
			Ef:          *unmarshalFromTestFile(t, "testdata/test_groups_one_parallel.json"),
			WantedGroup: unmarshalGroupsFromTestFile(t, "testdata/test_groups_one_parallel_result.json"),
		},

		{
			Name:        "ParallelInside",
			Ef:          *unmarshalFromTestFile(t, "testdata/test_groups_parallel_inside.json"),
			WantedGroup: unmarshalGroupsFromTestFile(t, "testdata/test_groups_parallel_inside_result.json"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			groups, _ := tt.Ef.Pipeline.Blocks.GetGroups()
			if !checkEqualityOfGroups(groups, tt.WantedGroup) {
				t.Errorf("unexpected group %v, \n  %v", groups, tt.WantedGroup)
			}
		})
	}
}

type NodeGroupMap struct {
	EndNode   string                  `json:"end_node"`
	Nodes     map[string]NodeGroupMap `json:"nodes"`
	Prev      string                  `json:"prev"`
	StartNode string                  `json:"start_node"`
}

func checkEqualityOfGroups(g1, g2 []*entity.NodeGroup) bool {
	if len(g1) != len(g2) {
		return false
	}

	gm1 := groupSliceToMap(g1)
	gm2 := groupSliceToMap(g2)

	return cmp.Equal(gm1, gm2)
}

func groupSliceToMap(g []*entity.NodeGroup) map[string]NodeGroupMap {
	if g == nil {
		return nil
	}

	gmap := map[string]NodeGroupMap{}

	for i := range g {
		gmap[g[i].StartNode] = NodeGroupMap{
			EndNode:   g[i].EndNode,
			Nodes:     groupSliceToMap(g[i].Nodes),
			Prev:      g[i].Prev,
			StartNode: g[i].StartNode,
		}
	}

	return gmap
}

func Test_validateMappingAndResetIfNotValid(t *testing.T) {
	log := logger.GetLogger(context.TODO())

	pipeline := *unmarshalFromTestFile(t, "testdata/mapping_validation.json")

	pipelineResult, err := os.ReadFile("testdata/mapping_validation_result.json")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		bt         entity.BlocksType
		want       bool
		wantResult string
	}{
		{
			name:       "success case",
			bt:         pipeline.Pipeline.Blocks,
			want:       false,
			wantResult: string(pipelineResult),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := validateMappingAndResetIfNotValid(tt.bt, log)

			var marshaledResult []byte
			marshaledResult, err = json.Marshal(tt.bt)
			assert.Nil(t, err)

			var prettyJSON bytes.Buffer
			err = json.Indent(&prettyJSON, marshaledResult, "", "  ")
			assert.Nil(t, err)

			resultString := string(prettyJSON.Bytes())

			assert.Equalf(t, tt.want, isValid, "validateMapping(%v)", tt.bt)
			assert.Equalf(t, tt.wantResult, resultString, "validateMappingResult(%v)", tt.bt)
		})
	}
}

func Test_validateMappingNotification(t *testing.T) {
	log := logger.GetLogger(context.TODO())

	pipeline := *unmarshalFromTestFile(t, "testdata/notification_validation.json")

	tests := []struct {
		name       string
		bt         entity.BlocksType
		want       bool
		wantResult string
	}{
		{
			name: "success case",
			bt:   pipeline.Pipeline.Blocks,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := validateMappingAndResetIfNotValid(tt.bt, log)

			assert.Equalf(t, tt.want, isValid, "validateMapping(%v)", tt.bt)
		})
	}
}

func Test_validateMappingMultiSelect(t *testing.T) {
	log := logger.GetLogger(context.TODO())

	pipeline := *unmarshalFromTestFile(t, "testdata/multiselect_validation.json")

	tests := []struct {
		name string
		bt   entity.BlocksType
		want bool
	}{
		{
			name: "success case",
			bt:   pipeline.Pipeline.Blocks,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := validateMappingAndResetIfNotValid(tt.bt, log)

			assert.Equalf(t, tt.want, isValid, "validateMapping(%v)", tt.bt)
		})
	}
}
