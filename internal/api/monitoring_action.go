package api

import (
	c "context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
)

const (
	cancel    = "cancel"
	skipped   = "skipped"
	finished  = "finished"
	noSuccess = "no_success"

	monitoringTimeLayout = "2006-01-02T15:04:05-0700"
)

func getMonitoringStatus(status string) MonitoringHistoryStatus {
	switch status {
	case cancel, finished, "no_success", "revoke", "error", "skipped":
		return MonitoringHistoryStatusFinished
	default:
		return MonitoringHistoryStatusRunning
	}
}

type startStepsDTO struct {
	workID             uuid.UUID
	author, workNumber string
	byOne              bool
	params             *MonitoringTaskActionParams
}

//nolint:gocyclo,gocognit //its ok here
func (ae *Env) MonitoringTaskAction(w http.ResponseWriter, r *http.Request, workNumber string) {
	ctx, span := trace.StartSpan(r.Context(), "monitoring_task_action")
	defer span.End()

	log := logger.GetLogger(ctx).
		WithField("funcName", "MonitoringTaskAction").
		WithField("workNumber", workNumber)
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

	log = log.WithField("action", req.Action)
	ctx = logger.WithLogger(ctx, log)

	switch req.Action {
	case MonitoringTaskActionRequestActionPause:
		err = ae.pauseTask(ctx, ui.Username, workID, req.Params)
		if err != nil {
			errorHandler.handleError(PauseTaskError, err)

			return
		}

	case MonitoringTaskActionRequestActionStart:
		err = ae.startTask(ctx, &startStepsDTO{
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
		err = ae.startTask(ctx, &startStepsDTO{
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
	}

	events, err := ae.DB.GetTaskEvents(ctx, workID.String())
	if err != nil {
		errorHandler.handleError(GetTaskEventsError, err)

		return
	}

	steps, err := ae.DB.GetTaskForMonitoring(ctx, workNumber, nil, nil)
	if err != nil {
		errorHandler.handleError(GetMonitoringNodesError, err)

		return
	}

	if len(steps) == 0 {
		errorHandler.handleError(NoProcessNodesForMonitoringError, errors.New("No process steps for monitoring"))

		return
	}

	err = sendResponse(w, http.StatusOK, toMonitoringTaskResponse(steps, events))
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

func getRunsByEvents(events []entity.TaskEvent) []MonitoringTaskRun {
	res := make([]MonitoringTaskRun, 0)
	run := MonitoringTaskRun{}

	for i := range events {
		if events[i].EventType == string(MonitoringTaskEventEventTypeStart) {
			run.StartEventId = events[i].ID
			run.StartEventAt = events[i].CreatedAt
		}

		if events[i].EventType == string(MonitoringTaskEventEventTypePause) && run.StartEventId != "" {
			run.EndEventId = events[i].ID
			run.EndEventAt = events[i].CreatedAt
		}

		isLastEvent := len(events) == i+1

		if isLastEvent && run.StartEventId == "" {
			continue
		}

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

func (ae *Env) pauseTask(ctx c.Context, author string, workID uuid.UUID, params *MonitoringTaskActionParams) error {
	const fn = "pauseTask"

	txStorage, err := ae.DB.StartTransaction(ctx)
	if err != nil {
		return fmt.Errorf("failed start transaction, %w", err)
	}

	err = txStorage.SetTaskPaused(ctx, workID.String(), true)
	if err != nil {
		ae.rollbackTransaction(ctx, txStorage, fn)

		return err
	}

	err = txStorage.UpdateTaskStatus(ctx, workID, db.RunStatusStopped, "", "")
	if err != nil {
		ae.rollbackTransaction(ctx, txStorage, fn)

		return err
	}

	stepIDs := make([]string, 0)
	if params != nil && params.Steps != nil {
		stepIDs = *params.Steps
	}

	ids, err := txStorage.PauseTaskBlocks(ctx, workID.String(), stepIDs)
	if err != nil {
		ae.rollbackTransaction(ctx, txStorage, fn)

		return err
	}

	if ids != nil {
		params.Steps = &ids
	}

	jsonParams := json.RawMessage{}
	if params != nil {
		jsonParams, err = json.Marshal(params)
		if err != nil {
			ae.rollbackTransaction(ctx, txStorage, fn)

			return err
		}
	}

	_, err = txStorage.CreateTaskEvent(ctx, &entity.CreateTaskEvent{
		WorkID:    workID.String(),
		Author:    author,
		EventType: string(MonitoringTaskActionRequestActionPause),
		Params:    jsonParams,
	})
	if err != nil {
		ae.rollbackTransaction(ctx, txStorage, fn)

		return err
	}

	err = txStorage.CommitTransaction(ctx)
	if err != nil {
		return fmt.Errorf("failed commit transaction, %w", err)
	}

	return nil
}

func (ae *Env) startTask(ctx c.Context, dto *startStepsDTO) error {
	const fn = "startTask"

	txStorage, err := ae.DB.StartTransaction(ctx)
	if err != nil {
		return fmt.Errorf("failed start transaction, %w", err)
	}

	isPaused, err := txStorage.IsTaskPaused(ctx, dto.workID)
	if err != nil {
		ae.rollbackTransaction(ctx, txStorage, fn)

		return err
	}

	if !isPaused {
		ae.rollbackTransaction(ctx, txStorage, fn)

		return errors.New("can't unpause running task")
	}

	if dto.params == nil || dto.params.Steps == nil {
		ae.rollbackTransaction(ctx, txStorage, fn)

		return errors.New("can't found restarting steps")
	}

	// remove double steps
	filteredSteps := make(map[string]interface{})
	steps := make([]string, 0)

	for i := range *dto.params.Steps {
		if _, ok := filteredSteps[(*dto.params.Steps)[i]]; !ok {
			steps = append(steps, (*dto.params.Steps)[i])
		}

		filteredSteps[(*dto.params.Steps)[i]] = nil
	}

	sort.Slice(steps, func(i, _ int) bool {
		return strings.Contains(steps[i], "wait_for_all_inputs")
	})

	crEventTime := time.Now()

	newSteps := make([]string, 0, len(steps))

	if updErr := txStorage.UpdateTaskStatus(ctx, dto.workID, db.RunStatusRunning, "", ""); updErr != nil {
		ae.rollbackTransaction(ctx, txStorage, fn)

		return updErr
	}

	for i := range steps {
		newStepID, restartErr := ae.restartStep(
			ctx,
			txStorage,
			dto.workID,
			dto.workNumber,
			(*dto.params.Steps)[i],
			dto.author,
			dto.byOne,
		)
		if restartErr != nil {
			ae.rollbackTransaction(ctx, txStorage, fn)

			return restartErr
		}

		newSteps = append(newSteps, newStepID)
	}

	dto.params.Steps = &newSteps

	jsonParams := json.RawMessage{}
	if dto.params != nil {
		jsonParams, err = json.Marshal(dto.params)
		if err != nil {
			ae.rollbackTransaction(ctx, txStorage, fn)

			return err
		}
	}

	_, err = txStorage.CreateTaskEvent(ctx, &entity.CreateTaskEvent{
		WorkID:    dto.workID.String(),
		Author:    dto.author,
		EventType: string(MonitoringTaskActionRequestActionStart),
		Params:    jsonParams,
		Time:      crEventTime,
	})
	if err != nil {
		ae.rollbackTransaction(ctx, txStorage, fn)

		return err
	}

	err = txStorage.TryUnpauseTask(ctx, dto.workID)
	if err != nil {
		ae.rollbackTransaction(ctx, txStorage, fn)

		return err
	}

	err = txStorage.CommitTransaction(ctx)
	if err != nil {
		return fmt.Errorf("failed commit transaction, %w", err)
	}

	return nil
}

func (ae *Env) restartStep(ctx c.Context, tx db.Database, wID uuid.UUID, wNumber, stepID, login string, byOne bool) (string, error) {
	sid, parseErr := uuid.Parse(stepID)
	if parseErr != nil {
		return "", parseErr
	}

	dbStep, stepErr := tx.GetTaskStepByID(ctx, sid)
	if stepErr != nil {
		return "", stepErr
	}

	isResemble, _, resembleErr := tx.IsBlockResumable(ctx, wID, dbStep.ID)
	if resembleErr != nil {
		return "", resembleErr
	}

	if !isResemble && !isStepFinished(dbStep.Status) {
		return "", fmt.Errorf("can't unpause running task block: %s", sid)
	}

	blockData, blockErr := tx.GetStepDataFromVersion(ctx, wNumber, dbStep.Name)
	if blockErr != nil {
		return "", blockErr
	}

	task, dbTaskErr := ae.GetTaskForRestart(ctx, wNumber)
	if dbTaskErr != nil {
		return "", dbTaskErr
	}

	skipErr := ae.skipTaskBlocksAfterRestart(ctx, &task.Steps, dbStep.Time, blockData.Next, wNumber, wID, tx)
	if skipErr != nil {
		return "", skipErr
	}

	if isStepFinished(dbStep.Status) {
		var errCopy error
		dbStep.ID, errCopy = tx.CopyTaskBlock(ctx, dbStep.ID)

		if errCopy != nil {
			return "", errCopy
		}
	}

	unpErr := tx.UnpauseTaskBlock(ctx, wID, dbStep.ID)
	if unpErr != nil {
		return "", unpErr
	}

	storage, getErr := tx.GetVariableStorageForStepByID(ctx, dbStep.ID)
	if getErr != nil {
		return "", getErr
	}

	pipelineID, versionID, err := ae.DB.GetPipelineIDByWorkID(ctx, task.ID.String())
	if err != nil {
		return "", err
	}

	log := logger.GetLogger(ctx).WithField("pipelineID", pipelineID).
		WithField("versionID", versionID).
		WithField("pipelineID", pipelineID)
	ctx = logger.WithLogger(ctx, log)

	_, _, processErr := pipeline.ProcessBlockWithEndMapping(ctx, dbStep.Name, blockData, &pipeline.BlockRunContext{
		TaskID:      task.ID,
		WorkNumber:  wNumber,
		WorkTitle:   task.Name,
		PipelineID:  pipelineID,
		VersionID:   versionID,
		Initiator:   task.Author,
		VarStore:    storage,
		Delegations: human_tasks.Delegations{},

		Services: pipeline.RunContextServices{
			HTTPClient:     ae.HTTPClient,
			Sender:         ae.Mail,
			Kafka:          ae.Kafka,
			People:         ae.People,
			ServiceDesc:    ae.ServiceDesc,
			FunctionStore:  ae.FunctionStore,
			HumanTasks:     ae.HumanTasks,
			Integrations:   ae.Integrations,
			FileRegistry:   ae.FileRegistry,
			FaaS:           ae.FaaS,
			HrGate:         ae.HrGate,
			Scheduler:      ae.Scheduler,
			SLAService:     ae.SLAService,
			Storage:        tx,
			StorageFactory: ae.DB,
			JocastaURL:     ae.HostURL,
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
		return "", processErr
	}

	return dbStep.ID.String(), nil
}

func (ae *Env) getStepsToSkip(ctx c.Context, nextSteps map[string][]string,
	workNumber string, steps map[string]bool, viewedSteps map[string]struct{},
) (stepList []string, err error) {
	for _, val := range nextSteps {
		for _, next := range val {
			if _, ok := steps[next]; !ok {
				continue
			}

			if _, ok := viewedSteps[next]; ok {
				continue
			}

			stepList = append(stepList, next)
			viewedSteps[next] = struct{}{}

			blockData, blockErr := ae.DB.GetStepDataFromVersion(ctx, workNumber, next)
			if blockErr != nil {
				return nil, blockErr
			}

			nodes, recErr := ae.getStepsToSkip(ctx, blockData.Next, workNumber, steps, viewedSteps)
			if recErr != nil {
				return nil, recErr
			}

			stepList = append(stepList, nodes...)
		}
	}

	return stepList, nil
}

func (ae *Env) skipTaskBlocksAfterRestart(ctx c.Context, steps *entity.TaskSteps, blockStartTime time.Time,
	nextNodes map[string][]string, workNumber string, workID uuid.UUID, tx db.Database,
) (err error) {
	dbSteps := make(map[string]bool, 0)

	for i := range *steps {
		if (*steps)[i].Time.Before(blockStartTime) {
			continue
		}

		dbSteps[(*steps)[i].Name] = true
	}

	nodesToSkip, skipErr := ae.getStepsToSkip(ctx, nextNodes, workNumber, dbSteps, map[string]struct{}{})
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

func (ae *Env) toMonitoringTaskEventsResponse(ctx c.Context, events []entity.TaskEvent) *MonitoringTaskEvents {
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

			if userFullName == "" {
				fullNameCache[events[i].Author] = events[i].Author
			}
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

func isStepFinished(status string) bool {
	return status == finished || status == skipped || status == cancel || status == noSuccess
}
