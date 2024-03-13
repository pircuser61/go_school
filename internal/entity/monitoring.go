package entity

import (
	"time"
)

type MonitoringTaskNode struct {
	WorkNumber    string     `json:"work_number"`
	VersionID     string     `json:"version_id"`
	IsPaused      bool       `json:"task_is_paused"`
	Author        string     `json:"author"`
	CreationTime  string     `json:"creation_time"`
	ScenarioName  string     `json:"scenario_name"`
	BlockID       string     `json:"block_id"`
	RealName      string     `json:"real_name"`
	Status        string     `json:"status"`
	NodeID        string     `json:"node_id"`
	BlockDateInit *time.Time `json:"block_date_init"`
	BlockIsPaused bool       `json:"block_is_paused"`
}

type BlockOutputs []BlockOutputValue

type BlockOutputValue struct {
	StepName string
	Name     string
	Value    interface{}
}

type TasksForMonitoringFilters struct {
	PerPage      *int
	Page         *int
	SortColumn   *string
	SortOrder    *string
	Filter       *string
	FromDate     *string
	ToDate       *string
	StatusFilter []string
}

type TaskForMonitoring struct {
	Initiator        string
	ProcessName      string
	StartedAt        time.Time
	FinishedAt       *time.Time
	ProcessDeletedAt *time.Time
	Status           string
	WorkNumber       string
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

type BlockState []BlockStateValue

type BlockStateValue struct {
	Name  string
	Value interface{}
}
