package entity

import (
	"time"

	"github.com/google/uuid"
)

type MonitoringTaskNode struct {
	WorkNumber   string `json:"work_number"`
	VersionId    string `json:"version_id"`
	Author       string `json:"author"`
	CreationTime string `json:"creation_time"`
	ScenarioName string `json:"scenario_name"`
	NodeId       string `json:"node_id"`
	RealName     string `json:"real_name"`
	Status       string `json:"status"`
	StepName     string `json:"step_name"`
}

type BlockOutputs []BlockOutputValue

type BlockOutputValue struct {
	StepName string
	Name     string
	Value    interface{}
}

type TasksForMonitoringFilters struct {
	PerPage    *int
	Page       *int
	SortColumn *string
	SortOrder  *string
	Filter     *string
	FromDate   *string
	ToDate     *string
}

type TaskForMonitoring struct {
	Id          uuid.UUID
	Initiator   string
	ProcessName string
	StartedAt   time.Time
	Status      string
	WorkNumber  string
}

type TasksForMonitoring struct {
	Tasks []TaskForMonitoring
	Total int
}

type BlockInputs []BlockInputValue

type BlockInputValue struct {
	Name  string
	Value interface{}
}
