package api

import (
	"context"
	"encoding/json"
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
			if tt.WantValid && !tt.Ef.Pipeline.Blocks.EndExists() {
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
			if tt.WantValid && !tt.Ef.Pipeline.Blocks.IsPipelineComplete() {
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
			ok, _ := tt.Ef.Pipeline.Blocks.IsSocketsFilled()
			if tt.WantValid && !ok {
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
			if tt.WantValid && !tt.Ef.Pipeline.Blocks.IsSdBlueprintFilled(context.Background(), sdApi) {
				t.Errorf("unexpected invalid %+v", tt.Ef.Pipeline.Blocks)
			}
		})
	}
}

func TestValidation_ParallelNodes(t *testing.T) {
	const cycleJson = "{\"id\":\"f8d583cc-9a69-40e6-8e37-d22259a225c1\",\"name\":\"JAP-2635\",\"status\":2,\"version_id\":\"b3c8ab52-1627-4d76-90f6-0efb7ef43ce5\",\"author\":\"\",\"tags\":[],\"comment\":\"\",\"comment_rejected\":\"\",\"pipeline\":{\"blocks\":{\"start_0\":{\"x\":0,\"y\":0,\"title\":\"Начало\",\"block_type\":\"go\",\"input\":[],\"output\":[{\"name\":\"workNumber\",\"type\":\"string\",\"global\":\"start_0.workNumber\"},{\"name\":\"initiator\",\"type\":\"SsoPerson\",\"global\":\"start_0.initiator\"}],\"type_id\":\"start\",\"next\":{\"default\":[\"approver_0\"]},\"sockets\":[{\"id\":\"default\",\"title\":\"Выход по умолчанию\",\"actionType\":\"\",\"nextBlockIds\":[\"approver_0\"]}],\"short_title\":\"\"},\"approver_0\":{\"x\":352,\"y\":-33,\"title\":\"Согласование\",\"block_type\":\"go\",\"input\":[],\"output\":[{\"name\":\"approver\",\"type\":\"string\",\"global\":\"approver_0.approver\"},{\"name\":\"decision\",\"type\":\"string\",\"global\":\"approver_0.decision\"},{\"name\":\"comment\",\"type\":\"string\",\"global\":\"approver_0.comment\"}],\"params\":{\"type\":\"user\",\"approver\":\"sobugreye1\",\"sla\":0,\"check_sla\":false,\"rework_sla\":16,\"check_rework_sla\":false,\"work_type\":null,\"repeat_prev_decision\":false,\"is_editable\":false,\"approvers_group_id\":\"\",\"approvers_group_name\":\"\",\"approvers_group_id_path\":\"\",\"approve_status_name\":\"На согласовании\",\"forms_accessibility\":[],\"approvementRule\":\"\"},\"type_id\":\"approver\",\"next\":{\"approve\":[\"begin_parallel_task_0\"],\"reject\":[\"begin_parallel_task_0\"]},\"sockets\":[{\"id\":\"approve\",\"title\":\"Согласовать\",\"nextBlockIds\":[\"begin_parallel_task_0\"],\"actionType\":\"primary\"},{\"id\":\"reject\",\"title\":\"Отклонить\",\"nextBlockIds\":[\"begin_parallel_task_0\"],\"actionType\":\"secondary\"}],\"param_type\":\"approver\",\"short_title\":\"овыдровдыаол\"},\"begin_parallel_task_0\":{\"x\":814,\"y\":2,\"title\":\"begin_parallel_task\",\"block_type\":\"go\",\"input\":[],\"output\":[],\"type_id\":\"begin_parallel_task\",\"next\":{\"default\":[\"execution_0\",\"approver_1\"]},\"sockets\":[{\"id\":\"default\",\"title\":\"Выход по умолчанию\",\"nextBlockIds\":[\"execution_0\",\"approver_1\"],\"actionType\":\"\"}],\"short_title\":\"\"},\"execution_0\":{\"x\":1274,\"y\":-295,\"title\":\"Исполнение\",\"block_type\":\"go\",\"input\":[],\"output\":[{\"name\":\"login\",\"type\":\"string\",\"global\":\"execution_0.login\"},{\"name\":\"decision\",\"type\":\"string\",\"global\":\"execution_0.decision\"},{\"name\":\"comment\",\"type\":\"string\",\"global\":\"execution_0.comment\"}],\"params\":{\"type\":\"user\",\"executors\":\"sobugreye1\",\"sla\":0,\"check_sla\":false,\"rework_sla\":16,\"check_rework_sla\":false,\"work_type\":null,\"repeat_prev_decision\":false,\"is_editable\":false,\"executors_group_id\":\"\",\"executors_group_name\":\"\",\"executors_group_id_path\":\"\",\"forms_accessibility\":[],\"use_actual_executor\":false},\"type_id\":\"execution\",\"next\":{\"executed\":[\"wait_for_all_inputs_0\"],\"not_executed\":[\"wait_for_all_inputs_0\"]},\"sockets\":[{\"id\":\"executed\",\"title\":\"Исполнено\",\"nextBlockIds\":[\"wait_for_all_inputs_0\"],\"actionType\":\"\"},{\"id\":\"not_executed\",\"title\":\"Не исполнено\",\"nextBlockIds\":[\"wait_for_all_inputs_0\"],\"actionType\":\"\"}],\"param_type\":\"execution\",\"short_title\":\"asfsafl\"},\"approver_1\":{\"x\":1288,\"y\":130,\"title\":\"Согласование\",\"block_type\":\"go\",\"input\":[],\"output\":[{\"name\":\"approver\",\"type\":\"string\",\"global\":\"approver_1.approver\"},{\"name\":\"decision\",\"type\":\"string\",\"global\":\"approver_1.decision\"},{\"name\":\"comment\",\"type\":\"string\",\"global\":\"approver_1.comment\"}],\"params\":{\"type\":\"user\",\"approver\":\"sobugreye1\",\"sla\":0,\"check_sla\":false,\"rework_sla\":16,\"check_rework_sla\":false,\"work_type\":null,\"repeat_prev_decision\":false,\"is_editable\":true,\"approvers_group_id\":\"\",\"approvers_group_name\":\"\",\"approvers_group_id_path\":\"\",\"approve_status_name\":\"На согласовании\",\"forms_accessibility\":[],\"approvementRule\":\"\"},\"type_id\":\"approver\",\"next\":{\"approve\":[\"execution_1\"],\"reject\":[\"execution_1\"],\"approver_send_edit_app\":[\"approver_0\"]},\"sockets\":[{\"id\":\"approve\",\"title\":\"Согласовать\",\"nextBlockIds\":[\"execution_1\"],\"actionType\":\"primary\"},{\"id\":\"reject\",\"title\":\"Отклонить\",\"nextBlockIds\":[\"execution_1\"],\"actionType\":\"secondary\"},{\"id\":\"approver_send_edit_app\",\"title\":\"На доработку\",\"nextBlockIds\":[\"approver_0\"],\"actionType\":\"other\"}],\"param_type\":\"approver\",\"short_title\":\"afdsasdfasdf\"},\"execution_1\":{\"x\":1715,\"y\":213,\"title\":\"Исполнение\",\"block_type\":\"go\",\"input\":[],\"output\":[{\"name\":\"login\",\"type\":\"string\",\"global\":\"execution_1.login\"},{\"name\":\"decision\",\"type\":\"string\",\"global\":\"execution_1.decision\"},{\"name\":\"comment\",\"type\":\"string\",\"global\":\"execution_1.comment\"}],\"params\":{\"type\":\"user\",\"executors\":\"sobugreye1\",\"sla\":0,\"check_sla\":false,\"rework_sla\":16,\"check_rework_sla\":false,\"work_type\":null,\"repeat_prev_decision\":false,\"is_editable\":false,\"executors_group_id\":\"\",\"executors_group_name\":\"\",\"executors_group_id_path\":\"\",\"forms_accessibility\":[],\"use_actual_executor\":false},\"type_id\":\"execution\",\"next\":{\"executed\":[\"wait_for_all_inputs_0\"],\"not_executed\":[\"wait_for_all_inputs_0\"]},\"sockets\":[{\"id\":\"executed\",\"title\":\"Исполнено\",\"nextBlockIds\":[\"wait_for_all_inputs_0\"],\"actionType\":\"\"},{\"id\":\"not_executed\",\"title\":\"Не исполнено\",\"nextBlockIds\":[\"wait_for_all_inputs_0\"],\"actionType\":\"\"}],\"param_type\":\"execution\",\"short_title\":\"asfasfasfd\"},\"wait_for_all_inputs_0\":{\"x\":2344,\"y\":39,\"title\":\"wait_for_all_inputs\",\"block_type\":\"go\",\"input\":[],\"output\":[],\"type_id\":\"wait_for_all_inputs\",\"next\":{\"default\":[\"end_0\"]},\"sockets\":[{\"id\":\"default\",\"title\":\"Выход по умолчанию\",\"nextBlockIds\":[\"end_0\"],\"actionType\":\"\"}],\"short_title\":\"\"},\"end_0\":{\"x\":2602,\"y\":44,\"title\":\"Конец\",\"block_type\":\"go\",\"input\":[],\"output\":[],\"type_id\":\"end\",\"next\":{},\"sockets\":[],\"short_title\":\"\"}},\"entrypoint\":\"start_0\"},\"input\":[],\"output\":[]}"
	const validJson = "{\"id\":\"f8d583cc-9a69-40e6-8e37-d22259a225c1\",\"name\":\"JAP-2635\",\"status\":1,\"version_id\":\"2b930d16-661b-4e97-9d5d-6b481e61e1fd\",\"author\":\"\",\"tags\":[],\"comment\":\"\",\"comment_rejected\":\"\",\"pipeline\":{\"blocks\":{\"approver_0\":{\"x\":352,\"y\":-33,\"title\":\"Согласование\",\"block_type\":\"go\",\"input\":[],\"output\":[{\"name\":\"approver\",\"type\":\"string\",\"global\":\"approver_0.approver\"},{\"name\":\"decision\",\"type\":\"string\",\"global\":\"approver_0.decision\"},{\"name\":\"comment\",\"type\":\"string\",\"global\":\"approver_0.comment\"}],\"params\":{\"sla\":0,\"type\":\"user\",\"approver\":\"sobugreye1\",\"check_sla\":false,\"work_type\":null,\"rework_sla\":16,\"is_editable\":false,\"approvementRule\":\"\",\"check_rework_sla\":false,\"approvers_group_id\":\"\",\"approve_status_name\":\"На согласовании\",\"forms_accessibility\":[],\"approvers_group_name\":\"\",\"repeat_prev_decision\":false,\"approvers_group_id_path\":\"\"},\"type_id\":\"approver\",\"next\":{\"approve\":[\"begin_parallel_task_0\"],\"reject\":[\"begin_parallel_task_0\"]},\"sockets\":[{\"id\":\"approve\",\"title\":\"Согласовать\",\"nextBlockIds\":[\"begin_parallel_task_0\"],\"actionType\":\"primary\"},{\"id\":\"reject\",\"title\":\"Отклонить\",\"nextBlockIds\":[\"begin_parallel_task_0\"],\"actionType\":\"secondary\"}],\"param_type\":\"approver\",\"short_title\":\"овыдровдыаол\"},\"approver_1\":{\"x\":1288,\"y\":130,\"title\":\"Согласование\",\"block_type\":\"go\",\"input\":[],\"output\":[{\"name\":\"approver\",\"type\":\"string\",\"global\":\"approver_1.approver\"},{\"name\":\"decision\",\"type\":\"string\",\"global\":\"approver_1.decision\"},{\"name\":\"comment\",\"type\":\"string\",\"global\":\"approver_1.comment\"}],\"params\":{\"type\":\"user\",\"approver\":\"sobugreye1\",\"sla\":0,\"check_sla\":false,\"rework_sla\":16,\"check_rework_sla\":false,\"work_type\":null,\"repeat_prev_decision\":false,\"is_editable\":false,\"approvers_group_id\":\"\",\"approvers_group_name\":\"\",\"approvers_group_id_path\":\"\",\"approve_status_name\":\"На согласовании\",\"forms_accessibility\":[],\"approvementRule\":\"\"},\"type_id\":\"approver\",\"next\":{\"approve\":[\"execution_1\"],\"reject\":[\"execution_1\"]},\"sockets\":[{\"id\":\"approve\",\"title\":\"Согласовать\",\"nextBlockIds\":[\"execution_1\"],\"actionType\":\"primary\"},{\"id\":\"reject\",\"title\":\"Отклонить\",\"nextBlockIds\":[\"execution_1\"],\"actionType\":\"secondary\"}],\"param_type\":\"approver\",\"short_title\":\"afdsasdfasdf\"},\"begin_parallel_task_0\":{\"x\":814,\"y\":2,\"title\":\"begin_parallel_task\",\"block_type\":\"go\",\"input\":[],\"output\":[],\"type_id\":\"begin_parallel_task\",\"next\":{\"default\":[\"execution_0\",\"approver_1\"]},\"sockets\":[{\"id\":\"default\",\"title\":\"Выход по умолчанию\",\"nextBlockIds\":[\"execution_0\",\"approver_1\"],\"actionType\":\"\"}],\"short_title\":\"\"},\"end_0\":{\"x\":2602,\"y\":44,\"title\":\"Конец\",\"block_type\":\"go\",\"input\":[],\"output\":[],\"type_id\":\"end\",\"next\":{},\"sockets\":[],\"short_title\":\"\"},\"execution_0\":{\"x\":1274,\"y\":-295,\"title\":\"Исполнение\",\"block_type\":\"go\",\"input\":[],\"output\":[{\"name\":\"login\",\"type\":\"string\",\"global\":\"execution_0.login\"},{\"name\":\"decision\",\"type\":\"string\",\"global\":\"execution_0.decision\"},{\"name\":\"comment\",\"type\":\"string\",\"global\":\"execution_0.comment\"}],\"params\":{\"sla\":0,\"type\":\"user\",\"check_sla\":false,\"executors\":\"sobugreye1\",\"work_type\":null,\"rework_sla\":16,\"is_editable\":false,\"check_rework_sla\":false,\"executors_group_id\":\"\",\"forms_accessibility\":[],\"use_actual_executor\":false,\"executors_group_name\":\"\",\"repeat_prev_decision\":false,\"executors_group_id_path\":\"\"},\"type_id\":\"execution\",\"next\":{\"executed\":[\"wait_for_all_inputs_0\"],\"not_executed\":[\"wait_for_all_inputs_0\"]},\"sockets\":[{\"id\":\"executed\",\"title\":\"Исполнено\",\"nextBlockIds\":[\"wait_for_all_inputs_0\"],\"actionType\":\"\"},{\"id\":\"not_executed\",\"title\":\"Не исполнено\",\"nextBlockIds\":[\"wait_for_all_inputs_0\"],\"actionType\":\"\"}],\"param_type\":\"execution\",\"short_title\":\"asfsafl\"},\"execution_1\":{\"x\":1715,\"y\":213,\"title\":\"Исполнение\",\"block_type\":\"go\",\"input\":[],\"output\":[{\"name\":\"login\",\"type\":\"string\",\"global\":\"execution_1.login\"},{\"name\":\"decision\",\"type\":\"string\",\"global\":\"execution_1.decision\"},{\"name\":\"comment\",\"type\":\"string\",\"global\":\"execution_1.comment\"}],\"params\":{\"sla\":0,\"type\":\"user\",\"check_sla\":false,\"executors\":\"sobugreye1\",\"work_type\":null,\"rework_sla\":16,\"is_editable\":false,\"check_rework_sla\":false,\"executors_group_id\":\"\",\"forms_accessibility\":[],\"use_actual_executor\":false,\"executors_group_name\":\"\",\"repeat_prev_decision\":false,\"executors_group_id_path\":\"\"},\"type_id\":\"execution\",\"next\":{\"executed\":[\"wait_for_all_inputs_0\"],\"not_executed\":[\"wait_for_all_inputs_0\"]},\"sockets\":[{\"id\":\"executed\",\"title\":\"Исполнено\",\"nextBlockIds\":[\"wait_for_all_inputs_0\"],\"actionType\":\"\"},{\"id\":\"not_executed\",\"title\":\"Не исполнено\",\"nextBlockIds\":[\"wait_for_all_inputs_0\"],\"actionType\":\"\"}],\"param_type\":\"execution\",\"short_title\":\"asfasfasfd\"},\"start_0\":{\"x\":0,\"y\":0,\"title\":\"Начало\",\"block_type\":\"go\",\"input\":[],\"output\":[{\"name\":\"workNumber\",\"type\":\"string\",\"global\":\"start_0.workNumber\"},{\"name\":\"initiator\",\"type\":\"SsoPerson\",\"global\":\"start_0.initiator\"}],\"type_id\":\"start\",\"next\":{\"default\":[\"approver_0\"]},\"sockets\":[{\"id\":\"default\",\"title\":\"Выход по умолчанию\",\"nextBlockIds\":[\"approver_0\"],\"actionType\":\"\"}],\"short_title\":\"\"},\"wait_for_all_inputs_0\":{\"x\":2344,\"y\":39,\"title\":\"wait_for_all_inputs\",\"block_type\":\"go\",\"input\":[],\"output\":[],\"type_id\":\"wait_for_all_inputs\",\"next\":{\"default\":[\"end_0\"]},\"sockets\":[{\"id\":\"default\",\"title\":\"Выход по умолчанию\",\"nextBlockIds\":[\"end_0\"],\"actionType\":\"\"}],\"short_title\":\"\"}},\"entrypoint\":\"start_0\"},\"input\":[],\"output\":[]}"

	var valid entity.EriusScenario
	json.Unmarshal([]byte(validJson), &valid)

	var cycleTest entity.EriusScenario
	json.Unmarshal([]byte(cycleJson), &cycleTest)

	tests := []struct {
		Name      string
		Ef        entity.EriusScenario
		WantValid bool
	}{{
		Name:      "Valid",
		Ef:        valid,
		WantValid: true,
	},
		{
			Name:      "cycle returning from parallel",
			Ef:        cycleTest,
			WantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			ok, _ := tt.Ef.Pipeline.Blocks.IsParallelNodesCorrect()
			if tt.WantValid && !ok {
				t.Errorf("unexpected invalid %+v", tt.Ef.Pipeline.Blocks)
			}
		})
	}
}
