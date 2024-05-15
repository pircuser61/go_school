package api

import (
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"net/http"

	"github.com/google/uuid"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

func (ae *Env) MonitoringGetBlockState(w http.ResponseWriter, r *http.Request, blockID string) {
	ctx, span := trace.StartSpan(r.Context(), "monitoring_get_block_state")
	defer span.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	id, err := uuid.Parse(blockID)
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}

	blockIsHidden, err := ae.DB.CheckBlockForHiddenFlag(ctx, blockID)
	if err != nil {
		e := CheckForHiddenError
		log.
			WithField("stepID", blockID).
			Error(e.errorMessage(err))
		errorHandler.sendError(e)

		return
	}

	if blockIsHidden {
		errorHandler.handleError(ForbiddenError, nil)

		return
	}

	dbStep, getStepErr := ae.DB.GetTaskStepByID(ctx, id)
	if getStepErr != nil {
		errorHandler.handleError(GetTaskStepError, getStepErr)

		return
	}

	state, err := ae.DB.GetBlockStateForMonitoring(ctx, id.String())
	if err != nil {
		errorHandler.handleError(GetBlockStateError, err)

		return
	}

	params := make(map[string]MonitoringBlockState, len(state))
	for _, bo := range state {
		params[bo.Name] = MonitoringBlockState{
			Name:  bo.Name,
			Value: bo.Value,
			Type:  utils.GetJSONType(bo.Value),
		}
	}

	var res BlockStateResponse

	if isStepFinished(dbStep.Status) {
		prevContent, errA := ae.DB.GetStepPreviousContent(ctx, blockID, dbStep.Time)
		if errA != nil {
			errorHandler.handleError(GetBlockStateError, errA)

			return
		}

		prevState := entity.BlockState{}

		if len(prevContent) > 0 {
			for i := range prevContent {
				prevState = append(prevState, entity.BlockStateValue{
					Name:  i,
					Value: prevContent[i],
				})
			}

			prevParams := make(map[string]MonitoringBlockState, len(state))
			for _, bo := range prevState {
				prevParams[bo.Name] = MonitoringBlockState{
					Name:  bo.Name,
					Value: bo.Value,
					Type:  utils.GetJSONType(bo.Value),
				}
			}

			res.WhileRunning = &BlockStateResponse_WhileRunning{prevParams}
			res.Edited = &BlockStateResponse_Edited{params}
		}

		if len(prevContent) == 0 {
			res.WhileRunning = &BlockStateResponse_WhileRunning{params}
			res.Edited = &BlockStateResponse_Edited{params}
		}
	}

	if !isStepFinished(dbStep.Status) {
		res.WhileRunning = &BlockStateResponse_WhileRunning{params}
		res.Edited = &BlockStateResponse_Edited{params}
	}

	if err = sendResponse(w, http.StatusOK, res); err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}
