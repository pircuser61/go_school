package api

import (
	"net/http"
	"strings"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

func (ae *APIEnv) GetTasksForMonitoring(w http.ResponseWriter, r *http.Request, params GetTasksForMonitoringParams) {
	ctx, span := trace.StartSpan(r.Context(), "start get tasks for monitoring")
	defer span.End()

	log := logger.GetLogger(ctx)

	dbTasks, err := ae.DB.GetTasksForMonitoring(ctx, entity.TasksForMonitoringFilters{
		PerPage:    params.PerPage,
		Page:       params.Page,
		SortColumn: (*string)(params.SortColumn),
		SortOrder:  (*string)(params.SortOrder),
		Filter:     (*string)(params.Filter),
		FromDate:   params.FromDate,
		ToDate:     params.ToDate,
	})
	if err != nil {
		e := GetTasksForMonitoringError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}

	responseTasks := make([]MonitoringTableTask, 0, len(dbTasks))
	for _, t := range dbTasks {
		responseTasks = append(responseTasks, MonitoringTableTask{
			Id:          t.Id.String(),
			Initiator:   t.Initiator,
			ProcessName: t.ProcessName,
			StartedAt:   t.StartedAt.String(),
			Status:      MonitoringTableTaskStatus(t.Status),
			WorkNumber:  t.WorkNumber,
		})
	}

	if err = sendResponse(w, http.StatusOK, MonitoringTasksPage{
		Tasks: responseTasks,
		Total: len(responseTasks),
	}); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}
}

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
		if strings.Contains(bo.Name, bo.StepName) {
			continue
		}

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

func (ae *APIEnv) GetMonitoringTask(w http.ResponseWriter, req *http.Request, workNumber string) {
	panic("implement me")
}
