package pipeline

import (
	"time"

	en "gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	ht "gitlab.services.mts.ru/jocasta/pipeliner/internal/humantasks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

type ApproverAction string

const (
	ApproverActionApprove    = "approve"
	ApproverActionReject     = "reject"
	ApproverActionViewed     = "viewed"
	ApproverActionInformed   = "informed"
	ApproverActionSign       = "sign"
	ApproverActionSignUkep   = "sign_ukep"
	ApproverActionConfirm    = "confirm"
	ApproverActionSendToEdit = "approver_send_edit_app"

	ApproverDecisionApprovedRU = "согласен"
	ApproverDecisionRejectedRU = "не согласен"
)

func ApproverActionFromString(action *string) *ApproverAction {
	if action == nil {
		return nil
	}

	appAction := ApproverAction(*action)

	return &appAction
}

func (a ApproverAction) ToDecision() ApproverDecision {
	switch a {
	case ApproverActionApprove:
		return ApproverDecisionApproved
	case ApproverActionReject:
		return ApproverDecisionRejected
	case ApproverActionViewed:
		return ApproverDecisionViewed
	case ApproverActionInformed:
		return ApproverDecisionInformed
	case ApproverActionSign:
		return ApproverDecisionSigned
	case ApproverActionSignUkep:
		return ApproverDecisionSignedUkep
	case ApproverActionConfirm:
		return ApproverDecisionConfirmed
	default:
		return ""
	}
}

type ApproverDecision string

func (a ApproverDecision) String() string {
	return string(a)
}

func (a ApproverDecision) ToAction() ApproverAction {
	switch a {
	case ApproverDecisionApproved:
		return ApproverActionApprove
	case ApproverDecisionRejected:
		return ApproverActionReject
	case ApproverDecisionViewed:
		return ApproverActionViewed
	case ApproverDecisionInformed:
		return ApproverActionInformed
	case ApproverDecisionSigned:
		return ApproverActionSign
	case ApproverDecisionSignedUkep:
		return ApproverActionSignUkep
	case ApproverDecisionConfirmed:
		return ApproverActionConfirm
	case ApproverDecisionSentToEdit:
		return ApproverActionSendToEdit
	default:
		return ""
	}
}

func (a ApproverDecision) ToRuString() string {
	// nolint:exhaustive //не хотим обрабатывать остальные случаи
	switch a {
	case ApproverDecisionApproved:
		return ApproverDecisionApprovedRU
	case ApproverDecisionRejected:
		return ApproverDecisionRejectedRU
	default:
		return string(a)
	}
}

const (
	ApproverDecisionApproved   ApproverDecision = "approved"
	ApproverDecisionRejected   ApproverDecision = "rejected"
	ApproverDecisionViewed     ApproverDecision = "viewed"
	ApproverDecisionInformed   ApproverDecision = "informed"
	ApproverDecisionSigned     ApproverDecision = "signed"
	ApproverDecisionSignedUkep ApproverDecision = "signed_ukep"
	ApproverDecisionConfirmed  ApproverDecision = "confirmed"
	ApproverDecisionSentToEdit ApproverDecision = "sent_to_edit"
)

type ApproverEditingApp struct {
	Approver    string          `json:"approver"`
	Comment     string          `json:"comment"`
	Attachments []en.Attachment `json:"attachments"`
	CreatedAt   time.Time       `json:"created_at"`
	DelegateFor string          `json:"delegate_for"`
}

type AdditionalInfoType string

const (
	RequestAddInfoType AdditionalInfoType = "request"
	ReplyAddInfoType   AdditionalInfoType = "reply"
)

type ApproverLogType string

const (
	ApproverLogDecision           ApproverLogType = "decision"
	AdditionalApproverLogDecision ApproverLogType = "additionalApproverDecision"
	ApproverLogAddApprover        ApproverLogType = "addApprover"
)

type AdditionalInfo struct {
	ID          string             `json:"id"`
	Login       string             `json:"login"`
	Comment     string             `json:"comment"`
	Attachments []en.Attachment    `json:"attachments"`
	LinkID      *string            `json:"link_id,omitempty"`
	Type        AdditionalInfoType `json:"type"`
	CreatedAt   time.Time          `json:"created_at"`
	DelegateFor string             `json:"delegate_for"`
}

type ApproverLogEntry struct {
	Login          string           `json:"login"`
	Decision       ApproverDecision `json:"decision"`
	Comment        string           `json:"comment"`
	CreatedAt      time.Time        `json:"created_at"`
	Attachments    []en.Attachment  `json:"attachments"`
	AddedApprovers []string         `json:"added_approvers"`
	LogType        ApproverLogType  `json:"log_type"`
	DelegateFor    string           `json:"delegate_for"`
}

type ApproverData struct {
	Type                script.ApproverType    `json:"type"`
	Approvers           map[string]struct{}    `json:"approvers"`
	Decision            *ApproverDecision      `json:"decision,omitempty"`
	DecisionAttachments []en.Attachment        `json:"decision_attachments,omitempty"`
	Comment             *string                `json:"comment,omitempty"`
	ActualApprover      *string                `json:"actual_approver,omitempty"`
	ApprovementRule     script.ApprovementRule `json:"approvementRule,omitempty"`
	ApproverLog         []ApproverLogEntry     `json:"approver_log,omitempty"`

	WaitAllDecisions   bool                 `json:"wait_all_decisions"`
	IsExpired          bool                 `json:"is_expired"`
	IsEditable         bool                 `json:"is_editable"`
	RepeatPrevDecision bool                 `json:"repeat_prev_decision"`
	EditingApp         *ApproverEditingApp  `json:"editing_app,omitempty"`
	EditingAppLog      []ApproverEditingApp `json:"editing_app_log,omitempty"`

	FormsAccessibility []script.FormAccessibility `json:"forms_accessibility,omitempty"`

	ApproversGroupID   string `json:"approvers_group_id"`
	ApproversGroupName string `json:"approvers_group_name"`

	ApproversGroupIDPath *string `json:"approvers_group_id_path,omitempty"`

	AddInfo []AdditionalInfo `json:"additional_info,omitempty"`

	ApproveStatusName string `json:"approve_status_name"`

	Deadline                     time.Time `json:"deadline,omitempty"`
	SLA                          int       `json:"sla"`
	CheckSLA                     bool      `json:"check_sla"`
	SLAChecked                   bool      `json:"sla_checked"`
	HalfSLAChecked               bool      `json:"half_sla_checked"`
	ReworkSLA                    int       `json:"rework_sla"`
	CheckReworkSLA               bool      `json:"check_rework_sla"`
	CheckDayBeforeSLARequestInfo bool      `json:"check_day_before_sla_request_info"`
	WorkType                     string    `json:"work_type"`

	AutoAction *ApproverAction `json:"auto_action,omitempty"`

	ActionList []Action `json:"action_list"`

	AdditionalApprovers []AdditionalApprover `json:"additional_approvers"`
}

func NewApproverState() *ApproverData {
	return &ApproverData{
		ApproverLog:         make([]ApproverLogEntry, 0),
		EditingAppLog:       make([]ApproverEditingApp, 0),
		FormsAccessibility:  make([]script.FormAccessibility, 0),
		AddInfo:             make([]AdditionalInfo, 0),
		ActionList:          make([]Action, 0),
		AdditionalApprovers: make([]AdditionalApprover, 0),
	}
}

type Action struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title"`
}

type AdditionalApprover struct {
	ApproverLogin     string            `json:"approver_login"`
	BaseApproverLogin string            `json:"base_approver_login"`
	Question          *string           `json:"question"`
	Comment           *string           `json:"comment"`
	Attachments       []en.Attachment   `json:"attachments"`
	Decision          *ApproverDecision `json:"decision"`
	CreatedAt         time.Time         `json:"created_at"`
	DecisionTime      *time.Time        `json:"decision_time"`
}

func (a *ApproverData) GetDecision() *ApproverDecision {
	return a.Decision
}

func (a *ApproverData) GetRepeatPrevDecision() bool {
	return a.RepeatPrevDecision
}

func (a *ApproverData) GetIsEditable() bool {
	return a.IsEditable
}

func (a *ApproverData) GetApproversGroupID() string {
	return a.ApproversGroupID
}

func (a *ApproverData) userIsAnyApprover(login string) bool {
	if login == AutoApprover {
		return true
	}

	_, ok := a.Approvers[login]
	if ok {
		return true
	}

	for _, approver := range a.AdditionalApprovers {
		if approver.Decision == nil && approver.ApproverLogin == login {
			return true
		}
	}

	return false
}

func (a *ApproverData) userIsDelegate(login string, delegations ht.Delegations) (delegateFor string, ok bool) {
	delegators := delegations.GetDelegators(login)
	for approver := range a.Approvers {
		for _, delegator := range delegators {
			if delegator == approver {
				return delegator, true
			}
		}
	}

	for _, addApprover := range a.AdditionalApprovers {
		for _, delegator := range delegators {
			if delegator == addApprover.ApproverLogin {
				return delegator, true
			}
		}
	}

	return "", false
}

func (a *ApproverData) delegateFor(delegators []string) []string {
	delegateFor := make([]string, 0)

	for approver := range a.Approvers {
		for _, delegator := range delegators {
			if delegator == approver && !isApproverDecisionExists(delegator, &a.ApproverLog) {
				delegateFor = append(delegateFor, delegator)
			}
		}
	}

	return delegateFor
}

func (a *ApproverData) getAdditionalApproversSlice() []string {
	result := make([]string, 0, len(a.AdditionalApprovers))

	for _, approver := range a.AdditionalApprovers {
		result = append(result, approver.ApproverLogin)
	}

	return result
}

//nolint:dupl //its not duplicate
func (a *ApproverData) setEditAppToInitiator(login, delegateFor string, params approverUpdateEditingParams) error {
	editing := &ApproverEditingApp{
		Approver:    login,
		Comment:     params.Comment,
		Attachments: params.Attachments,
		CreatedAt:   time.Now(),
		DelegateFor: delegateFor,
	}

	a.EditingAppLog = append(a.EditingAppLog, *editing)
	a.EditingApp = editing

	return nil
}

//nolint:dupl //its not duplicate
func (a *ApproverData) setEditToNextBlock(approver, delegateFor string, params approverUpdateEditingParams) error {
	sentToEdit := ApproverDecisionSentToEdit
	a.ActualApprover = &approver
	a.Decision = &sentToEdit
	a.Comment = &params.Comment
	a.DecisionAttachments = params.Attachments

	logEntry := ApproverLogEntry{
		Login:       approver,
		Decision:    sentToEdit,
		Comment:     params.Comment,
		Attachments: params.Attachments,
		CreatedAt:   time.Now(),
		LogType:     ApproverLogDecision,
		DelegateFor: delegateFor,
	}

	a.ApproverLog = append(a.ApproverLog, logEntry)

	return nil
}

// if exists empty link, then true, else false
func (a *ApproverData) checkEmptyLinkIDAddInfo() bool {
	for i := range a.AddInfo {
		if a.AddInfo[i].LinkID == nil {
			return true
		}
	}

	return false
}

func (a *ApproverData) latestUnansweredAddInfoLogEntry() *AdditionalInfo {
	qq := make(map[string]*AdditionalInfo)

	for i := range a.AddInfo {
		item := a.AddInfo[i]
		if item.Type == RequestAddInfoType {
			qq[item.ID] = &item
		}
	}

	for i := range a.AddInfo {
		item := a.AddInfo[i]
		if item.Type == ReplyAddInfoType && item.LinkID != nil {
			delete(qq, *item.LinkID)
		}
	}

	var latest *AdditionalInfo

	for _, q := range qq {
		if latest == nil || q.CreatedAt.After(latest.CreatedAt) {
			latest = q
		}
	}

	return latest
}

func (a *ApproverData) findAddInfoLogEntry(linkID string) *AdditionalInfo {
	for i := range a.AddInfo {
		item := a.AddInfo[i]
		if item.ID == linkID {
			return &item
		}
	}

	return nil
}

func (a *ApproverData) addInfoLogEntryHasResponse(linkID string) bool {
	for i := range a.AddInfo {
		item := a.AddInfo[i]
		if item.LinkID != nil && *item.LinkID == linkID {
			return true
		}
	}

	return false
}
