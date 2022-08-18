package entity

import (
	"encoding/json"
	"time"

	"github.com/pkg/errors"

	"github.com/google/uuid"
)

type Step struct {
	ID          uuid.UUID                  `json:"-"`
	Time        time.Time                  `json:"time"`
	Type        string                     `json:"type"`
	Name        string                     `json:"name"`
	State       map[string]json.RawMessage `json:"state" swaggertype:"object"`
	Storage     map[string]interface{}     `json:"storage"`
	Errors      []string                   `json:"errors"`
	Steps       []string                   `json:"steps"`
	BreakPoints []string                   `json:"-"`
	HasError    bool                       `json:"has_error"`
	Status      string                     `json:"status"`
}

type TaskSteps []*Step

func (ts *TaskSteps) IsEmpty() bool {
	return len(*ts) == 0
}

type EriusTasks struct {
	Tasks []EriusTask `json:"tasks"`
}

type EriusTasksPage struct {
	Tasks []EriusTask `json:"tasks"`
	Total int         `json:"total"`
}

type CountTasks struct {
	TotalActive   int `json:"active"`
	TotalApprover int `json:"approve"`
	TotalExecutor int `json:"execute"`
}

type EriusTask struct {
	ID            uuid.UUID              `json:"id"`
	VersionID     uuid.UUID              `json:"version_id"`
	StartedAt     time.Time              `json:"started_at"`
	LastChangedAt time.Time              `json:"last_changed_at"`
	Name          string                 `json:"name"`
	Description   string                 `json:"description"`
	Status        string                 `json:"status"`
	HumanStatus   string                 `json:"human_status"`
	Author        string                 `json:"author"`
	IsDebugMode   bool                   `json:"debug"`
	Parameters    map[string]interface{} `json:"parameters"`
	Steps         TaskSteps              `json:"steps"`
	WorkNumber    string                 `json:"work_number"`
	BlueprintID   string                 `json:"blueprint_id"`
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
	Name     *string     `json:"name"`
	Created  *TimePeriod `json:"created"`
	Order    *string     `json:"order"`
	Limit    *int        `json:"limit"`
	Offset   *int        `json:"offset"`
	TaskIDs  *[]string   `json:"task_ids"`
	SelectAs *string     `json:"select_as"`
	Archived *bool       `json:"archived"`
}

type TimePeriod struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type TaskFilter struct {
	GetTaskParams
	CurrentUser string
}

type TaskUpdateAction string

const (
	TaskUpdateActionApprovement          TaskUpdateAction = "approvement"
	TaskUpdateActionExecution            TaskUpdateAction = "execution"
	TaskUpdateActionChangeExecutor       TaskUpdateAction = "change_executor"
	TaskUpdateActionRequestExecutionInfo TaskUpdateAction = "request_execution_info"
	TaskUpdateActionSendEditApp          TaskUpdateAction = "send_edit_app"
	TaskUpdateActionCancelApp            TaskUpdateAction = "cancel_app"
	TaskUpdateActionRequestAddInfo       TaskUpdateAction = "request_add_info"
)

type TaskUpdate struct {
	Action     TaskUpdateAction `json:"action" enums:"approvement,execution,change_executor,request_execution_info"`
	Parameters json.RawMessage  `json:"parameters" swaggertype:"object"`
}

func (t *TaskUpdate) Validate() error {
	if t.Action != TaskUpdateActionApprovement &&
		t.Action != TaskUpdateActionExecution &&
		t.Action != TaskUpdateActionCancelApp &&
		t.Action != TaskUpdateActionRequestExecutionInfo &&
		t.Action != TaskUpdateActionSendEditApp &&
		t.Action != TaskUpdateActionChangeExecutor {
		return errors.New("unknown action")
	}

	return nil
}
