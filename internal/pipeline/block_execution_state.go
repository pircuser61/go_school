package pipeline

import (
	"errors"
	"fmt"
	"time"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"

	human_tasks "gitlab.services.mts.ru/jocasta/pipeliner/internal/human-tasks"
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
	DelegateFor string    `json:"delegate_for"`
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
	DelegateFor         string               `json:"delegate_for"`

	EditingApp               *ExecutorEditApp           `json:"editing_app,omitempty"`
	EditingAppLog            []ExecutorEditApp          `json:"editing_app_log,omitempty"`
	ChangedExecutorsLogs     []ChangeExecutorLog        `json:"change_executors_logs,omitempty"`
	RequestExecutionInfoLogs []RequestExecutionInfoLog  `json:"request_execution_info_logs,omitempty"`
	FormsAccessibility       []script.FormAccessibility `json:"forms_accessibility,omitempty"`

	ExecutorsGroupID   string `json:"executors_group_id"`
	ExecutorsGroupName string `json:"executors_group_name"`

	IsTakenInWork               bool `json:"is_taken_in_work"`
	IsExecutorVariablesResolved bool `json:"is_executor_variables_resolved"`

	IsRevoked          bool `json:"is_revoked"`
	IsEditable         bool `json:"is_editable"`
	RepeatPrevDecision bool `json:"repeat_prev_decision"`

	SLA            int  `json:"sla"`
	CheckSLA       bool `json:"check_sla"`
	SLAChecked     bool `json:"sla_checked"`
	HalfSLAChecked bool `json:"half_sla_checked"`
}

func (a *ExecutionData) GetDecision() *ExecutionDecision {
	return a.Decision
}

func (a *ExecutionData) IncreaseSLA(addSla int) {
	a.SLA += addSla
}

func (a *ExecutionData) GetRepeatPrevDecision() bool {
	return a.RepeatPrevDecision
}

func (a *ExecutionData) GetExecutors() []string {
	executors := make([]string, 0, len(a.Executors))
	for executor := range a.Executors {
		executors = append(executors, executor)
	}

	return executors
}

func (a *ExecutionData) setEditApp(login string, params executorUpdateEditParams, delegations human_tasks.Delegations) error {
	_, executorFound := a.Executors[login]

	delegateFor, isDelegate := delegations.FindDelegatorFor(login, getSliceFromMapOfStrings(a.Executors))
	if !(executorFound || isDelegate) && login != AutoApprover {
		return fmt.Errorf("%s not found in executors or delegates", login)
	}

	if a.Decision != nil {
		return errors.New("decision already set")
	}

	editing := &ExecutorEditApp{
		Executor:    login,
		Comment:     params.Comment,
		Attachments: params.Attachments,
		CreatedAt:   time.Now(),
		DelegateFor: delegateFor,
	}

	a.EditingAppLog = append(a.EditingAppLog, *editing)
	a.EditingApp = editing

	return nil
}

func (a *ExecutionData) GetIsEditable() bool {
	return a.IsEditable
}
