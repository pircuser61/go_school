package pipeline

import (
	"errors"
	"fmt"
	"time"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

type RequestInfoType string

type ExecutionDecision string

func (a ExecutionDecision) String() string {
	return string(a)
}

type ExecutorEditApp struct {
	Executor    string    `json:"executor"`
	Comment     string    `json:"comment"`
	Attachments []string  `json:"attachments"`
	CreatedAt   time.Time `json:"created_at"`
}

type RequestExecutionInfoLog struct {
	Login       string          `json:"login"`
	Comment     string          `json:"comment"`
	CreatedAt   time.Time       `json:"created_at"`
	ReqType     RequestInfoType `json:"req_type"`
	Attachments []string        `json:"attachments"`
}

type ChangeExecutorLog struct {
	OldLogin    string    `json:"old_login"`
	NewLogin    string    `json:"new_login"`
	Comment     string    `json:"comment"`
	Attachments []string  `json:"attachments"`
	CreatedAt   time.Time `json:"created_at"`
}

type ExecutionData struct {
	ExecutionType       script.ExecutionType `json:"execution_type"`
	Executors           map[string]struct{}  `json:"executors"`
	Decision            *ExecutionDecision   `json:"decision,omitempty"`
	DecisionAttachments []string             `json:"decision_attachments,omitempty"`
	DecisionComment     *string              `json:"comment,omitempty"`
	ActualExecutor      *string              `json:"actual_executor,omitempty"`
	SLA                 int                  `json:"sla"`
	DidSLANotification  bool                 `json:"did_sla_notification"`

	EditingApp               *ExecutorEditApp           `json:"editing_app,omitempty"`
	EditingAppLog            []ExecutorEditApp          `json:"editing_app_log,omitempty"`
	ChangedExecutorsLogs     []ChangeExecutorLog        `json:"change_executors_logs,omitempty"`
	RequestExecutionInfoLogs []RequestExecutionInfoLog  `json:"request_execution_info_logs,omitempty"`
	FormsAccessibility       []script.FormAccessibility `json:"forms_accessibility,omitempty"`

	ExecutorsGroupID   string `json:"executors_group_id"`
	ExecutorsGroupName string `json:"executors_group_name"`

	LeftToNotify map[string]struct{} `json:"left_to_notify"`

	IsTakenInWork               bool `json:"is_taken_in_work"`
	IsExecutorVariablesResolved bool `json:"is_executor_variables_resolved"`

	IsRevoked          bool `json:"is_revoked"`
	IsEditable         bool `json:"is_editable"`
	RepeatPrevDecision bool `json:"repeat_prev_decision"`
}

func (a *ExecutionData) GetDecision() *ExecutionDecision {
	return a.Decision
}

func (a *ExecutionData) IncreaseSLA(addSla int) {
	a.SLA += addSla
}

func (a *ExecutionData) setEditApp(login string, params executorUpdateEditParams) error {
	_, ok := a.Executors[login]
	if !ok && login != AutoApprover {
		return fmt.Errorf("%s not found in executors", login)
	}

	if a.Decision != nil {
		return errors.New("decision already set")
	}

	editing := &ExecutorEditApp{
		Executor:    login,
		Comment:     params.Comment,
		Attachments: params.Attachments,
		CreatedAt:   time.Now(),
	}

	a.EditingAppLog = append(a.EditingAppLog, *editing)

	a.EditingApp = editing

	return nil
}
