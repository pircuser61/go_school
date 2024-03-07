package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/pkg/errors"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

const (
	monitoringTimeLayout  = "2006-01-02T15:04:05-0700"
	monitoringActionPause = "pause"
	monitoringActionStart = "start"
)

func (ae *Env) GetTasksForMonitoring(w http.ResponseWriter, r *http.Request, params GetTasksForMonitoringParams) {
	ctx, span := trace.StartSpan(r.Context(), "start get tasks for monitoring")
	defer span.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	statusFilter := make([]string, 0)

	if params.Status != nil {
		for i := range *params.Status {
			statusFilter = append(statusFilter, string((*params.Status)[i]))
		}
	}

	dbTasks, err := ae.DB.GetTasksForMonitoring(ctx, &entity.TasksForMonitoringFilters{
		PerPage:      params.PerPage,
		Page:         params.Page,
		SortColumn:   (*string)(params.SortColumn),
		SortOrder:    (*string)(params.SortOrder),
		Filter:       params.Filter,
		FromDate:     params.FromDate,
		ToDate:       params.ToDate,
		StatusFilter: statusFilter,
	})
	if err != nil {
		errorHandler.handleError(GetTasksForMonitoringError, err)

		return
	}

	initiatorsFullNameCache := make(map[string]string)

	responseTasks := make([]MonitoringTableTask, 0, len(dbTasks.Tasks))

	for i := range dbTasks.Tasks {
		t := dbTasks.Tasks[i]

		if _, ok := initiatorsFullNameCache[t.Initiator]; !ok {
			userLog := log.WithField("username", t.Initiator)

			userFullName, getUserErr := ae.getUserFullName(ctx, t.Initiator)
			if getUserErr != nil {
				e := GetTasksForMonitoringGetUserError
				userLog.Error(e.errorMessage(getUserErr))
			}

			initiatorsFullNameCache[t.Initiator] = userFullName
		}

		var processName string

		if t.ProcessDeletedAt != nil {
			const regexpString = "^(.+)(_deleted_at_\\d{4}-\\d{2}-\\d{2} \\d{2}:\\d{2}:\\d{2}.+)$"

			regexCompiled := regexp.MustCompile(regexpString)
			processName = regexCompiled.ReplaceAllString(t.ProcessName, "$1")
		} else {
			processName = t.ProcessName
		}

		monitoringTableTask := MonitoringTableTask{
			Initiator:         t.Initiator,
			InitiatorFullname: initiatorsFullNameCache[t.Initiator],
			ProcessName:       processName,
			StartedAt:         t.StartedAt.Format(monitoringTimeLayout),
			Status:            MonitoringTableTaskStatus(t.Status),
			WorkNumber:        t.WorkNumber,
		}

		if t.FinishedAt != nil {
			monitoringTableTask.FinishedAt = t.FinishedAt.Format(monitoringTimeLayout)
		}

		responseTasks = append(responseTasks, monitoringTableTask)
	}

	err = sendResponse(w, http.StatusOK, MonitoringTasksPage{
		Tasks: responseTasks,
		Total: dbTasks.Total,
	})
	if err != nil {
		errorHandler.handleError(UnknownError, err)
	}
}

func (ae *Env) getUserFullName(ctx context.Context, username string) (string, error) {
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

func (ae *Env) GetBlockContext(w http.ResponseWriter, r *http.Request, blockID string) {
	ctx, span := trace.StartSpan(r.Context(), "start get block context")
	defer span.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

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

	blocksOutputs, err := ae.DB.GetBlocksOutputs(ctx, blockID)
	if err != nil {
		errorHandler.handleError(GetBlockContextError, err)

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
			Type:        utils.GetJSONType(bo.Value),
		}
	}

	err = sendResponse(w, http.StatusOK, BlockContextResponse{
		Blocks: &BlockContextResponse_Blocks{blocks},
	})
	if err != nil {
		errorHandler.handleError(UnknownError, err)
	}
}

func (ae *Env) GetMonitoringTask(w http.ResponseWriter, req *http.Request, workNumber string) {
	ctx, s := trace.StartSpan(req.Context(), "get_monitoring_task")
	defer s.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	if workNumber == "" {
		err := errors.New("workNumber is empty")
		errorHandler.handleError(UUIDParsingError, err)

		return
	}

	taskIsHidden, err := ae.DB.CheckTaskForHiddenFlag(ctx, workNumber)
	if err != nil {
		errorHandler.handleError(CheckForHiddenError, err)

		return
	}

	if taskIsHidden {
		errorHandler.handleError(ForbiddenError, nil)

		return
	}

	nodes, err := ae.DB.GetTaskForMonitoring(ctx, workNumber)
	if err != nil {
		errorHandler.handleError(GetMonitoringNodesError, err)

		return
	}

	if len(nodes) == 0 {
		errorHandler.handleError(NoProcessNodesForMonitoringError, errors.New("No process nodes for monitoring"))

		return
	}

	res := MonitoringTask{History: make([]MonitoringHistory, 0)}
	res.ScenarioInfo = MonitoringScenarioInfo{
		Author:       nodes[0].Author,
		CreationTime: nodes[0].CreationTime,
		ScenarioName: nodes[0].ScenarioName,
	}
	res.VersionId = nodes[0].VersionID
	res.WorkNumber = nodes[0].WorkNumber
	res.IsPaused = nodes[0].ProcessIsPaused

	for i := range nodes {
		monitoringHistory := MonitoringHistory{
			BlockId:  nodes[i].BlockID,
			RealName: nodes[i].RealName,
			Status:   getMonitoringStatus(nodes[i].Status),
			NodeId:   nodes[i].NodeID,
			IsPaused: nodes[i].BlockIsPaused,
		}

		if nodes[i].BlockDateInit != nil {
			formattedTime := nodes[i].BlockDateInit.Format(monitoringTimeLayout)
			monitoringHistory.BlockDateInit = &formattedTime
		}

		res.History = append(res.History, monitoringHistory)
	}

	if err = sendResponse(w, http.StatusOK, res); err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

func getMonitoringStatus(status string) MonitoringHistoryStatus {
	switch status {
	case "cancel", "finished", "no_success", "revoke", "error":
		return MonitoringHistoryStatusFinished
	default:
		return MonitoringHistoryStatusRunning
	}
}

//nolint:revive,stylecheck //need to implement interface in api.go
func (ae *Env) GetMonitoringTasksBlockBlockIdParams(w http.ResponseWriter, req *http.Request, blockID string) {
	ctx, span := trace.StartSpan(req.Context(), "get_monitoring_tasks_block_blockId_params")
	defer span.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	blockIDUUID, err := uuid.Parse(blockID)
	if err != nil {
		errorHandler.handleError(UUIDParsingError, err)
	}

	taskStep, err := ae.DB.GetTaskStepByID(ctx, blockIDUUID)
	if err != nil {
		e := UnknownError

		log.WithField("blockId", blockID).
			Error(e.errorMessage(err))
		errorHandler.sendError(e)
	}

	blockInputs, err := ae.DB.GetBlockInputs(ctx, taskStep.Name, taskStep.WorkNumber)
	if err != nil {
		e := GetBlockContextError

		log.WithField("blockId", blockID).
			WithField("taskStep.Name", taskStep.Name).
			Error(e.errorMessage(err))
		errorHandler.sendError(e)

		return
	}

	inputs := make(map[string]MonitoringBlockParam, 0)

	for _, bo := range blockInputs {
		inputs[bo.Name] = MonitoringBlockParam{
			Name:  bo.Name,
			Value: bo.Value,
			Type:  utils.GetJSONType(bo.Value),
		}
	}

	blockOutputs, err := ae.DB.GetBlockOutputs(ctx, blockID, taskStep.Name)
	if err != nil {
		e := GetBlockContextError

		log.WithField("blockId", blockID).
			WithField("taskStep.Name", taskStep.Name).
			Error(e.errorMessage(err))
		errorHandler.sendError(e)

		return
	}

	blockIsHidden, err := ae.DB.CheckBlockForHiddenFlag(ctx, blockID)
	if err != nil {
		e := CheckForHiddenError

		log.WithField("blockId", blockID).
			WithField("taskStep.Name", taskStep.Name).
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
		errorHandler.handleError(UnknownError, err)
	}
}

func (ae *Env) GetBlockState(w http.ResponseWriter, r *http.Request, blockID string) {
	ctx, span := trace.StartSpan(r.Context(), "start get block state")
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
			WithField("blockId", blockID).
			Error(e.errorMessage(err))
		errorHandler.sendError(e)

		return
	}

	if blockIsHidden {
		errorHandler.handleError(ForbiddenError, nil)

		return
	}

	state, err := ae.DB.GetBlockState(ctx, id.String())
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

	if err = sendResponse(w, http.StatusOK, BlockStateResponse{
		State: &BlockStateResponse_State{params},
	}); err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

func (ae *Env) MonitoringTaskAction(w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.StartSpan(r.Context(), "monitoring_task_action")
	defer span.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	b, err := io.ReadAll(r.Body)

	defer r.Body.Close()

	if err != nil {
		errorHandler.handleError(RequestReadError, err)

		return
	}

	req := &MonitoringTaskActionRequest{}

	err = json.Unmarshal(b, req)
	if err != nil {
		errorHandler.handleError(PipelineParseError, err)

		return
	}

	txStorage, transactionErr := ae.DB.StartTransaction(ctx)
	if transactionErr != nil {
		log.WithError(transactionErr).Error("couldn't start transaction")

		errorHandler.sendError(UnknownError)

		return
	}

	defer func() {
		if r := recover(); r != nil {
			log.WithField("funcName", "recover").
				Error(r)

			if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
				log.WithField("funcName", "RollbackTransaction").
					WithError(txErr).
					Error("rollback transaction")
			}
		}
	}()

	workID, err := ae.DB.GetWorkIDByWorkNumber(ctx, req.WorkNumber)
	if err != nil {
		errorHandler.handleError(GetTaskError, err)

		if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
			log.WithField("funcName", "MonitoringTaskAction").
				WithError(errors.New("couldn't rollback tx")).
				Error(txErr)
		}

		return
	}

	switch req.Action {
	case monitoringActionPause:
		err = ae.pauseProcess(ctx, workID.String(), req.Params)
		if err != nil {
			errorHandler.handleError(GetTaskError, err)

			if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
				log.WithField("funcName", "MonitoringTaskAction").
					WithError(errors.New("couldn't rollback tx")).
					Error(txErr)
			}

			return
		}

	case monitoringActionStart:
		err = ae.startPreocess()
		if err != nil {
			errorHandler.handleError(GetTaskError, err)

			if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
				log.WithField("funcName", "MonitoringTaskAction").
					WithError(errors.New("couldn't rollback tx")).
					Error(txErr)
			}

			return
		}
	}

	if err = txStorage.CommitTransaction(ctx); err != nil {
		log.WithError(err).Error("couldn't commit transaction")

		errorHandler.sendError(UnknownError)

		return
	}

	err = sendResponse(w, http.StatusOK, nil)
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

func (ae *Env) pauseProcess(ctx context.Context, workID string, params *MonitoringTaskActionParams) error {
	err := ae.DB.SetTaskPaused(ctx, workID, true)
	if err != nil {
		return err
	}

	stepNames := make([]string, 0)
	if params != nil && params.Steps != nil {
		stepNames = *params.Steps
	}

	err = ae.DB.SetTaskBlocksPaused(ctx, workID, stepNames, true)
	if err != nil {
		return err
	}

	return nil
}

func (ae *Env) startPreocess() error {
	return nil
}
