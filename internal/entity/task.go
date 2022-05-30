package entity

import (
	"time"

	"github.com/google/uuid"
)

type Step struct {
	Time        time.Time              `json:"time"`
	Name        string                 `json:"name"`
	Storage     map[string]interface{} `json:"storage"`
	Errors      []string               `json:"errors"`
	Steps       []string               `json:"steps"`
	BreakPoints []string               `json:"-"`
	HasError    bool                   `json:"has_error"`
}

type TaskSteps []*Step

func (ts *TaskSteps) IsEmpty() bool {
	return len(*ts) == 0
}

type EriusTasks struct {
	Tasks []EriusTask `json:"tasks"`
}

type EriusTask struct {
	ID          uuid.UUID              `json:"id"`
	VersionID   uuid.UUID              `json:"version_id"`
	StartedAt   time.Time              `json:"started_at"`
	Status      string                 `json:"status"`
	Author      string                 `json:"author"`
	IsDebugMode bool                   `json:"debug"`
	Parameters  map[string]interface{} `json:"parameters"`
	Steps       TaskSteps              `json:"steps"`
}

func (et *EriusTask) IsRun() bool {
	return et.Status == "run"
}

func (et *EriusTask) IsCreated() bool {
	return et.Status == "created"
}

func (et *EriusTask) IsStopped() bool {
	return et.Status == "stopped"
}

func (et *EriusTask) IsFinished() bool {
	return et.Status == "finished"
}

func (et *EriusTask) IsError() bool {
	return et.Status == "error"
}

type GetTaskParams struct {
	Name    *string     `json:"name"`
	Created *TimePeriod `json:"created"`
	Order   *string     `json:"order"`
	Limit   *int        `json:"limit"`
	Offset  *int        `json:"offset"`
}

type TimePeriod struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type TaskFilter struct {
	GetTaskParams
	CurrentUser string
	TaskID      string
}
