package handlers

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

const (
	statusRunned   = "runned"
	statusFinished = "finished"
	statusError    = "error"
)

type RunContext struct {
	ID         string            `json:"id"`
	Parameters map[string]string `json:"parameters"`
}

// @Summary Active scheduler tasks
// @Description Наличие у сценария активных заданий в шедулере
// @Tags pipeline
// @ID pipeline-scheduler-tasks
// @Accept json
// @Produce json
// @Param pipelineID path string true "Pipeline ID"
// @Success 200 {object} httpResponse{data=entity.SchedulerTasksResponse}
// @Failure 400 {object} httpError
// @Failure 500 {object} httpError
// @Router /pipelines/{pipelineID}/scheduler-tasks [post]
func (ae *APIEnv) ListSchedulerTasks(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "scheduler tasks list")
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

	tasks, err := ae.SchedulerClient.GetTasksByPipelineID(ctx, id)
	if err != nil {
		e := SchedulerClientFailed
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	// в текущей реализации возращаем только факт наличия заданий
	result := &entity.SchedulerTasksResponse{
		Result: len(tasks) > 0,
	}

	err = sendResponse(w, http.StatusOK, result)
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

// Metrics godoc
// @Summary metrics
// @Tags metrics
// @Description Метрики
// @ID metrics
// @Produce plain
// @Success 200 "metrics content"
// @Router /api/pipeliner/v1/metrics [get]
func (ae *APIEnv) ServePrometheus() http.Handler {
	return promhttp.Handler()
}
