package api

import (
	"net/http"

	"go.opencensus.io/trace"

	"github.com/google/uuid"

	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

func (ae *APIEnv) GetBlockContext(w http.ResponseWriter, r *http.Request, blockId string) {
	ctx, span := trace.StartSpan(r.Context(), "start get block context")
	defer span.End()

	log := logger.GetLogger(ctx)

	blocksOutputs, err := ae.DB.GetBlocksOutputs(ctx, blockId)
	if err != nil {
		e := GetBlockContextError
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

	if err = sendResponse(w, http.StatusOK, BlockContextResponse{
		Blocks: &BlockContextResponse_Blocks{blocks},
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

	blockIdUUID, err := uuid.Parse(blockId)
	if err != nil {
		e := UUIDParsingError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
	}

	taskStep, err := ae.DB.GetTaskStepById(ctx, blockIdUUID)
	if err != nil {
		e := UUIDParsingError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
	}

	blockInputs, err := ae.DB.GetBlockInputs(ctx, blockId, taskStep.WorkNumber)
	if err != nil {
		e := GetBlockContextError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}

	inputs := make(map[string]MonitoringBlockParam, 0)
	for _, bo := range blockInputs {
		inputs[bo.Name] = MonitoringBlockParam{
			Name:  bo.Name,
			Value: bo.Value,
			Type:  utils.GetJsonType(bo.Value),
		}
	}

	blockOutputs, err := ae.DB.GetBlockOutputs(ctx, blockId, taskStep.Name)
	if err != nil {
		e := GetBlockContextError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}

	outputs := make(map[string]MonitoringBlockParam, 0)
	for _, bo := range blockOutputs {
		outputs[bo.Name] = MonitoringBlockParam{
			Name:  bo.Name,
			Value: bo.Value,
			Type:  utils.GetJsonType(bo.Value),
		}
	}

	startedAt := taskStep.Time.String()
	finishedAt := ""
	if taskStep.Status == string(MonitoringHistoryStatusFinished) && taskStep.UpdatedAt != nil {
		finishedAt = taskStep.UpdatedAt.String()
	}

	if err := sendResponse(w, http.StatusOK, MonitoringParamsResponse{
		StartedAt:  &startedAt,
		FinishedAt: &finishedAt,
		Inputs:     &MonitoringParamsResponse_Inputs{AdditionalProperties: inputs},
		Outputs:    &MonitoringParamsResponse_Outputs{AdditionalProperties: outputs},
	}); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}
}
