package entity

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/iancoleman/orderedmap"
)

type Step struct {
	ID          uuid.UUID                  `json:"-"`
	WorkID      uuid.UUID                  `json:"work_id"`
	WorkNumber  string                     `json:"work_number"`
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
	Initiator   string                     `json:"initiator"`
	UpdatedAt   *time.Time                 `json:"updated_at"`
	IsTest      bool                       `json:"-"`
	ShortTitle  *string                    `json:"short_title,omitempty"`
	Attachments int                        `json:"attachments"`
	IsPaused    bool                       `json:"is_paused"`
}

type TaskSteps []*Step

func (ts *TaskSteps) IsEmpty() bool {
	return len(*ts) == 0
}

type EriusTasks struct {
	Tasks []EriusTask `json:"tasks"`
}

type TasksMeta struct {
	Blueprints map[string][]string `json:"blueprints"` // SD blueprints: [workNumbers]
}

type TaskRelations struct {
	WorkNumber       string
	ParentWorkNumber *string
	ChildWorkNumbers []string
}

type EriusTasksPage struct {
	Tasks     []EriusTask `json:"tasks"`
	Total     int         `json:"total"`
	TasksMeta TasksMeta   `json:"tasks_meta"`
}

type BlueprintSchemas struct {
	ApplicationIDs []string `json:"applicationIds"`
	ID             string   `json:"pipeline_id"`
	Name           string   `json:"pipeline_name"`
	SchemasIDs     []string `json:"schemaIds"`
}

type CountTasks struct {
	TotalActive       int `json:"active"`
	TotalApprover     int `json:"approve"`
	TotalExecutor     int `json:"execute"`
	TotalFormExecutor int `json:"form_execute"`
	TotalSign         int `json:"sign"`
}

type TaskAction struct {
	ID                 string                 `json:"id"`
	Title              string                 `json:"title"`
	ButtonType         string                 `json:"button_type"`
	NodeType           string                 `json:"node_type"`
	CommentEnabled     bool                   `json:"comment_enabled"`
	AttachmentsEnabled bool                   `json:"attachments_enabled"`
	IsPublic           bool                   `json:"-"`
	Params             map[string]interface{} `json:"params,omitempty"`
}

type TaskCompletionInterval struct {
	StartedAt  time.Time
	FinishedAt time.Time
}

type TaskMeanSolveTime struct {
	WorkHours float64
}

type EriusTask struct {
	ID                 uuid.UUID              `json:"id"`
	VersionID          uuid.UUID              `json:"version_id"`
	StartedAt          time.Time              `json:"started_at"`
	LastChangedAt      *time.Time             `json:"last_changed_at"`
	FinishedAt         *time.Time             `json:"finished_at"`
	Name               string                 `json:"name"`
	VersionContent     string                 `json:"-"`
	Description        string                 `json:"description"`
	Status             string                 `json:"status"`
	HumanStatus        string                 `json:"human_status"`
	HumanStatusComment string                 `json:"human_status_comment"`
	Author             string                 `json:"author"`
	IsDebugMode        bool                   `json:"debug"`
	Parameters         map[string]interface{} `json:"parameters"`
	Steps              TaskSteps              `json:"steps"`
	WorkNumber         string                 `json:"work_number"`
	BlueprintID        string                 `json:"blueprint_id"`
	Rate               *int                   `json:"rate"`
	RateComment        *string                `json:"rate_comment"`
	Actions            []TaskAction           `json:"available_actions"`
	IsDelegate         bool                   `json:"is_delegate"`
	IsExpired          bool                   `json:"is_expired"`

	ActiveBlocks           map[string]struct{} `json:"active_blocks"`
	SkippedBlocks          map[string]struct{} `json:"skipped_blocks"`
	NotifiedBlocks         map[string][]string `json:"notified_blocks"`
	PrevUpdateStatusBlocks map[string]string   `json:"prev_update_status_blocks"`
	Total                  int                 `json:"-"`
	AttachmentsCount       *int                `json:"attachments_count"`
	IsTest                 bool                `json:"-"`
	CustomTitle            string              `json:"-"`
	StatusComment          string              `json:"status_comment"`
	StatusAuthor           string              `json:"status_author"`

	ProcessDeadline         *time.Time          `json:"process_deadline"`
	NodeGroup               []*NodeGroup        `json:"node_group"`
	ApprovalList            map[string]string   `json:"approval_list"`
	CurrentExecutor         CurrentExecutorData `json:"current_executor"`
	CurrentExecutionStart   *time.Time          `json:"current_execution_start,omitempty"`
	CurrentApprovementStart *time.Time          `json:"current_approvement_start,omitempty"`
	IsPaused                bool                `json:"is_paused"`
	GroupLimitExceeded      bool                `json:"group_limit_exceeded"`

	ParentWorkNumber *string  `json:"parent_work_number,omitempty"`
	ChildWorkNumbers []string `json:"child_work_numbers,omitempty"`
}

type CurrentExecutorData struct {
	People              []string `json:"people"`
	InitialPeople       []string `json:"initial_people"`
	ExecutionGroupID    string   `json:"execution_group_id,omitempty"`
	ExecutionGroupName  string   `json:"execution_group_name,omitempty"`
	ExecutionGroupLimit int      `json:"execution_group_limit,omitempty"`
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
	Name            *string     `json:"name"`
	Created         *TimePeriod `json:"created"`
	ProcessDeadline *TimePeriod `json:"processDeadline"`
	Order           *string     `json:"order"`
	OrderBy         *[]string   `json:"order_by"`
	Fields          *[]string   `json:"fields"`
	Expired         *bool       `json:"expired"`
	Limit           *int        `json:"limit"`
	Offset          *int        `json:"offset"`
	TaskIDs         *[]string   `json:"task_ids"`
	SelectAs        *string     `json:"select_as"`
	// fot initiator
	Archived         *bool   `json:"archived"`
	ForCarousel      *bool   `json:"forCarousel"`
	Status           *string `json:"status"`
	Receiver         *string `json:"receiver"`
	HasAttachments   *bool   `json:"hasAttachments"`
	SignatureCarrier *string `json:"signature_carrier"`

	Initiator            *[]string `json:"initiator"`
	InitiatorLogins      *[]string `json:"initiatorLogins"`
	InitiatorReq         *bool     `json:"initiatorReq"`
	ProcessingLogins     *[]string `json:"processingLogins"`
	ProcessingGroupIds   *[]string `json:"processingGroupIds"`
	ExecutorLogins       *[]string `json:"executorLogins"`
	ExecutorGroupIDs     *[]string `json:"executorGroupIds"`
	ExecutorTypeAssigned *string   `json:"executorTypeAssigned"`
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
	TaskUpdateActionApprovement                TaskUpdateAction = "approvement"
	TaskUpdateActionAdditionalApprovement      TaskUpdateAction = "additional_approvement"
	TaskUpdateActionSLABreach                  TaskUpdateAction = "sla_breached"
	TaskUpdateActionDayBeforeSLABreach         TaskUpdateAction = "sla_day_before"
	TaskUpdateActionHalfSLABreach              TaskUpdateAction = "half_sla_breached"
	TaskUpdateActionReworkSLABreach            TaskUpdateAction = "rework_sla_breached"
	TaskUpdateActionExecution                  TaskUpdateAction = "execution"
	TaskUpdateActionChangeExecutor             TaskUpdateAction = "change_executor"
	TaskUpdateActionRequestExecutionInfo       TaskUpdateAction = "request_execution_info"
	TaskUpdateActionReplyExecutionInfo         TaskUpdateAction = "reply_execution_info"
	TaskUpdateActionExecutorStartWork          TaskUpdateAction = "executor_start_work"
	TaskUpdateActionBackToGroup                TaskUpdateAction = "back_to_group"
	TaskUpdateActionNewExecutionTask           TaskUpdateAction = "new_execution_task"
	TaskUpdateActionApproverSendEditApp        TaskUpdateAction = "approver_send_edit_app"
	TaskUpdateActionExecutorSendEditApp        TaskUpdateAction = "executor_send_edit_app"
	TaskUpdateActionRequestApproveInfo         TaskUpdateAction = "request_add_info"
	TaskUpdateActionReplyApproverInfo          TaskUpdateAction = "reply_approver_info"
	TaskUpdateActionEditApp                    TaskUpdateAction = "edit_app"
	TaskUpdateActionRequestFillForm            TaskUpdateAction = "fill_form"
	TaskUpdateActionCancelApp                  TaskUpdateAction = "cancel_app"
	TaskUpdateActionAddApprovers               TaskUpdateAction = "add_approvers"
	TaskUpdateActionDayBeforeSLARequestAddInfo TaskUpdateAction = "day_before_sla_request_add_info"
	TaskUpdateActionSLABreachRequestAddInfo    TaskUpdateAction = "sla_breach_request_add_info"
	TaskUpdateActionFormExecutorStartWork      TaskUpdateAction = "form_executor_start_work"
	TaskUpdateActionSign                       TaskUpdateAction = "sign"
	TaskUpdateActionFinishTimer                TaskUpdateAction = "finish_timer"
	TaskUpdateActionFuncSLAExpired             TaskUpdateAction = "func_sla_expired"
	TaskUpdateActionRetry                      TaskUpdateAction = "func_retry"
	TaskUpdateActionSignChangeWorkStatus       TaskUpdateAction = "sign_change_work_status"
	TaskUpdateActionReload                     TaskUpdateAction = "reload"
)

type GetUnfinishedTaskSteps struct {
	ID        uuid.UUID
	StepType  string
	Action    TaskUpdateAction
	StepNames []string
}

type TaskUpdate struct {
	Action     TaskUpdateAction `json:"action"`
	Parameters json.RawMessage  `json:"parameters" swaggertype:"object"`
	StepNames  []string         `json:"step_names"`
}

type CancelAppParams struct {
	Comment string `json:"comment"`
}

func (t *TaskUpdate) IsSchedulerTaskUpdateAction() bool {
	//nolint:exhaustive //нам нужны только эти три кейса
	switch t.Action {
	case TaskUpdateActionFinishTimer, TaskUpdateActionSignChangeWorkStatus,
		TaskUpdateActionFuncSLAExpired, TaskUpdateActionRetry:
		return true
	default:
		return false
	}
}

func (t *TaskUpdate) Validate() error {
	//nolint:exhaustive //нам нужны только эти кейсы
	switch t.Action {
	case TaskUpdateActionApprovement,
		TaskUpdateActionAdditionalApprovement,
		TaskUpdateActionExecution,
		TaskUpdateActionChangeExecutor,
		TaskUpdateActionRequestExecutionInfo,
		TaskUpdateActionExecutorStartWork,
		TaskUpdateActionApproverSendEditApp,
		TaskUpdateActionExecutorSendEditApp,
		TaskUpdateActionRequestApproveInfo,
		TaskUpdateActionRequestFillForm,
		TaskUpdateActionAddApprovers,
		TaskUpdateActionFormExecutorStartWork,
		TaskUpdateActionSign,
		TaskUpdateActionFinishTimer,
		TaskUpdateActionFuncSLAExpired,
		TaskUpdateActionRetry,
		TaskUpdateActionBackToGroup,
		TaskUpdateActionNewExecutionTask,
		TaskUpdateActionSignChangeWorkStatus,
		TaskUpdateActionReplyExecutionInfo,
		TaskUpdateActionReplyApproverInfo:
		return nil
	default:
		return ErrUnknownAction
	}
}

func (t *TaskUpdate) IsApplicationAction() bool {
	return t.Action == TaskUpdateActionCancelApp
}

type NeededNotif struct {
	Initiator   string
	Recipient   string
	WorkNum     string
	Description interface{}
	Status      string
}

type InitialApplication struct {
	Description               string                `json:"description"`
	ApplicationBody           orderedmap.OrderedMap `json:"application_body"`
	AttachmentFields          []string              `json:"attachment_fields"`
	Keys                      map[string]string     `json:"keys"`
	IsTestApplication         bool                  `json:"is_test_application"`
	CustomTitle               string                `json:"custom_title"`
	ApplicationBodyFromSystem orderedmap.OrderedMap `json:"application_body_from_system"`
	HiddenFields              []string              `json:"hidden_fields"`
}

type TaskRunContext struct {
	ClientID           string             `json:"client_id"`
	PipelineID         string             `json:"pipeline_id"`
	InitialApplication InitialApplication `json:"initial_application"`
}

type Attachment struct {
	FileID       string `json:"file_id,omitempty"`
	ExternalLink string `json:"external_link,omitempty"`
}

func (at *Attachment) UnmarshalJSON(b []byte) error {
	var atTemp struct {
		FileID       string `json:"file_id"`
		ExternalLink string `json:"external_link"`
	}

	var stTemp string
	if err := json.Unmarshal(b, &atTemp); err != nil {
		if errStr := json.Unmarshal(b, &stTemp); errStr != nil {
			return err
		}

		_, errParse := uuid.Parse(stTemp)
		if errParse != nil {
			return errParse
		}

		at.FileID = stTemp

		return nil
	}

	at.FileID = atTemp.FileID
	at.ExternalLink = atTemp.ExternalLink

	return nil
}
