package entity

import (
	"encoding/json"
	"reflect"
	"time"

	"github.com/google/uuid"

	"github.com/iancoleman/orderedmap"

	"github.com/pkg/errors"
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
	LastChangedAt      time.Time              `json:"last_changed_at"`
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

	ProcessDeadline time.Time         `json:"process_deadline"`
	NodeGroup       []*NodeGroup      `json:"node_group"`
	ApprovalList    map[string]string `json:"approval_list"`
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
	// fot initiator
	Archived         *bool   `json:"archived"`
	ForCarousel      *bool   `json:"forCarousel"`
	Status           *string `json:"status"`
	Receiver         *string `json:"receiver"`
	HasAttachments   *bool   `json:"hasAttachments"`
	SignatureCarrier *string `json:"signature_carrier"`

	Initiator            *[]string `json:"initiator"`
	InitiatorLogins      *[]string `json:"initiatorLogins"`
	ProcessingLogins     *[]string `json:"processingLogins"`
	ProcessingGroupIds   *[]string `json:"processingGroupIds"`
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
	TaskUpdateActionApproverSendEditApp        TaskUpdateAction = "approver_send_edit_app"
	TaskUpdateActionExecutorSendEditApp        TaskUpdateAction = "executor_send_edit_app"
	TaskUpdateActionRequestApproveInfo         TaskUpdateAction = "request_add_info"
	TaskUpdateActionReplyApproverInfo          TaskUpdateAction = "reply_approver_info"
	TaskUpdateActionRequestFillForm            TaskUpdateAction = "fill_form"
	TaskUpdateActionCancelApp                  TaskUpdateAction = "cancel_app"
	TaskUpdateActionAddApprovers               TaskUpdateAction = "add_approvers"
	TaskUpdateActionDayBeforeSLARequestAddInfo TaskUpdateAction = "day_before_sla_request_add_info"
	TaskUpdateActionSLABreachRequestAddInfo    TaskUpdateAction = "sla_breach_request_add_info"
	TaskUpdateActionFormExecutorStartWork      TaskUpdateAction = "form_executor_start_work"
	TaskUpdateActionSign                       TaskUpdateAction = "sign"
	TaskUpdateActionFinishTimer                TaskUpdateAction = "finish_timer"
	TaskUpdateActionFuncSLAExpired             TaskUpdateAction = "func_sla_expired"
	TaskUpdateActionSignChangeWorkStatus       TaskUpdateAction = "sign_change_work_status"
)

var (
	//nolint:gochecknoglobals //система проектировалась без этого линтера поэтому gg
	checkTaskUpdateMap = map[TaskUpdateAction]struct{}{
		TaskUpdateActionApprovement:           {},
		TaskUpdateActionAdditionalApprovement: {},
		TaskUpdateActionExecution:             {},
		TaskUpdateActionChangeExecutor:        {},
		TaskUpdateActionRequestExecutionInfo:  {},
		TaskUpdateActionExecutorStartWork:     {},
		TaskUpdateActionApproverSendEditApp:   {},
		TaskUpdateActionExecutorSendEditApp:   {},
		TaskUpdateActionRequestApproveInfo:    {},
		TaskUpdateActionRequestFillForm:       {},
		TaskUpdateActionAddApprovers:          {},
		TaskUpdateActionFormExecutorStartWork: {},
		TaskUpdateActionSign:                  {},
		TaskUpdateActionFinishTimer:           {},
		TaskUpdateActionFuncSLAExpired:        {},
		TaskUpdateActionSignChangeWorkStatus:  {},
		TaskUpdateActionReplyExecutionInfo:    {},
		TaskUpdateActionReplyApproverInfo:     {},
	}
	// nolint:gochecknoglobals //система проектировалась без этого линтера поэтому gg
	checkTaskUpdateAppMap = map[TaskUpdateAction]struct{}{
		TaskUpdateActionCancelApp: {},
	}
)

type TaskUpdate struct {
	Action     TaskUpdateAction `json:"action"`
	Parameters json.RawMessage  `json:"parameters" swaggertype:"object"`
	StepNames  []string         `json:"step_names"`
}

type CancelAppParams struct {
	Comment string `json:"comment"`
}

func (t *TaskUpdate) Validate() error {
	if _, ok := checkTaskUpdateMap[t.Action]; !ok {
		return errors.New("unknown action")
	}

	return nil
}

func (t *TaskUpdate) IsApplicationAction() bool {
	_, ok := checkTaskUpdateAppMap[t.Action]

	return ok
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

type NodeEvent struct {
	TaskID        string                 `json:"task_id"`
	WorkNumber    string                 `json:"work_number"`
	NodeName      string                 `json:"node_name"`
	NodeShortName string                 `json:"node_short_name"`
	NodeStart     string                 `json:"node_start"`
	NodeEnd       string                 `json:"node_end"`
	TaskStatus    string                 `json:"task_status"`
	NodeStatus    string                 `json:"node_status"`
	NodeOutput    map[string]interface{} `json:"node_output"`
}

func (ne NodeEvent) ToMap() map[string]interface{} {
	if ne.NodeOutput == nil {
		ne.NodeOutput = make(map[string]interface{})
	}

	res := make(map[string]interface{})

	for i := 0; i < reflect.TypeOf(ne).NumField(); i++ {
		f := reflect.TypeOf(ne).Field(i)
		k := f.Tag.Get("json")

		if k == "" {
			continue
		}

		val := reflect.ValueOf(ne).Field(i).Interface()
		res[k] = val
	}

	return res
}
