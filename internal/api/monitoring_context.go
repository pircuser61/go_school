package api

import (
	"net/http"
	"strings"

	"github.com/google/uuid"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

func (ae *Env) MonitoringGetBlockContext(w http.ResponseWriter, r *http.Request, blockID string) {
	ctx, span := trace.StartSpan(r.Context(), "monitoring_get_block_context")
	defer span.End()

	log := script.SetMainFuncLog(ctx,
		"MonitoringGetBlockContext",
		script.MethodGet,
		script.HTTP,
		span.SpanContext().TraceID.String(),
		"v1")
	errorHandler := newHTTPErrorHandler(log, w)

	id, err := uuid.Parse(blockID)
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}

	blockIsHidden, err := ae.DB.CheckBlockForHiddenFlag(ctx, blockID)
	if err != nil {
		e := newHTTPErrorHandler(log.WithField("blockId", blockID), w)
		e.handleError(CheckForHiddenError, err)

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

	blocksOutputs, err := ae.DB.GetBlocksOutputs(ctx, blockID)
	if err != nil {
		errorHandler.handleError(GetBlockContextError, err)

		return
	}

	blocks := make(map[string]MonitoringBlockOutput, len(blocksOutputs))

	for _, bo := range blocksOutputs {
		prefix := bo.StepName + "."

		if strings.HasPrefix(bo.Name, prefix) {
			continue
		}

		blocks[bo.Name] = MonitoringBlockOutput{
			Name:  bo.Name,
			Value: bo.Value,
			Type:  utils.GetJSONType(bo.Value),
		}
	}

	var res BlockContextResponse

	if isStepFinished(dbStep.Status) {
		prevContent, errA := ae.DB.GetStepPreviousContent(ctx, blockID, dbStep.Time)
		if errA != nil {
			errorHandler.handleError(GetBlockStateError, errA)

			return
		}

		prevContext := entity.BlockOutputs{}

		if len(prevContent) > 0 {
			for i := range prevContent {
				prevContext = append(prevContext, entity.BlockOutputValue{
					Name:  i,
					Value: prevContent[i],
				})
			}

			prevBlocks := make(map[string]MonitoringBlockOutput, len(blocks))
			for _, bo := range prevContext {
				prevBlocks[bo.Name] = MonitoringBlockOutput{
					Name:  bo.Name,
					Value: bo.Value,
					Type:  utils.GetJSONType(bo.Value),
				}
			}

			res.WhileRunning = &BlockContextResponse_WhileRunning{prevBlocks}
			res.Edited = &BlockContextResponse_Edited{blocks}
		}

		if len(prevContent) == 0 {
			res.WhileRunning = &BlockContextResponse_WhileRunning{blocks}
			res.Edited = &BlockContextResponse_Edited{blocks}
		}
	}

	if !isStepFinished(dbStep.Status) {
		res.WhileRunning = &BlockContextResponse_WhileRunning{blocks}
		res.Edited = &BlockContextResponse_Edited{blocks}
	}

	err = sendResponse(w, http.StatusOK, res)
	if err != nil {
		errorHandler.handleError(UnknownError, err)
	}
}
