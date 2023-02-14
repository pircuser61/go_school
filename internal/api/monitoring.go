package api

import (
	"net/http"
)

func (ae *APIEnv) GetTasksForMonitoring(w http.ResponseWriter, req *http.Request, params GetTasksForMonitoringParams) {
	panic("implement me")
}

func (ae *APIEnv) GetMonitoringTask(w http.ResponseWriter, req *http.Request, workNumber string) {
	panic("implement me")
}
