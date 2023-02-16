package api

import (
	"net/http"

	"github.com/pkg/errors"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"
)

func (ae *APIEnv) GetTasksForMonitoring(w http.ResponseWriter, req *http.Request, params GetTasksForMonitoringParams) {
	panic("implement me")
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
	res := MonitoringTask{History: make([]MonitoringHistory, 0)}
	if nodes != nil {
		res.ScenarioInfo = MonitoringScenarioInfo{
			Author:       nodes[0].Author,
			CreationTime: nodes[0].CreationTime,
			ScenarioName: nodes[0].ScenarioName,
		}
		res.VersionId = nodes[0].VersionId
		res.WorkNumber = nodes[0].WorkNumber
	}

	for i := range nodes {
		res.History = append(res.History, MonitoringHistory{
			NodeId:   nodes[i].NodeId,
			RealName: nodes[i].RealName,
			Status:   getMonitoringStatus(nodes[i].Status),
			StepName: nodes[i].StepName,
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
		{
			return MonitoringHistoryStatusFinished
		}
	default:
		return MonitoringHistoryStatusRunning
	}
}
