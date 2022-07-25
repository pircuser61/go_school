package api

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/google/uuid"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

const (
	statusRunned = "runned"
)

type RunContext struct {
	ID         string            `json:"id"`
	Parameters map[string]string `json:"parameters"`
}

func (ae *APIEnv) ListSchedulerTasks(w http.ResponseWriter, req *http.Request, pipelineID string) {
	ctx, s := trace.StartSpan(req.Context(), "scheduler tasks list")
	defer s.End()

	log := logger.GetLogger(ctx)

	id, err := uuid.Parse(pipelineID)
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

func (ae *APIEnv) ServePrometheus() http.Handler {
	return promhttp.Handler()
}
