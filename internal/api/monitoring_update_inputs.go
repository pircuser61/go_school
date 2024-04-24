package api

import (
	"encoding/json"
	"io"
	"net/http"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"github.com/google/uuid"
	"go.opencensus.io/trace"

	e "gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/user"
)

//nolint:revive,gocritic,stylecheck
func (ae *Env) MonitoringUpdateBlockInputs(w http.ResponseWriter, r *http.Request) {
	const fn = "MonitoringUpdateBlockInputs"
	ctx, span := trace.StartSpan(r.Context(), "monitoring_update_block_inputs")
	defer span.End()

	log := logger.GetLogger(ctx).
		WithField("funcName", fn)
	errorHandler := newHTTPErrorHandler(log, w)

	b, err := io.ReadAll(r.Body)

	defer r.Body.Close()

	if err != nil {
		errorHandler.handleError(RequestReadError, err)

		return
	}

	txStorage, transactionErr := ae.DB.StartTransaction(ctx)
	if transactionErr != nil {
		log.WithError(transactionErr).Error("couldn't start transaction")

		errorHandler.sendError(UnknownError)

		return
	}

	req := &MonitoringUpdateBlockInputsRequest{}

	err = json.Unmarshal(b, req)
	if err != nil {
		errorHandler.handleError(MonitoringEditBlockParseError, err)
		ae.rollbackTransaction(ctx, txStorage, fn)

		return
	}

	log = log.WithField("stepName", req.StepName).
		WithField("workID", req.WorkId)
	ctx = logger.WithLogger(ctx, log)

	ui, err := user.GetUserInfoFromCtx(ctx)
	if err != nil {
		errorHandler.handleError(NoUserInContextError, err)
		ae.rollbackTransaction(ctx, txStorage, fn)

		return
	}

	eventData := struct {
		Data  map[string]interface{} `json:"data"`
		Steps []uuid.UUID            `json:"steps"`
	}{
		Data:  req.Inputs,
		Steps: []uuid.UUID{},
	}

	// nolint:ineffassign,staticcheck
	jsonParams := json.RawMessage{}

	jsonParams, err = json.Marshal(eventData)
	if err != nil {
		errorHandler.handleError(MarshalEventParamsError, err)
		ae.rollbackTransaction(ctx, txStorage, fn)

		return
	}

	eventID, err := txStorage.CreateTaskEvent(ctx, &e.CreateTaskEvent{
		WorkID:    req.WorkId,
		Author:    ui.Username,
		EventType: string(MonitoringTaskActionRequestActionEdit),
		Params:    jsonParams,
	})
	if err != nil {
		errorHandler.handleError(CreateTaskEventError, err)
		ae.rollbackTransaction(ctx, txStorage, fn)

		return
	}

	err = txStorage.CreateUpdatesInputsHistory(ctx, &e.CreateUpdatesInputsHistory{
		WorkID:   req.WorkId,
		EventID:  eventID,
		StepName: req.StepName,
		Author:   ui.Username,
		Inputs:   req.Inputs,
	})
	if err != nil {
		errorHandler.handleError(CreateUpdatesInputsHistoryError, err)
		ae.rollbackTransaction(ctx, txStorage, fn)

		return
	}

	err = txStorage.CommitTransaction(ctx)
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}

	err = sendResponse(w, http.StatusOK, nil)
	if err != nil {
		errorHandler.handleError(UnknownError, err)
	}
}
