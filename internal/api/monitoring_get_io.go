package api

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

//nolint:revive,stylecheck //need to implement interface in api.go
func (ae *Env) MonitoringGetBlockInputs(w http.ResponseWriter, req *http.Request, blockID string) {
	ctx, span := trace.StartSpan(req.Context(), "monitoring_get_block_inputs")
	defer span.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	stepID, err := uuid.Parse(blockID)
	if err != nil {
		errorHandler.handleError(UUIDParsingError, err)
	}

	dbStep, err := ae.DB.GetTaskStepByID(ctx, stepID)
	if err != nil {
		e := UnknownError

		log.WithField("stepID", stepID).
			Error(e.errorMessage(err))
		errorHandler.sendError(e)
	}

	dbWhileRunningInputs, err := ae.DB.GetStepInputs(ctx, dbStep.Name, dbStep.WorkNumber, dbStep.Time)
	if err != nil {
		e := GetBlockContextError

		log.WithField("stepID", stepID).
			WithField("stepName", dbStep.Name).
			Error(e.errorMessage(err))
		errorHandler.sendError(e)

		return
	}

	var dbEditedInputs entity.BlockInputs

	if dbStep.UpdatedAt != nil {
		dbEditedInputs, err = ae.DB.GetEditedStepInputs(ctx, dbStep.Name, dbStep.WorkNumber, *dbStep.UpdatedAt)
		if err != nil {
			e := GetBlockContextError

			log.WithField("stepID", stepID).
				WithField("stepName", dbStep.Name).
				Error(e.errorMessage(err))
			errorHandler.sendError(e)

			return
		}
	}

	blockOutputs, err := ae.DB.GetBlockOutputs(ctx, blockID, dbStep.Name)
	if err != nil {
		e := GetBlockContextError

		log.WithField("stepID", blockID).
			WithField("stepName", dbStep.Name).
			Error(e.errorMessage(err))
		errorHandler.sendError(e)

		return
	}

	blockIsHidden, err := ae.DB.CheckBlockForHiddenFlag(ctx, blockID)
	if err != nil {
		e := CheckForHiddenError

		log.WithField("stepID", blockID).
			WithField("stepName", dbStep.Name).
			Error(e.errorMessage(err))
		errorHandler.sendError(e)

		return
	}

	if blockIsHidden {
		errorHandler.handleError(ForbiddenError, err)

		return
	}

	outputs := make(map[string]MonitoringBlockParam, 0)

	for _, bo := range blockOutputs {
		outputs[bo.Name] = MonitoringBlockParam{
			Name:  bo.Name,
			Value: bo.Value,
			Type:  utils.GetJSONType(bo.Value),
		}
	}

	startedAt := dbStep.Time.String()
	finishedAt := ""

	if dbStep.Status == string(MonitoringHistoryStatusFinished) && dbStep.UpdatedAt != nil {
		finishedAt = dbStep.UpdatedAt.String()
	}

	if err = sendResponse(w, http.StatusOK, MonitoringInputsResponse{
		StartedAt:    &startedAt,
		FinishedAt:   &finishedAt,
		WhileRunning: &MonitoringInputsResponse_WhileRunning{AdditionalProperties: toMonitoringInputs(dbWhileRunningInputs)},
		Edited:       &MonitoringInputsResponse_Edited{AdditionalProperties: toMonitoringInputs(dbEditedInputs)},
	}); err != nil {
		errorHandler.handleError(UnknownError, err)
	}
}

//nolint:revive,stylecheck //need to implement interface in api.go
func (ae *Env) MonitoringGetBlockOutputs(w http.ResponseWriter, req *http.Request, blockID string) {
	ctx, span := trace.StartSpan(req.Context(), "monitoring_get_block_outputs")
	defer span.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	stepID, err := uuid.Parse(blockID)
	if err != nil {
		errorHandler.handleError(UUIDParsingError, err)
	}

	dbStep, err := ae.DB.GetTaskStepByID(ctx, stepID)
	if err != nil {
		e := UnknownError

		log.WithField("stepID", stepID).
			Error(e.errorMessage(err))
		errorHandler.sendError(e)
	}

	blockOutputs, err := ae.DB.GetBlockOutputs(ctx, blockID, dbStep.Name)
	if err != nil {
		e := GetBlockContextError

		log.WithField("stepID", blockID).
			WithField("stepName", dbStep.Name).
			Error(e.errorMessage(err))
		errorHandler.sendError(e)

		return
	}

	blockIsHidden, err := ae.DB.CheckBlockForHiddenFlag(ctx, blockID)
	if err != nil {
		e := CheckForHiddenError

		log.WithField("stepID", blockID).
			WithField("stepName", dbStep.Name).
			Error(e.errorMessage(err))
		errorHandler.sendError(e)

		return
	}

	if blockIsHidden {
		errorHandler.handleError(ForbiddenError, err)

		return
	}

	outputs := make(map[string]MonitoringBlockParam, 0)

	for _, bo := range blockOutputs {
		outputs[bo.Name] = MonitoringBlockParam{
			Name:  bo.Name,
			Value: bo.Value,
			Type:  utils.GetJSONType(bo.Value),
		}
	}

	startedAt := dbStep.Time.String()
	finishedAt := ""

	if dbStep.Status == string(MonitoringHistoryStatusFinished) && dbStep.UpdatedAt != nil {
		finishedAt = dbStep.UpdatedAt.String()
	}

	if err = sendResponse(w, http.StatusOK, MonitoringOutputsResponse{
		StartedAt:  &startedAt,
		FinishedAt: &finishedAt,
		Outputs:    &MonitoringOutputsResponse_Outputs{AdditionalProperties: outputs},
	}); err != nil {
		errorHandler.handleError(UnknownError, err)
	}
}

//nolint:revive,stylecheck //need to implement interface in api.go
func (ae *Env) MonitoringGetNotCreatedBlockInputs(w http.ResponseWriter, req *http.Request, workNumber, stepName string) {
	ctx, span := trace.StartSpan(req.Context(), "monitoring_get_not_created_block_inputs")
	defer span.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	dbInputs, err := ae.DB.GetEditedStepInputs(ctx, stepName, workNumber, time.Time{})
	if err != nil {
		e := GetBlockContextError

		log.WithField("workNumber", workNumber).
			WithField("stepName", stepName).
			Error(e.errorMessage(err))
		errorHandler.sendError(e)

		return
	}

	if err = sendResponse(w, http.StatusOK, MonitoringInputsResponse{
		Edited: &MonitoringInputsResponse_Edited{AdditionalProperties: toMonitoringInputs(dbInputs)},
	}); err != nil {
		errorHandler.handleError(UnknownError, err)
	}
}

func toMonitoringInputs(in entity.BlockInputs) map[string]MonitoringBlockParam {
	res := make(map[string]MonitoringBlockParam)
	for _, bo := range in {
		res[bo.Name] = MonitoringBlockParam{
			Name:  bo.Name,
			Value: bo.Value,
			Type:  utils.GetJSONType(bo.Value),
		}
	}

	return res
}
