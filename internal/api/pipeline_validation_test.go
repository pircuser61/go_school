package api

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/hrishin/httpmock"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc"
)

func TestValidation_EndExists(t *testing.T) {
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
			if tt.Ef.Pipeline.Blocks.EndExists() != tt.WantValid {
				t.Errorf("unexpected invalid %+v", tt.Ef.Pipeline.Blocks)
			}
		})
	}
}

func TestValidation_IsolationNode(t *testing.T) {
	tests := []struct {
		Name      string
		Ef        entity.EriusScenario
		WantValid bool
	}{
		{
			Name: "test valid blocks all blocks related",
			Ef: entity.EriusScenario{
				Pipeline: entity.PipelineType{
					Blocks: entity.BlocksType{
						"start_0": {
							TypeID: "start",
							Sockets: []entity.Socket{
								{
									Id:           script.DefaultSocketID,
									Title:        script.DefaultSocketTitle,
									NextBlockIds: []string{"end_0"},
								},
							},
						},
						"end_0": {
							TypeID: "end",
							Sockets: []entity.Socket{
								{
									Id:           script.DefaultSocketID,
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
			Name: "test invalid blocks start block unrelated",
			Ef: entity.EriusScenario{
				Pipeline: entity.PipelineType{
					Blocks: entity.BlocksType{
						"start_0": {
							TypeID: "start",
							Sockets: []entity.Socket{
								{
									Id:           script.DefaultSocketID,
									Title:        script.DefaultSocketTitle,
									NextBlockIds: []string{},
								},
							},
						},
						"approver_0": {
							TypeID: "approver",
							Sockets: []entity.Socket{
								{
									Id:           script.DefaultSocketID,
									Title:        script.DefaultSocketTitle,
									NextBlockIds: []string{"end_0"},
								},
							},
						},
						"end_0": {
							TypeID: "end",
							Sockets: []entity.Socket{
								{
									Id:           script.DefaultSocketID,
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
			Name: "test invalid blocks approver block unrelated",
			Ef: entity.EriusScenario{
				Pipeline: entity.PipelineType{
					Blocks: entity.BlocksType{
						"start_0": {
							TypeID: "start",
							Sockets: []entity.Socket{
								{
									Id:           script.DefaultSocketID,
									Title:        script.DefaultSocketTitle,
									NextBlockIds: []string{"approver_0"},
								},
							},
						},
						"approver_0": {
							TypeID: "approver",
							Sockets: []entity.Socket{
								{
									Id:           script.DefaultSocketID,
									Title:        script.DefaultSocketTitle,
									NextBlockIds: []string{},
								},
							},
						},
						"end_0": {
							TypeID: "end",
							Sockets: []entity.Socket{
								{
									Id:           script.DefaultSocketID,
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
			Name: "test invalid blocks all blocks unrelated",
			Ef: entity.EriusScenario{
				Pipeline: entity.PipelineType{
					Blocks: entity.BlocksType{
						"start_0": {
							TypeID: "start",
							Sockets: []entity.Socket{
								{
									Id:           script.DefaultSocketID,
									Title:        script.DefaultSocketTitle,
									NextBlockIds: []string{},
								},
							},
						},
						"approver_0": {
							TypeID: "approver",
							Sockets: []entity.Socket{
								{
									Id:           script.DefaultSocketID,
									Title:        script.DefaultSocketTitle,
									NextBlockIds: []string{},
								},
							},
						},
						"end_0": {
							TypeID: "end",
							Sockets: []entity.Socket{
								{
									Id:           script.DefaultSocketID,
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
			Name: "test invalid blocks cycle + unrelated",
			Ef: entity.EriusScenario{
				Pipeline: entity.PipelineType{
					Blocks: entity.BlocksType{
						"start_0": {
							TypeID: "start",
							Sockets: []entity.Socket{
								{
									Id:           script.DefaultSocketID,
									Title:        script.DefaultSocketTitle,
									NextBlockIds: []string{"form_0"},
								},
							},
						},
						"form_0": {
							TypeID: "form",
							Sockets: []entity.Socket{
								{
									Id:           script.DefaultSocketID,
									Title:        script.DefaultSocketTitle,
									NextBlockIds: []string{"approver_0"},
								},
							},
						},
						"approver_0": {
							TypeID: "approver",
							Sockets: []entity.Socket{
								{
									Id:           script.DefaultSocketID,
									Title:        script.DefaultSocketTitle,
									NextBlockIds: []string{"form_0"},
								},
							},
						},
						"end_0": {
							TypeID: "end",
							Sockets: []entity.Socket{
								{
									Id:           script.DefaultSocketID,
									Title:        script.DefaultSocketTitle,
									NextBlockIds: []string{},
								},
							},
						},
						"approver_1": {
							TypeID: "approver",
							Sockets: []entity.Socket{
								{
									Id:           script.DefaultSocketID,
									Title:        script.DefaultSocketTitle,
									NextBlockIds: []string{"end_1"},
								},
							},
						},
						"end_1": {
							TypeID: "end",
							Sockets: []entity.Socket{
								{
									Id:           script.DefaultSocketID,
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
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			if tt.Ef.Pipeline.Blocks.IsPipelineComplete() != tt.WantValid {
				t.Errorf("unexpected invalid %+v", tt.Ef.Pipeline.Blocks)
			}
		})
	}
}

func TestValidation_SocketFilled(t *testing.T) {
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
									Id: "approved",
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
									Id: "approved",
								},
								{
									Id: "rejected",
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
									Id: "approved",
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
									Id: "approved",
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
									Id: "approved",
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
			isValid, _ := tt.Ef.Pipeline.Blocks.IsSocketsFilled()
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
	sdApi := &servicedesc.Service{
		Cli:   httpmock.Client(&mockResponse),
		SdURL: "https://dev.servicedesk.mts.ru",
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			if tt.Ef.Pipeline.Blocks.IsSdBlueprintFilled(context.Background(), sdApi) != tt.WantValid {
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
									Id:           script.DefaultSocketID,
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
									Id:           script.DefaultSocketID,
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
									Id:           "approve",
									Title:        "Согласовать",
									NextBlockIds: []string{"wait_for_all_inputs_0"},
								},
								{
									Id:           "reject",
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
									Id:           "approve",
									Title:        "Согласовать",
									NextBlockIds: []string{"approver_3"},
								},
								{
									Id:           "reject",
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
									Id:           "approve",
									Title:        "Согласовать",
									NextBlockIds: []string{"approver_2"},
								},
								{
									Id:           "reject",
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
									Id:           script.DefaultSocketID,
									Title:        script.DefaultSocketTitle,
									NextBlockIds: []string{"end_0"},
								},
							},
						},
						"end_0": {
							TypeID: "end",
							Sockets: []entity.Socket{
								{
									Id:           script.DefaultSocketID,
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
									Id:           script.DefaultSocketID,
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
									Id:           script.DefaultSocketID,
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
									Id:           "approve",
									Title:        "Согласовать",
									NextBlockIds: []string{"wait_for_all_inputs_0"},
								},
								{
									Id:           "reject",
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
									Id:           "approve",
									Title:        "Согласовать",
									NextBlockIds: []string{"approver_3"},
								},
								{
									Id:           "reject",
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
									Id:           "approve",
									Title:        "Согласовать",
									NextBlockIds: []string{"approver_2"},
								},
								{
									Id:           "reject",
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
									Id:           script.DefaultSocketID,
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
									Id:           script.DefaultSocketID,
									Title:        script.DefaultSocketTitle,
									NextBlockIds: []string{"end_0"},
								},
							},
						},
						"end_0": {
							TypeID: "end",
							Sockets: []entity.Socket{
								{
									Id:           script.DefaultSocketID,
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
			Ef:        *unmarshalFromTestFile(t, "testdata/test_parallel_mixed_branches_bad1.json"),
			WantValid: false,
		},
		{
			Name:      "intersected branch valid 1",
			Ef:        *unmarshalFromTestFile(t, "testdata/test_parallel_mixed_branches_valid1.json"),
			WantValid: true,
		},
		{
			Name:      "intersected branch sent_to_edit valid 2",
			Ef:        *unmarshalFromTestFile(t, "testdata/test_parallel_mixed_branches_valid2.json"),
			WantValid: true,
		},
		{
			Name:      "intersected branch inside parall bad 2",
			Ef:        *unmarshalFromTestFile(t, "testdata/test_parallel_mixed_branches_bad2.json"),
			WantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			isValid, _ := tt.Ef.Pipeline.Blocks.IsParallelNodesCorrect()
			if isValid != tt.WantValid {
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
