package api

import (
	"net/http"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

func (ae *APIEnv) GetMonitoringTasksBlockBlockIdContext(w http.ResponseWriter, r *http.Request, blockId string) {
	ctx, span := trace.StartSpan(r.Context(), "start get block context")
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
		blocks[bo.Name] = MonitoringBlockOutput{
			Name:        bo.Name,
			Value:       bo.Value,
			Description: "",
			Type:        utils.GetJsonType(bo.Value),
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

func (ae *APIEnv) GetTasksForMonitoring(w http.ResponseWriter, req *http.Request, params GetTasksForMonitoringParams) {
	panic("implement me")
}

func (ae *APIEnv) GetMonitoringTask(w http.ResponseWriter, req *http.Request, workNumber string) {
	panic("implement me")
}

func (ae *APIEnv) GetMonitoringTasksBlockBlockIdParams(w http.ResponseWriter, req *http.Request, blockId string) {
	ctx, span := trace.StartSpan(req.Context(), "get_monitoring_tasks_block_blockId_params")
	defer span.End()

	log := logger.GetLogger(ctx)

	if err := sendResponse(w, http.StatusOK, nil); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}
}
