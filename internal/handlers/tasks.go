package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/google/uuid"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

// GetTask
// @Summary Get Task
// @Description Получить экземпляр задачи
// @Tags tasks
// @ID      get-task-entity
// @Produce json
// @Param taskID path string true "Task ID"
// @success 200 {object} httpResponse{data=entity.EriusTask}
// @Failure 400 {object} httpError
// @Failure 401 {object} httpError
// @Failure 500 {object} httpError
// @Router /tasks/{taskID} [get]
func (ae *APIEnv) GetTask(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "get_task")
	defer s.End()

	log := logger.GetLogger(ctx)

	idParam := chi.URLParam(req, "taskID")

	id, err := uuid.Parse(idParam)
	if err != nil {
		e := UUIDParsingError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	resp, err := ae.DB.GetTask(ctx, id)
	if err != nil {
		e := GetTaskError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	steps, err := ae.DB.GetTaskSteps(ctx, id)
	if err != nil {
		e := GetTaskError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	resp.Steps = steps

	if err := sendResponse(w, http.StatusOK, resp); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

func compileGetTasksFilters(req *http.Request) (filters entity.TaskFilter, err error) {
	user, err := GetEffectiveUserInfoFromCtx(req.Context())
	if err != nil {
		return filters, err
	}
	filters.CurrentUser = user.Username

	taskIDs := req.URL.Query().Get("taskIDs")
	if taskIDs != "" {
		ids := strings.Split(taskIDs, ",")
		filters.TaskIDs = &ids
	}

	name := req.URL.Query().Get("name")
	if name != "" {
		filters.Name = &name
	}

	createdStart := req.URL.Query().Get("created[start]")
	if createdStart != "" {
		createdEnd := req.URL.Query().Get("created[end]")
		if createdEnd != "" {
			st, convErr := strconv.Atoi(createdStart)
			if convErr != nil {
				return filters, convErr
			}

			end, convErr := strconv.Atoi(createdEnd)
			if convErr != nil {
				return filters, convErr
			}

			filters.Created = &entity.TimePeriod{
				Start: st,
				End:   end,
			}
		}
	}

	order := req.URL.Query().Get("order")
	if order != "" {
		filters.Order = &order
	}

	lim := 10
	limit := req.URL.Query().Get("limit")
	if limit != "" {
		lim, err = strconv.Atoi(limit)
		if err != nil {
			return
		}
	}
	filters.Limit = &lim

	off := 0
	offset := req.URL.Query().Get("offset")
	if offset != "" {
		off, err = strconv.Atoi(offset)
		if err != nil {
			return
		}
	}
	filters.Offset = &off

	return
}

// GetTasks
// @Summary Get Tasks
// @Description Получить задачи
// @Tags pipeline, tasks
// @ID      get-tasks
// @Produce json
// @Param name query string false "Pipeline name"
// @Param taskIDs query []string false "Task IDs"
// @Param created[start] query string false "Created after"
// @Param created[end] query string false "Created before"
// @Param order query string false "Order"
// @Param limit query string false "Limit"
// @Param offset query string false "Offset"
// @success 200 {object} httpResponse{data=entity.EriusTasksPage}
// @Failure 400 {object} httpError
// @Failure 401 {object} httpError
// @Failure 500 {object} httpError
// @Router /tasks [get]
//nolint:dupl //diff logic
func (ae *APIEnv) GetTasks(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "get_tasks")
	defer s.End()

	log := logger.GetLogger(ctx)

	filters, err := compileGetTasksFilters(req)
	if err != nil {
		e := BadFiltersError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	resp, err := ae.DB.GetTasks(ctx, filters)
	if err != nil {
		e := GetTasksError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if err := sendResponse(w, http.StatusOK, resp); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

// GetPipelineTasks
// @Summary Get Pipeline Tasks
// @Description Получить задачи по сценарию
// @Tags pipeline, tasks
// @ID      get-pipeline-tasks
// @Produce json
// @Param pipelineID path string true "Pipeline ID"
// @success 200 {object} httpResponse{data=entity.EriusTasks}
// @Failure 400 {object} httpError
// @Failure 401 {object} httpError
// @Failure 500 {object} httpError
// @Router /tasks/pipeline/{pipelineID} [get]
//nolint:dupl //diff logic
func (ae *APIEnv) GetPipelineTasks(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "get_pipeline_tasks")
	defer s.End()

	log := logger.GetLogger(ctx)

	idParam := chi.URLParam(req, "pipelineID")

	id, err := uuid.Parse(idParam)
	if err != nil {
		e := UUIDParsingError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	resp, err := ae.DB.GetPipelineTasks(ctx, id)
	if err != nil {
		e := GetTasksError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if err := sendResponse(w, http.StatusOK, resp); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

// GetVersionTasks
// @Summary Get Version Tasks
// @Description Получить задачи по версии сценарию
// @Tags version, tasks
// @ID      get-version-tasks
// @Produce json
// @Param versionID path string true "Version ID"
// @success 200 {object} httpResponse{data=entity.EriusTasks}
// @Failure 400 {object} httpError
// @Failure 401 {object} httpError
// @Failure 500 {object} httpError
// @Router /tasks/version/{versionID} [get]
//nolint:dupl //diff logic
func (ae *APIEnv) GetVersionTasks(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "get_version_logs")
	defer s.End()

	log := logger.GetLogger(ctx)

	idParam := chi.URLParam(req, "versionID")

	id, err := uuid.Parse(idParam)
	if err != nil {
		e := UUIDParsingError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	resp, err := ae.DB.GetVersionTasks(ctx, id)
	if err != nil {
		e := GetTasksError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if err := sendResponse(w, http.StatusOK, resp); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}
