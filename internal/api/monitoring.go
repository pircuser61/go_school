package api

import (
	"encoding/json"
	"net/http"

	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
	"go.opencensus.io/trace"
)

func (ae *APIEnv) GetTasksForMonitoring(w http.ResponseWriter, r *http.Request, params GetTasksForMonitoringParams) {
	//TODO implement me
	panic("implement me")
}

func (ae *APIEnv) GetMonitoringTask(w http.ResponseWriter, r *http.Request, workNumber string) {
	//TODO implement me
	panic("implement me")
}

func (ae *APIEnv) GetMonitoringTasksBlockBlockIdContext(w http.ResponseWriter, r *http.Request, blockId string) {
	ctx, span := trace.StartSpan(r.Context(), "start debug info")
	defer span.End()

	log := logger.GetLogger(ctx)

	blocksOutputs, err := ae.DB.GetBlocksOutputs(ctx, blockId)
	if err != nil {
		e := GetBlockOutputsError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}

	blocks := make(map[string]MonitoringBlockOutput, len(blocksOutputs))
	for _, bo := range blocksOutputs {
		valueData, err := json.Marshal(bo.Value)
		if err != nil {
			e := UnknownError
			log.Error(e.errorMessage(err))
			_ = e.sendError(w)
			return
		}

		var valueType string
		valueType = utils.GetJsonType(bo)

		blocks[bo.Name] = MonitoringBlockOutput{
			Name:        bo.Name,
			Value:       valueData,
			Description: "",
			Type:        valueType,
		}
	}

	if err = sendResponse(w, http.StatusOK, MonitoringOutputsResponse{
		Blocks: &MonitoringOutputsResponse_Blocks{blocks},
	}); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}
}
