package api

import (
	"context"
	"errors"
	"math"
	"net/http"

	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/metrics"
	"go.opencensus.io/trace"
)

func (ae *Env) RetryTasks(w http.ResponseWriter, r *http.Request, params RetryTasksParams) {
	ctx, span := trace.StartSpan(r.Context(), "retry_tasks")
	defer span.End()

	errorHandler := newHTTPErrorHandler(
		logger.GetLogger(ctx).WithField("funcName", "RetryTasks"),
		w,
	)

	ctx = logger.WithLogger(ctx, errorHandler.log)

	limit := math.MaxInt
	if params.Limit != nil && *params.Limit > 0 {
		limit = *params.Limit
	}

	_, err := ae.retryEmptyTasks(ctx, limit)
	if err != nil {
		httpErr := getErr(err)

		errorHandler.handleError(httpErr, err)

		return
	}

	err = sendResponse(w, http.StatusOK, nil)
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

func (ae *Env) retryEmptyTasks(ctx context.Context, limit int) (retried int, err error) {
	log := logger.GetLogger(ctx)

	emptyTasks, err := ae.DB.EmptyTasks(ctx, ae.TaskRetry.MinLifetime, ae.TaskRetry.MaxLifetime, limit)
	if err != nil {
		return 0, errors.Join(GetTaskError, err)
	}

	for _, emptyTask := range emptyTasks {
		processErr := ae.processEmptyTask(ctx, ae.DB, emptyTask, "", &metrics.RequestInfo{})
		if processErr != nil {
			log.WithError(processErr).
				WithFields(
					logger.Fields{
						"workId":     emptyTask.WorkID,
						"workNumber": emptyTask.WorkNumber,
					},
				).
				Error("process empty task error")
		}
	}

	return len(emptyTasks), nil
}
