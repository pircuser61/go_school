package api

import (
	"net/http"
	"strings"

	"github.com/google/uuid"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"

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

	isFinished := isStepFinished(dbStep.Status)

	if isFinished {
		prevContent, errA := ae.DB.GetStepPreviousContent(ctx, blockID, dbStep.Time)
		if errA != nil {
			errorHandler.handleError(GetBlockStateError, errA)

			return
		}

		prevState := getPrevStepState(prevContent, dbStep.Name)

		if len(prevState) > 0 {
			prevStateRes := make(map[string]MonitoringBlockState, len(state))
			for _, bo := range prevState {
				prevStateRes[bo.Name] = MonitoringBlockState{
					Name:  bo.Name,
					Value: bo.Value,
					Type:  utils.GetJSONType(bo.Value),
				}
			}

			res.WhileRunning = &BlockStateResponse_WhileRunning{prevStateRes}
			res.Edited = &BlockStateResponse_Edited{params}
		}

		if len(prevState) == 0 {
			res.WhileRunning = &BlockStateResponse_WhileRunning{params}
			res.Edited = &BlockStateResponse_Edited{params}
		}
	}

	if !isFinished {
		res.WhileRunning = &BlockStateResponse_WhileRunning{params}
		res.Edited = &BlockStateResponse_Edited{params}
	}

	if err = sendResponse(w, http.StatusOK, res); err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

func getPrevStepState(prevContent map[string]interface{}, stepName string) entity.BlockState {
	prevState := entity.BlockState{}

	for contentKey := range prevContent {
		if !strings.EqualFold(contentKey, "state") {
			continue
		}

		if commonState, ok := prevContent[contentKey].(map[string]interface{}); ok {
			for stateKey := range commonState {
				if stateKey != stepName {
					continue
				}

				if stepState, okStepState := commonState[stateKey].(map[string]interface{}); okStepState {
					for stepStateKey := range stepState {
						prevState = append(prevState, entity.BlockStateValue{
							Name:  stepStateKey,
							Value: stepState[stepStateKey],
						})
					}
				}
			}
		}
	}

	return prevState
}
