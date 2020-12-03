package handlers

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/google/uuid"
	"go.opencensus.io/trace"
)

// GetTaskSteps
// @Summary Get Task Steps
// @Description Получить логи по задаче
// @Tags tasks log
// @ID      get-task-log
// @Produce json
// @Param taskID path string true "Task ID"
// @success 200 {object} httpResponse{data=entity.EriusLog}
// @Failure 400 {object} httpError
// @Failure 401 {object} httpError
// @Failure 500 {object} httpError
// @Router /tasks/{taskID} [get]
//nolint:dupl //diff logic
func (ae *APIEnv) GetTask(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "get_version_logs")
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
		e := GetLogError
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

// LastVersionDebugTask
// @Summary Get last work for version
// @Description Получить последнюю сессию выполнения версии сценария
// @Tags tasks log
// @ID      get-last-task-log
// @Produce json
// @Param versionID path string true "Version ID"
// @success 200 {object} httpResponse{data=entity.EriusLog}
// @Failure 400 {object} httpError
// @Failure 401 {object} httpError
// @Failure 500 {object} httpError
// @Router /tasks/last/{versionID} [get]
// nolint:dupl //its unique
func (ae *APIEnv) LastVersionTask(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "get_last_version_tasks")
	defer s.End()

	idParam := chi.URLParam(req, "versionID")

	id, err := uuid.Parse(idParam)
	if err != nil {
		e := UUIDParsingError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	resp, err := ae.DB.GetTask(ctx, id)
	if err != nil {
		e := GetLogError
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

// GetPipelineTasks
// @Summary Get Pipeline Tasks
// @Description Получить задачи по сценарию
// @Tags pipeline tasks
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
// @Tags version tasks
// @ID      get-version-tasks
// @Produce json
// @Param versionID path string true "Version ID"
// @success 200 {object} httpResponse{data=entity.EriusTasks}
// @Failure 400 {object} httpError
// @Failure 401 {object} httpError
// @Failure 500 {object} httpError
// @Router /tasks/version/{pipelineID} [get]
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
