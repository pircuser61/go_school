package entity

import (
	"time"

	"github.com/google/uuid"
)

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
