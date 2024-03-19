package pipeline

import (
	"fmt"
	"time"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	human_tasks "gitlab.services.mts.ru/jocasta/pipeliner/internal/humantasks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

type (
	RequestInfoType   string
	ExecutionDecision string
)

func (a ExecutionDecision) String() string {
	return string(a)
}

type ExecutorEditApp struct {
	Executor    string              `json:"executor"`
	Comment     string              `json:"comment"`
	Attachments []entity.Attachment `json:"attachments"`
	CreatedAt   time.Time           `json:"created_at"`
	DelegateFor string              `json:"delegate_for"`
}

type RequestExecutionInfoLog struct {
	Login       string              `json:"login"`
	Comment     string              `json:"comment"`
	CreatedAt   time.Time           `json:"created_at"`
	ReqType     RequestInfoType     `json:"req_type"`
	Attachments []entity.Attachment `json:"attachments"`
	DelegateFor string              `json:"delegate_for"`
}

type ChangeExecutorLog struct {
	OldLogin    string              `json:"old_login"`
	NewLogin    string              `json:"new_login"`
	Comment     string              `json:"comment"`
	Attachments []entity.Attachment `json:"attachments"`
	CreatedAt   time.Time           `json:"created_at"`
	DelegateFor string              `json:"delegate_for"`
	ByLogin     string              `json:"by_login"`
}

type StartWorkLog struct {
	Executor    string    `json:"executor"`
	CreatedAt   time.Time `json:"created_at"`
	DelegateFor string    `json:"delegate_for"`
}

type ExecutionData struct {
	ExecutionType       script.ExecutionType `json:"execution_type"`
	Executors           map[string]struct{}  `json:"executors"`
	InitialExecutors    map[string]struct{}  `json:"initial_executors"`
	Decision            *ExecutionDecision   `json:"decision,omitempty"`
	DecisionAttachments []entity.Attachment  `json:"decision_attachments,omitempty"`
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

	ExecutorsGroupIDPath *string `json:"executors_group_id_path,omitempty"`

	IsTakenInWork               bool           `json:"is_taken_in_work"`
	TakenInWorkLog              []StartWorkLog `json:"taken_in_work_log"`
	IsExecutorVariablesResolved bool           `json:"is_executor_variables_resolved"`

	IsEditable         bool `json:"is_editable"`
	RepeatPrevDecision bool `json:"repeat_prev_decision"`
	UseActualExecutor  bool `json:"use_actual_executor"`

	Deadline                     time.Time `json:"deadline,omitempty"`
	SLA                          int       `json:"sla"`
	CheckSLA                     bool      `json:"check_sla"`
	SLAChecked                   bool      `json:"sla_checked"`
	HalfSLAChecked               bool      `json:"half_sla_checked"`
	ReworkSLA                    int       `json:"rework_sla"`
	CheckReworkSLA               bool      `json:"check_rework_sla"`
	CheckDayBeforeSLARequestInfo bool      `json:"check_day_before_sla_request_info"`
	WorkType                     string    `json:"work_type"`
	HideExecutor                 bool      `json:"hide_executor"`
}

func (a *ExecutionData) GetDecision() *ExecutionDecision {
	return a.Decision
}

func (a *ExecutionData) IncreaseSLA(addSLA int) {
	a.SLA += addSLA
}

func (a *ExecutionData) GetRepeatPrevDecision() bool {
	return a.RepeatPrevDecision
}

//nolint:dupl //its not duplicate
func (a *ExecutionData) setEditAppToInitiator(login, delegateFor string, params executorUpdateEditParams) error {
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

func (a *ExecutionData) SetDecision(login string, in *ExecutionUpdateParams, delegations human_tasks.Delegations) error {
	_, executorFound := a.Executors[login]

	delegateFor, isDelegate := delegations.FindDelegatorFor(login, getSliceFromMap(a.Executors))
	if !(executorFound || isDelegate) {
		return NewUserIsNotPartOfProcessErr()
	}

	if a.Decision != nil {
		return errors.New("decision already set")
	}

	if in.Decision != ExecutionDecisionExecuted && in.Decision != ExecutionDecisionRejected {
		return fmt.Errorf("unknown decision %s", in.Decision)
	}

	a.Decision = &in.Decision
	a.DecisionComment = &in.Comment
	a.DecisionAttachments = in.Attachments
	a.ActualExecutor = &login
	a.DelegateFor = delegateFor

	return nil
}

//nolint:dupl //its not duplicate
func (a *ExecutionData) setEditToNextBlock(executor, delegateFor string, params executorUpdateEditParams) error {
	rejected := ExecutionDecisionSentEdit
	a.ActualExecutor = &executor
	a.Decision = &rejected
	a.DecisionComment = &params.Comment
	a.DecisionAttachments = params.Attachments
	a.DelegateFor = delegateFor

	return nil
}

func (a *ExecutionData) GetIsEditable() bool {
	return a.IsEditable
}

func (a *ExecutionData) latestUnansweredAddInfoLogEntry() *RequestExecutionInfoLog {
	if len(a.RequestExecutionInfoLogs) == 0 {
		return nil
	}

	return &a.RequestExecutionInfoLogs[len(a.RequestExecutionInfoLogs)-1]
}
