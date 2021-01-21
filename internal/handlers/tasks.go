package handlers

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/google/uuid"
	"go.opencensus.io/trace"
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
//nolint:dupl //diff logic
func (ae *APIEnv) GetTask(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "get_task")
	defer s.End()

	idParam := chi.URLParam(req, "taskID")

	id, err := uuid.Parse(idParam)
	if err != nil {
		e := UUIDParsingError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	resp, err := ae.DB.GetTask(ctx, id)
	if err != nil {
		e := GetTaskError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	steps, err := ae.DB.GetTaskSteps(ctx, id)
	if err != nil {
		e := GetTaskError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	resp.Steps = steps

	if err := sendResponse(w, http.StatusOK, resp); err != nil {
		e := UnknownError
		ae.Logger.Error(e.errorMessage(err))
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
// @Router /tasks/{pipelineID} [get]
//nolint:dupl //diff logic
func (ae *APIEnv) GetPipelineTasks(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "get_pipeline_logs")
	defer s.End()

	idParam := chi.URLParam(req, "pipelineID")

	id, err := uuid.Parse(idParam)
	if err != nil {
		e := UUIDParsingError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	resp, err := ae.DB.GetPipelineTasks(ctx, id)
	if err != nil {
		e := GetTasksError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if err := sendResponse(w, http.StatusOK, resp); err != nil {
		e := UnknownError
		ae.Logger.Error(e.errorMessage(err))
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

	idParam := chi.URLParam(req, "versionID")

	id, err := uuid.Parse(idParam)
	if err != nil {
		e := UUIDParsingError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	resp, err := ae.DB.GetVersionTasks(ctx, id)
	if err != nil {
		e := GetTasksError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if err := sendResponse(w, http.StatusOK, resp); err != nil {
		e := UnknownError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}
