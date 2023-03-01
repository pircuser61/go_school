package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
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
		Filter:     params.Filter,
		FromDate:   params.FromDate,
		ToDate:     params.ToDate,
	})
	if err != nil {
		e := GetTasksForMonitoringError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}

	initiatorsFullNameCache := make(map[string]string)

	responseTasks := make([]MonitoringTableTask, 0, len(dbTasks.Tasks))
	for _, t := range dbTasks.Tasks {
		if _, ok := initiatorsFullNameCache[t.Initiator]; !ok {
			userLog := log.WithField("username", t.Initiator)

			userFullName, getUserErr := ae.getUserFullName(ctx, t.Initiator)
			if getUserErr != nil {
				e := GetTasksForMonitoringGetUserError
				userLog.Error(e.errorMessage(getUserErr))
			}

			initiatorsFullNameCache[t.Initiator] = userFullName
		}

		responseTasks = append(responseTasks, MonitoringTableTask{
			Id:                t.Id.String(),
			Initiator:         t.Initiator,
			InitiatorFullname: initiatorsFullNameCache[t.Initiator],
			ProcessName:       t.ProcessName,
			StartedAt:         t.StartedAt.Format("2006-01-02T15:04:05-0700"),
			Status:            MonitoringTableTaskStatus(t.Status),
			WorkNumber:        t.WorkNumber,
		})
	}

	if err = sendResponse(w, http.StatusOK, MonitoringTasksPage{
		Tasks: responseTasks,
		Total: dbTasks.Total,
	}); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}
}

func (ae *APIEnv) getUserFullName(ctx context.Context, username string) (string, error) {
	initiatorUserInfo, getUserErr := ae.People.GetUser(ctx, username)
	if getUserErr != nil {
		return "", getUserErr
	}

	initiatorSSOUser, typedErr := initiatorUserInfo.ToSSOUserTyped()
	if typedErr != nil {
		return "", typedErr
	}

	return initiatorSSOUser.GetFullName(), nil
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
	ctx, s := trace.StartSpan(req.Context(), "get_task_for_monitoring")
	defer s.End()

	log := logger.GetLogger(ctx)

	if workNumber == "" {
		e := UUIDParsingError
		log.Error(e.errorMessage(errors.New("workNumber is empty")))
		_ = e.sendError(w)
		return
	}

	nodes, err := ae.DB.GetTaskForMonitoring(ctx, workNumber)
	if err != nil {
		e := GetMonitoringNodesError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}
	if len(nodes) == 0 {
		e := NoProcessNodesForMonitoringError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}

	res := MonitoringTask{History: make([]MonitoringHistory, 0)}
	res.ScenarioInfo = MonitoringScenarioInfo{
		Author:       nodes[0].Author,
		CreationTime: nodes[0].CreationTime,
		ScenarioName: nodes[0].ScenarioName,
	}
	res.VersionId = nodes[0].VersionId
	res.WorkNumber = nodes[0].WorkNumber

	for i := range nodes {
		res.History = append(res.History, MonitoringHistory{
			BlockId:  nodes[i].BlockId,
			RealName: nodes[i].RealName,
			Status:   getMonitoringStatus(nodes[i].Status),
			NodeId:   nodes[i].NodeId,
		})
	}
	if err = sendResponse(w, http.StatusOK, res); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

func getMonitoringStatus(status string) MonitoringHistoryStatus {
	switch status {
	case "cancel", "finished", "no_success", "revoke":
		return MonitoringHistoryStatusFinished
	default:
		return MonitoringHistoryStatusRunning
	}
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
		e := UnknownError
		log.WithField("blockId", blockId).
			Error(e.errorMessage(err))
		_ = e.sendError(w)
	}

	blockInputs, err := ae.DB.GetBlockInputs(ctx, taskStep.Name, taskStep.WorkNumber)
	if err != nil {
		e := GetBlockContextError
		log.WithField("blockId", blockId).
			WithField("taskStep.Name", taskStep.Name).
			Error(e.errorMessage(err))
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
		log.WithField("blockId", blockId).
			WithField("taskStep.Name", taskStep.Name).
			Error(e.errorMessage(err))
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
