package api

import (
	c "context"
	"net/http"
	"regexp"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"

	"go.opencensus.io/trace"
)

func (ae *Env) MonitoringGetTasks(w http.ResponseWriter, r *http.Request, params MonitoringGetTasksParams) {
	ctx, span := trace.StartSpan(r.Context(), "monitoring_get_tasks")
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

func (ae *Env) MonitoringGetTask(w http.ResponseWriter, req *http.Request, workNumber string, params MonitoringGetTaskParams) {
	ctx, s := trace.StartSpan(req.Context(), "monitoring_get_task")
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

	steps, err := ae.DB.GetTaskForMonitoring(ctx, workNumber, params.FromEventId, params.ToEventId)
	if err != nil {
		errorHandler.handleError(GetMonitoringNodesError, err)

		return
	}

	if len(steps) == 0 {
		errorHandler.handleError(NoProcessNodesForMonitoringError, errors.New("no process steps for monitoring"))

		return
	}

	if err = sendResponse(w, http.StatusOK, toMonitoringTaskResponse(steps, events)); err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

func (ae *Env) getUserFullName(ctx c.Context, username string) (string, error) {
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

//nolint:all // ok
func toMonitoringTaskResponse(steps []entity.MonitoringTaskStep, events []entity.TaskEvent) *MonitoringTask {
	const (
		finished = 2
		canceled = 6
	)

	res := &MonitoringTask{
		History:  make([]MonitoringHistory, 0),
		TaskRuns: make([]MonitoringTaskRun, 0),
	}
	res.ScenarioInfo = MonitoringScenarioInfo{
		Author:       steps[0].Author,
		CreationTime: steps[0].CreationTime,
		ScenarioName: steps[0].ScenarioName,
	}
	res.VersionId = steps[0].VersionID
	res.WorkNumber = steps[0].WorkNumber
	res.WorkId = steps[0].WorkID
	res.IsPaused = steps[0].IsPaused
	res.TaskRuns = getRunsByEvents(events)
	res.IsFinished = steps[0].WorkStatus == finished || steps[0].WorkStatus == canceled

	for i := range steps {
		monitoringHistory := MonitoringHistory{
			BlockId:  steps[i].BlockID,
			RealName: steps[i].RealName,
			Status:   getMonitoringStatus(steps[i].Status),
			NodeId:   steps[i].NodeID,
			IsPaused: steps[i].BlockIsPaused,
		}

		if steps[i].BlockDateInit != nil {
			formattedTime := steps[i].BlockDateInit.Format(monitoringTimeLayout)
			monitoringHistory.BlockDateInit = &formattedTime
		}

		res.History = append(res.History, monitoringHistory)
	}

	return res
}
