package api

import (
	"net/http"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

type GetApproveActionNamesResponse struct {
	Id    string `json:"id"`
	Title string `json:"title"`
}

func (ae *APIEnv) GetTaskEventSchema(w http.ResponseWriter, r *http.Request) {
	ctx, s := trace.StartSpan(r.Context(), "get_task_event_schema")
	defer s.End()

	log := logger.GetLogger(ctx)

	type eventSchemaProperties struct {
		TaskID     script.JSONSchemaPropertiesValue `json:"task_id"`
		TaskStatus script.JSONSchemaPropertiesValue `json:"task_status"`
		WorkNumber script.JSONSchemaPropertiesValue `json:"work_number"`
		NodeName   script.JSONSchemaPropertiesValue `json:"node_name"`
		NodeStart  script.JSONSchemaPropertiesValue `json:"node_start"`
		NodeEnd    script.JSONSchemaPropertiesValue `json:"node_end"`
		NodeStatus script.JSONSchemaPropertiesValue `json:"node_status"`
		NodeOutput script.JSONSchemaPropertiesValue `json:"node_output"`
	}

	type eventSchema struct {
		Type       string                `json:"type"`
		Properties eventSchemaProperties `json:"properties"`
	}

	schema := eventSchema{
		Type: "object",
		Properties: eventSchemaProperties{
			TaskID: script.JSONSchemaPropertiesValue{
				Type:   "string",
				Format: "uuid",
				Title:  "Идентификатор процесса",
			},
			WorkNumber: script.JSONSchemaPropertiesValue{
				Type:  "string",
				Title: "Номер заявки ",
			},
			NodeName: script.JSONSchemaPropertiesValue{
				Type:  "string",
				Title: "Название ноды",
			},
			NodeStart: script.JSONSchemaPropertiesValue{
				Type:   "string",
				Format: "date-time",
				Title:  "Дата старта ноды",
			},
			NodeEnd: script.JSONSchemaPropertiesValue{
				Type:   "string",
				Format: "date-time",
				Title:  "Дата окончания ноды",
			},
			TaskStatus: script.JSONSchemaPropertiesValue{
				Type:  "string",
				Title: "Статус процесса",
			},
			NodeStatus: script.JSONSchemaPropertiesValue{
				Type:  "string",
				Title: "Статус ноды",
			},
			NodeOutput: script.JSONSchemaPropertiesValue{
				Type:       "object",
				Title:      "Выходные параметры ноды",
				Properties: script.JSONSchemaProperties{},
			},
		},
	}

	if err := sendResponse(w, http.StatusOK, schema); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

//nolint:dupl //its not duplicate
func (ae *APIEnv) GetApproveActionNames(w http.ResponseWriter, r *http.Request) {
	ctx, s := trace.StartSpan(r.Context(), "get_approve_action_names")
	defer s.End()

	log := logger.GetLogger(ctx)

	data, err := ae.DB.GetApproveActionNames(ctx)
	if err != nil {
		e := UnknownError
		log.Error(err)
		_ = e.sendError(w)

		return
	}

	res := make([]GetApproveActionNamesResponse, 0, len(data))
	for i := range data {
		res = append(res, GetApproveActionNamesResponse{
			Id:    data[i].Id,
			Title: data[i].Title,
		})
	}

	if err = sendResponse(w, http.StatusOK, res); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

type GetApproveStatusesResponse struct {
	Id    string `json:"id"`
	Title string `json:"title"`
}

//nolint:dupl //its not duplicate
func (ae *APIEnv) GetApproveStatuses(w http.ResponseWriter, r *http.Request) {
	ctx, s := trace.StartSpan(r.Context(), "get_approve_statuses")
	defer s.End()

	log := logger.GetLogger(ctx)

	data, err := ae.DB.GetApproveStatuses(ctx)
	if err != nil {
		e := UnknownError
		log.Error(err)
		_ = e.sendError(w)

		return
	}

	res := make([]GetApproveStatusesResponse, 0, len(data))
	for i := range data {
		res = append(res, GetApproveStatusesResponse{
			Id:    data[i].Id,
			Title: data[i].Title,
		})
	}

	if err = sendResponse(w, http.StatusOK, res); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

// GetNodeDecisions returns all decisions by nodes.
func (ae *APIEnv) GetNodeDecisions(w http.ResponseWriter, r *http.Request) {
	ctx, s := trace.StartSpan(r.Context(), "get_node_decisions")
	defer s.End()

	log := logger.GetLogger(ctx)

	data, err := ae.DB.GetNodeDecisions(ctx)
	if err != nil {
		log.Error(err)
		_ = GetDecisionsError.sendError(w)

		return
	}

	res := make([]NodeDecisions, 0, len(data))
	for i := range data {
		res = append(res, NodeDecisions{
			Id:       data[i].Id,
			NodeType: data[i].NodeType,
			Decision: data[i].Decision,
			Title:    data[i].Title,
		})
	}

	if err = sendResponse(w, http.StatusOK, res); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}
