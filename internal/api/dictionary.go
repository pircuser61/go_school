package api

import (
	"net/http"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

type GetApproveActionNamesResponse struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

func (ae *Env) GetTaskEventSchema(w http.ResponseWriter, r *http.Request) {
	ctx, s := trace.StartSpan(r.Context(), "get_task_event_schema")
	defer s.End()

	log := logger.GetLogger(ctx).
		WithField("mainFuncName", "GetTaskEventSchema").
		WithField("method", "get").
		WithField("transport", "rest").
		WithField("traceID", s.SpanContext().TraceID.String()).
		WithField("logVersion", "v1")
	errorHandler := newHTTPErrorHandler(log, w)

	type eventSchemaProperties struct {
		TaskID        script.JSONSchemaPropertiesValue `json:"task_id"`
		TaskStatus    script.JSONSchemaPropertiesValue `json:"task_status"`
		WorkNumber    script.JSONSchemaPropertiesValue `json:"work_number"`
		NodeName      script.JSONSchemaPropertiesValue `json:"node_name"`
		NodeShortName script.JSONSchemaPropertiesValue `json:"node_short_name"`
		NodeStart     script.JSONSchemaPropertiesValue `json:"node_start"`
		NodeEnd       script.JSONSchemaPropertiesValue `json:"node_end"`
		NodeStatus    script.JSONSchemaPropertiesValue `json:"node_status"`
		NodeOutput    script.JSONSchemaPropertiesValue `json:"node_output"`
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
				Title: "Техническое название ноды",
			},
			NodeShortName: script.JSONSchemaPropertiesValue{
				Type:  "string",
				Title: "Краткое название ноды",
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
		errorHandler.handleError(UnknownError, err)

		return
	}
}

//nolint:dupl //its not duplicate
func (ae *Env) GetApproveActionNames(w http.ResponseWriter, r *http.Request) {
	ctx, s := trace.StartSpan(r.Context(), "get_approve_action_names")
	defer s.End()

	log := logger.GetLogger(ctx).
		WithField("mainFuncName", "GetApproveActionNames").
		WithField("method", "get").
		WithField("transport", "rest").
		WithField("traceID", s.SpanContext().TraceID.String()).
		WithField("logVersion", "v1")
	errorHandler := newHTTPErrorHandler(log, w)

	data, err := ae.DB.GetApproveActionNames(ctx)
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}

	res := make([]GetApproveActionNamesResponse, 0, len(data))
	for i := range data {
		res = append(res, GetApproveActionNamesResponse{
			ID:    data[i].ID,
			Title: data[i].Title,
		})
	}

	if err = sendResponse(w, http.StatusOK, res); err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

type GetApproveStatusesResponse struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

//nolint:dupl //its not duplicate
func (ae *Env) GetApproveStatuses(w http.ResponseWriter, r *http.Request) {
	ctx, s := trace.StartSpan(r.Context(), "get_approve_statuses")
	defer s.End()

	log := logger.GetLogger(ctx).WithField("mainFuncName", "GetApproveStatuses").
		WithField("method", "get").
		WithField("transport", "rest").
		WithField("traceID", s.SpanContext().TraceID.String()).
		WithField("logVersion", "v1")
	errorHandler := newHTTPErrorHandler(log, w)

	data, err := ae.DB.GetApproveStatuses(ctx)
	if err != nil {
		log.WithField("funcName", "GetApproveStatuses").
			Error(err)
		errorHandler.handleError(UnknownError, err)

		return
	}

	res := make([]GetApproveStatusesResponse, 0, len(data))
	for i := range data {
		res = append(res, GetApproveStatusesResponse{
			ID:    data[i].ID,
			Title: data[i].Title,
		})
	}

	if err = sendResponse(w, http.StatusOK, res); err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

// GetNodeDecisions returns all decisions by nodes.
func (ae *Env) GetNodeDecisions(w http.ResponseWriter, r *http.Request) {
	ctx, s := trace.StartSpan(r.Context(), "get_node_decisions")
	defer s.End()

	log := logger.GetLogger(ctx).
		WithField("mainFuncName", "GetNodeDecisions").
		WithField("method", "get").
		WithField("transport", "rest").
		WithField("traceID", s.SpanContext().TraceID.String()).
		WithField("logVersion", "v1")
	errorHandler := newHTTPErrorHandler(log, w)

	data, err := ae.DB.GetNodeDecisions(ctx)
	if err != nil {
		log.WithField("funcName", "GetNodeDecisions").
			Error(err)

		_ = GetDecisionsError.sendError(w)

		return
	}

	res := make([]NodeDecision, 0, len(data))
	for i := range data {
		res = append(res, NodeDecision{
			Id:       data[i].ID,
			NodeType: data[i].NodeType,
			Decision: data[i].Decision,
			Title:    data[i].Title,
		})
	}

	if err = sendResponse(w, http.StatusOK, res); err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}
