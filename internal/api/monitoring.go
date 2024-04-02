package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/pkg/errors"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	human_tasks "gitlab.services.mts.ru/jocasta/pipeliner/internal/humantasks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/user"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

const (
	monitoringTimeLayout = "2006-01-02T15:04:05-0700"
)

func (ae *Env) GetTasksForMonitoring(w http.ResponseWriter, r *http.Request, params GetTasksForMonitoringParams) {
	ctx, span := trace.StartSpan(r.Context(), "get_tasks_for_monitoring")
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

		if t.LastEventAt != nil && t.LastEventType != nil && *t.LastEventType == string(MonitoringTaskActionRequestActionPause) {
			monitoringTableTask.PausedAt = t.LastEventAt.Format(monitoringTimeLayout)
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
		prefix := bo.StepName + "."

		if strings.HasPrefix(bo.Name, prefix) {
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

func (ae *Env) GetMonitoringTask(w http.ResponseWriter, req *http.Request, workNumber string, params GetMonitoringTaskParams) {
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

	workID, err := ae.DB.GetWorkIDByWorkNumber(ctx, workNumber)
	if err != nil {
		errorHandler.handleError(GetTaskError, err)

		return
	}

	events, err := ae.DB.GetTaskEvents(ctx, workID.String())
	if err != nil {
		errorHandler.handleError(GetTaskEventsError, err)

		return
	}

	nodes, err := ae.DB.GetTaskForMonitoring(ctx, workNumber, params.FromEventId, params.ToEventId)
	if err != nil {
		errorHandler.handleError(GetMonitoringNodesError, err)

		return
	}

	if len(nodes) == 0 {
		errorHandler.handleError(NoProcessNodesForMonitoringError, errors.New("No process nodes for monitoring"))

		return
	}

	resp := toMonitoringTaskResponse(nodes, events)
	if err = sendResponse(w, http.StatusOK, resp); err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

func getMonitoringStatus(status string) MonitoringHistoryStatus {
	switch status {
	case "cancel", "finished", "no_success", "revoke", "error", "skipped":
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

	if err = sendResponse(w, http.StatusOK, BlockStateResponse{
		State: &BlockStateResponse_State{params},
	}); err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

type startNodesParams struct {
	workID             uuid.UUID
	author, workNumber string
	byOne              bool
	params             *MonitoringTaskActionParams
}

//nolint:gocyclo,gocognit //its ok here
func (ae *Env) MonitoringTaskAction(w http.ResponseWriter, r *http.Request, workNumber string) {
	ctx, span := trace.StartSpan(r.Context(), "monitoring_task_action")
	defer span.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	if workNumber == "" {
		err := errors.New("workNumber is empty")
		errorHandler.handleError(ValidationError, err)

		return
	}

	b, err := io.ReadAll(r.Body)

	defer r.Body.Close()

	if err != nil {
		errorHandler.handleError(RequestReadError, err)

		return
	}

	req := &MonitoringTaskActionRequest{}

	err = json.Unmarshal(b, req)
	if err != nil {
		errorHandler.handleError(MonitoringTaskActionParseError, err)

		return
	}

	ui, err := user.GetUserInfoFromCtx(ctx)
	if err != nil {
		errorHandler.sendError(NoUserInContextError)

		return
	}

	defer func() {
		if rc := recover(); rc != nil {
			log.WithField("funcName", "recover").
				Error(r)
		}
	}()

	workID, err := ae.DB.GetWorkIDByWorkNumber(ctx, workNumber)
	if err != nil {
		errorHandler.handleError(GetTaskError, err)

		return
	}

	switch req.Action {
	case MonitoringTaskActionRequestActionPause:
		err = ae.pauseTask(ctx, ui.Username, workID.String(), req.Params)
		if err != nil {
			errorHandler.handleError(PauseTaskError, err)

			return
		}

	case MonitoringTaskActionRequestActionStart:
		err = ae.startTask(ctx, &startNodesParams{
			workID:     workID,
			author:     ui.Username,
			workNumber: workNumber,
			byOne:      false,
			params:     req.Params,
		})
		if err != nil {
			errorHandler.handleError(UnpauseTaskError, err)

			return
		}

	case MonitoringTaskActionRequestActionStartByOne:
		err = ae.startTask(ctx, &startNodesParams{
			workID:     workID,
			author:     ui.Username,
			workNumber: workNumber,
			byOne:      true,
			params:     req.Params,
		})
		if err != nil {
			errorHandler.handleError(UnpauseTaskError, err)

			return
		}
	case MonitoringTaskActionRequestActionEdit:
		if err != nil {
			errorHandler.handleError(WrongMonitoringActionError, err)

			return
		}
	}

	events, err := ae.DB.GetTaskEvents(ctx, workID.String())
	if err != nil {
		errorHandler.handleError(GetTaskEventsError, err)

		return
	}

	nodes, err := ae.DB.GetTaskForMonitoring(ctx, workNumber, nil, nil)
	if err != nil {
		errorHandler.handleError(GetMonitoringNodesError, err)

		return
	}

	if len(nodes) == 0 {
		errorHandler.handleError(NoProcessNodesForMonitoringError, errors.New("No process nodes for monitoring"))

		return
	}

	err = sendResponse(w, http.StatusOK, toMonitoringTaskResponse(nodes, events))
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

//nolint:all // ok
func toMonitoringTaskResponse(nodes []entity.MonitoringTaskNode, events []entity.TaskEvent) *MonitoringTask {
	const (
		finished = 2
		canceled = 6
	)

	res := &MonitoringTask{
		History:  make([]MonitoringHistory, 0),
		TaskRuns: make([]MonitoringTaskRun, 0),
	}
	res.ScenarioInfo = MonitoringScenarioInfo{
		Author:       nodes[0].Author,
		CreationTime: nodes[0].CreationTime,
		ScenarioName: nodes[0].ScenarioName,
	}
	res.VersionId = nodes[0].VersionID
	res.WorkNumber = nodes[0].WorkNumber
	res.IsPaused = nodes[0].IsPaused
	res.TaskRuns = getRunsByEvents(events)
	res.IsFinished = nodes[0].WorkStatus == finished || nodes[0].WorkStatus == canceled

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

	return res
}

func getRunsByEvents(events []entity.TaskEvent) []MonitoringTaskRun {
	res := make([]MonitoringTaskRun, 0)
	run := MonitoringTaskRun{}

	for i := range events {
		if events[i].EventType == string(MonitoringTaskEventEventTypeStart) {
			run.StartEventId = events[i].ID
			run.StartEventAt = events[i].CreatedAt
		}

		if events[i].EventType == string(MonitoringTaskEventEventTypePause) {
			run.EndEventId = events[i].ID
			run.EndEventAt = events[i].CreatedAt
		}

		isLastEvent := len(events) == i+1

		if isLastEvent && run.EndEventId == "" {
			run.EndEventAt = time.Now()
		}

		if (run.StartEventId != "" && run.EndEventId != "") || isLastEvent {
			run.Index = len(res) + 1
			res = append(res, run)
			run = MonitoringTaskRun{}
		}
	}

	return res
}

func (ae *Env) pauseTask(ctx context.Context, author, workID string, params *MonitoringTaskActionParams) error {
	txStorage, err := ae.DB.StartTransaction(ctx)
	if err != nil {
		return fmt.Errorf("failed start transaction, %w", err)
	}

	defer func() {
		rollbackErr := txStorage.RollbackTransaction(ctx)
		if rollbackErr != nil {
			ae.Log.WithError(err).
				WithField("funcName", "pauseTask").
				Error("failed rollback transaction")
		}
	}()

	err = txStorage.SetTaskPaused(ctx, workID, true)
	if err != nil {
		return err
	}

	stepNames := make([]string, 0)
	if params != nil && params.Steps != nil {
		stepNames = *params.Steps
	}

	err = txStorage.SetTaskBlocksPaused(ctx, workID, stepNames, true)
	if err != nil {
		return err
	}

	jsonParams := json.RawMessage{}
	if params != nil {
		jsonParams, err = json.Marshal(params)
		if err != nil {
			return err
		}
	}

	_, err = txStorage.CreateTaskEvent(ctx, &entity.CreateTaskEvent{
		WorkID:    workID,
		Author:    author,
		EventType: string(MonitoringTaskActionRequestActionPause),
		Params:    jsonParams,
	})
	if err != nil {
		return err
	}

	err = txStorage.CommitTransaction(ctx)
	if err != nil {
		return fmt.Errorf("failed commit transaction, %w", err)
	}

	return nil
}

func (ae *Env) startTask(ctx context.Context, dto *startNodesParams) error {
	isPaused, err := ae.DB.IsTaskPaused(ctx, dto.workID)
	if err != nil {
		return err
	}

	if !isPaused {
		return errors.New("can't unpause running task")
	}

	jsonParams := json.RawMessage{}
	if dto.params != nil {
		jsonParams, err = json.Marshal(dto.params)
		if err != nil {
			return err
		}
	}

	_, err = ae.DB.CreateTaskEvent(ctx, &entity.CreateTaskEvent{
		WorkID:    dto.workID.String(),
		Author:    dto.author,
		EventType: string(MonitoringTaskActionRequestActionStart),
		Params:    jsonParams,
	})
	if err != nil {
		return err
	}

	steps := *dto.params.Steps
	sort.Slice(steps, func(i, j int) bool {
		return strings.Contains(steps[i], "wait_for_all_inputs")
	})

	restartedNodes := make(map[string]interface{})
	for i := range *dto.params.Steps {
		if _, ok := restartedNodes[(*dto.params.Steps)[i]]; !ok {
			restartErr := ae.restartNode(
				ctx,
				dto.workID,
				dto.workNumber,
				(*dto.params.Steps)[i],
				dto.author,
				dto.byOne,
			)
			if restartErr != nil {
				return restartErr
			}
		}

		restartedNodes[(*dto.params.Steps)[i]] = nil
	}

	err = ae.DB.TryUnpauseTask(ctx, dto.workID)
	if err != nil {
		return err
	}

	return nil
}

func (ae *Env) restartNode(ctx context.Context, workID uuid.UUID, workNumber, stepName, login string, byOne bool) error {
	txStorage, err := ae.DB.StartTransaction(ctx)
	if err != nil {
		return fmt.Errorf("failed start transaction, %w", err)
	}

	defer func() {
		rollbackErr := txStorage.RollbackTransaction(ctx)
		if rollbackErr != nil {
			ae.Log.WithError(err).Error("failed rollback transaction")
		}
	}()

	dbStep, stepErr := txStorage.GetTaskStepByName(ctx, workID, stepName)
	if stepErr != nil {
		return stepErr
	}

	isFinished := dbStep.Status == "finished" || dbStep.Status == "skipped" || dbStep.Status == "cancel"

	if isFinished {
		var errCopy error
		dbStep.ID, errCopy = txStorage.CopyTaskBlock(ctx, dbStep.ID)
		if errCopy != nil {
			return errCopy
		}
	}

	isResumable, blockStartTime, resumableErr := txStorage.IsBlockResumable(ctx, workID, dbStep.ID)
	if resumableErr != nil {
		return resumableErr
	}

	if !isResumable {
		return fmt.Errorf("can't unpause running task block: %s", stepName)
	}

	blockData, blockErr := txStorage.GetBlockDataFromVersion(ctx, workNumber, stepName)
	if blockErr != nil {
		return blockErr
	}

	task, dbTaskErr := ae.GetTaskForUpdate(ctx, workNumber)
	if dbTaskErr != nil {
		return dbTaskErr
	}

	skipErr := ae.skipTaskBlocksAfterRestart(ctx, &task.Steps, blockStartTime, blockData.Next, workNumber, workID, txStorage)
	if skipErr != nil {
		return skipErr
	}

	unpErr := txStorage.UnpauseTaskBlock(ctx, workID, dbStep.ID)
	if unpErr != nil {
		return unpErr
	}

	storage, getErr := txStorage.GetVariableStorageForStep(ctx, workID, stepName)
	if getErr != nil {
		return getErr
	}

	err = txStorage.CommitTransaction(ctx)
	if err != nil {
		return fmt.Errorf("failed commit transaction, %w", err)
	}

	_, processErr := pipeline.ProcessBlockWithEndMapping(ctx, stepName, blockData, &pipeline.BlockRunContext{
		TaskID:      task.ID,
		WorkNumber:  workNumber,
		WorkTitle:   task.Name,
		Initiator:   task.Author,
		VarStore:    storage,
		Delegations: human_tasks.Delegations{},

		Services: pipeline.RunContextServices{
			HTTPClient:    ae.HTTPClient,
			Sender:        ae.Mail,
			Kafka:         ae.Kafka,
			People:        ae.People,
			ServiceDesc:   ae.ServiceDesc,
			FunctionStore: ae.FunctionStore,
			HumanTasks:    ae.HumanTasks,
			Integrations:  ae.Integrations,
			FileRegistry:  ae.FileRegistry,
			FaaS:          ae.FaaS,
			HrGate:        ae.HrGate,
			Scheduler:     ae.Scheduler,
			SLAService:    ae.SLAService,
			Storage:       ae.DB,
		},
		BlockRunResults: &pipeline.BlockRunResults{},

		UpdateData: &script.BlockUpdateData{
			Action:  string(entity.TaskUpdateActionReload),
			ByLogin: login,
		},

		Productive:     true,
		OnceProductive: byOne,

		IsTest:    task.IsTest,
		NotifName: task.Name,
	}, true)
	if processErr != nil {
		return processErr
	}

	return nil
}

func (ae *Env) getNodesToSkip(ctx context.Context, nextNodes map[string][]string,
	workNumber string, steps map[string]bool,
) (nodeList []string, err error) {
	for _, val := range nextNodes {
		for _, next := range val {
			if _, ok := steps[next]; !ok {
				continue
			}

			nodeList = append(nodeList, next)

			blockData, blockErr := ae.DB.GetBlockDataFromVersion(ctx, workNumber, next)
			if blockErr != nil {
				return nil, blockErr
			}

			nodes, recErr := ae.getNodesToSkip(ctx, blockData.Next, workNumber, steps)
			if recErr != nil {
				return nil, recErr
			}

			nodeList = append(nodeList, nodes...)
		}
	}

	return nodeList, nil
}

func (ae *Env) skipTaskBlocksAfterRestart(ctx context.Context, steps *entity.TaskSteps, blockStartTime time.Time,
	nextNodes map[string][]string, workNumber string, workID uuid.UUID, tx db.Database,
) (err error) {
	dbSteps := make(map[string]bool, 0)

	for i := range *steps {
		if (*steps)[i].Time.Before(blockStartTime) {
			continue
		}

		dbSteps[(*steps)[i].Name] = true
	}

	nodesToSkip, skipErr := ae.getNodesToSkip(ctx, nextNodes, workNumber, dbSteps)
	if skipErr != nil {
		return skipErr
	}

	dbSkipErr := tx.SkipBlocksAfterRestarted(ctx, workID, blockStartTime, nodesToSkip)
	if dbSkipErr != nil {
		return dbSkipErr
	}

	return nil
}

func (ae *Env) GetMonitoringTaskEvents(w http.ResponseWriter, req *http.Request, workNumber string) {
	ctx, s := trace.StartSpan(req.Context(), "get_monitoring_task_events")
	defer s.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	if workNumber == "" {
		err := errors.New("workNumber is empty")
		errorHandler.handleError(ValidationError, err)

		return
	}

	workID, err := ae.DB.GetWorkIDByWorkNumber(ctx, workNumber)
	if err != nil {
		errorHandler.handleError(GetTaskError, err)

		return
	}

	events, err := ae.DB.GetTaskEvents(ctx, workID.String())
	if err != nil {
		errorHandler.handleError(GetTaskEventsError, err)

		return
	}

	err = sendResponse(w, http.StatusOK, ae.toMonitoringTaskEventsResponse(ctx, events))
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

func (ae *Env) toMonitoringTaskEventsResponse(ctx context.Context, events []entity.TaskEvent) *MonitoringTaskEvents {
	res := &MonitoringTaskEvents{
		Events: make([]MonitoringTaskEvent, 0, len(events)),
	}

	fullNameCache := make(map[string]string)

	for i := range events {
		if _, ok := fullNameCache[events[i].Author]; !ok {
			userFullName, getUserErr := ae.getUserFullName(ctx, events[i].Author)
			if getUserErr != nil {
				fullNameCache[events[i].Author] = events[i].Author
			}

			fullNameCache[events[i].Author] = userFullName
		}

		params := MonitoringTaskActionParams{}

		err := json.Unmarshal(events[i].Params, &params)
		if err != nil {
			return res
		}

		event := MonitoringTaskEvent{
			Id:        events[i].ID,
			Author:    fullNameCache[events[i].Author],
			EventType: MonitoringTaskEventEventType(events[i].EventType),
			Params:    params,
			RunIndex:  1,
			CreatedAt: events[i].CreatedAt,
		}

		runs := getRunsByEvents(events)

		for runIndex := range runs {
			if (event.CreatedAt.After(runs[runIndex].StartEventAt) ||
				event.CreatedAt.Equal(runs[runIndex].StartEventAt)) &&
				(event.CreatedAt.Before(runs[runIndex].EndEventAt) ||
					event.CreatedAt.Equal(runs[runIndex].EndEventAt)) {
				event.RunIndex = runs[runIndex].Index

				break
			}
		}

		res.Events = append(res.Events, event)
	}

	return res
}
