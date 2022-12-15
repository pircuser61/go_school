package pipeline

import (
	"fmt"
	"time"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

type ApproverAction string

const (
	ApproverActionApprove  = "approve"
	ApproverActionReject   = "reject"
	ApproverActionViewed   = "viewed"
	ApproverActionInformed = "informed"
	ApproverActionSign     = "sign"
	ApproverActionConfirm  = "confirm"

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
	case ApproverDecisionConfirmed:
		return ApproverActionConfirm
	default:
		return ""
	}
}

func (a ApproverDecision) ToRuString() string {
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
	ApproverDecisionApproved  ApproverDecision = "approved"
	ApproverDecisionRejected  ApproverDecision = "rejected"
	ApproverDecisionViewed    ApproverDecision = "viewed"
	ApproverDecisionInformed  ApproverDecision = "informed"
	ApproverDecisionSigned    ApproverDecision = "signed"
	ApproverDecisionConfirmed ApproverDecision = "confirmed"
)

type ApproverEditingApp struct {
	Approver    string    `json:"approver"`
	Comment     string    `json:"comment"`
	Attachments []string  `json:"attachments"`
	CreatedAt   time.Time `json:"created_at"`
}

type RequestApproverInfoLog struct {
	Approver    string             `json:"approver"`
	Comment     string             `json:"comment"`
	Attachments []string           `json:"attachments"`
	Type        AdditionalInfoType `json:"type"`
	CreatedAt   time.Time          `json:"created_at"`
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
	Id          string             `json:"id"`
	Login       string             `json:"login"`
	Comment     string             `json:"comment"`
	Attachments []string           `json:"attachments"`
	LinkId      *string            `json:"link_id,omitempty"`
	Type        AdditionalInfoType `json:"type"`
	CreatedAt   time.Time          `json:"created_at"`
}

type ApproverLogEntry struct {
	Login          string           `json:"login"`
	Decision       ApproverDecision `json:"decision"`
	Comment        string           `json:"comment"`
	CreatedAt      time.Time        `json:"created_at"`
	Attachments    []string         `json:"attachments"`
	AddedApprovers []string         `json:"added_approvers"`
	LogType        ApproverLogType  `json:"log_type"`
}

type ApproverData struct {
	Type                script.ApproverType    `json:"type"`
	Approvers           map[string]struct{}    `json:"approvers"`
	Decision            *ApproverDecision      `json:"decision,omitempty"`
	DecisionAttachments []string               `json:"decision_attachments,omitempty"`
	Comment             *string                `json:"comment,omitempty"`
	ActualApprover      *string                `json:"actual_approver,omitempty"`
	ApprovementRule     script.ApprovementRule `json:"approvementRule,omitempty"`
	ApproverLog         []ApproverLogEntry     `json:"approver_log,omitempty"`

	SLA        int             `json:"sla"`
	AutoAction *ApproverAction `json:"auto_action,omitempty"`

	IsEditable             bool                     `json:"is_editable"`
	RepeatPrevDecision     bool                     `json:"repeat_prev_decision"`
	EditingApp             *ApproverEditingApp      `json:"editing_app,omitempty"`
	EditingAppLog          []ApproverEditingApp     `json:"editing_app_log,omitempty"`
	RequestApproverInfoLog []RequestApproverInfoLog `json:"request_approver_info_log,omitempty"`

	FormsAccessibility []script.FormAccessibility `json:"forms_accessibility,omitempty"`

	ApproversGroupID   string `json:"approvers_group_id"`
	ApproversGroupName string `json:"approvers_group_name"`

	AddInfo []AdditionalInfo `json:"additional_info,omitempty"`

	IsRevoked         bool   `json:"is_revoked"`
	ApproveStatusName string `json:"approve_status_name"`

	SLAChecked     bool `json:"sla_checked"`
	HalfSLAChecked bool `json:"half_sla_checked"`

	ActionList []Action `json:"action_list"`

	AdditionalApprovers []AdditionalApprover `json:"additional_approvers"`
}

type Action struct {
	Id    string `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title"`
}

type AdditionalApprover struct {
	ApproverLogin     string            `json:"approver_login"`
	BaseApproverLogin string            `json:"base_approver_login"`
	Question          *string           `json:"question"`
	Comment           *string           `json:"comment"`
	Attachments       []string          `json:"attachments"`
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

//nolint:gocyclo //its ok here
func (a *ApproverData) SetDecision(login string, decision ApproverDecision, comment string, attach []string) error {
	_, ok := a.Approvers[login]
	if !ok && login != AutoApprover {
		return fmt.Errorf("%s not found in approvers", login)
	}

	if decision == "" {
		return errors.New("missing decision")
	}

	if a.Decision != nil {
		return errors.New("decision already set")
	}

	var approvementRule = a.ApprovementRule

	if approvementRule == script.AnyOfApprovementRequired {
		a.Decision = &decision
		a.Comment = &comment
		a.ActualApprover = &login
		a.DecisionAttachments = attach
	}

	if approvementRule == script.AllOfApprovementRequired {
		for _, entry := range a.ApproverLog {
			if entry.Login == login && entry.LogType == ApproverLogDecision {
				return errors.New(fmt.Sprintf("decision of user %s is already set", login))
			}
		}

		var approverLogEntry = ApproverLogEntry{
			Login:       login,
			Decision:    decision,
			Comment:     comment,
			Attachments: attach,
			CreatedAt:   time.Now(),
			LogType:     ApproverLogDecision,
		}

		a.ApproverLog = append(a.ApproverLog, approverLogEntry)

		var overallDecision ApproverDecision
		if decision == ApproverDecisionRejected {
			overallDecision = ApproverDecisionRejected
		} else {
			decisions := make(map[ApproverDecision]int)
			decisionsCount := 0
			for _, entry := range a.ApproverLog {
				if entry.LogType != ApproverLogDecision {
					continue
				}
				decisionsCount += 1
				if entry.Decision != ApproverDecisionRejected {
					count, decisionExists := decisions[entry.Decision]
					if !decisionExists {
						count = 0
					}
					decisions[entry.Decision] = count + 1
				}
			}

			if decisionsCount < len(a.Approvers) {
				return nil
			}

			maxC := 0
			for k, v := range decisions {
				if v > maxC {
					maxC = v
					overallDecision = k
				}
			}
		}

		a.Decision = &overallDecision
	}

	return nil
}

//nolint:gocyclo //its ok here
func (a *ApproverData) SetDecisionByAdditionalApprover(login string, params additionalApproverUpdateParams) ([]string, error) {
	if a.Decision != nil {
		return nil, errors.New("decision already set")
	}

	loginsToNotify := make([]string, 0)
	couldUpdateOne := false
	timeNow := time.Now()

	for i := range a.AdditionalApprovers {
		if login != a.AdditionalApprovers[i].ApproverLogin || a.AdditionalApprovers[i].Decision != nil {
			continue
		}

		a.AdditionalApprovers[i].Decision = &params.Decision
		a.AdditionalApprovers[i].Comment = &params.Comment
		a.AdditionalApprovers[i].Attachments = params.Attachments
		if a.AdditionalApprovers[i].DecisionTime == nil {
			a.AdditionalApprovers[i].DecisionTime = &timeNow
		}

		var approverLogEntry = ApproverLogEntry{
			Login:       login,
			Decision:    params.Decision,
			Comment:     params.Comment,
			Attachments: params.Attachments,
			CreatedAt:   time.Now(),
			LogType:     AdditionalApproverLogDecision,
		}

		a.ApproverLog = append(a.ApproverLog, approverLogEntry)
		loginsToNotify = append(loginsToNotify, a.AdditionalApprovers[i].BaseApproverLogin)
		couldUpdateOne = true
	}

	if !couldUpdateOne {
		return nil, fmt.Errorf("can't approve any request")
	}

	return loginsToNotify, nil
}

func (a *ApproverData) setEditApp(login string, params approverUpdateEditingParams) error {
	_, ok := a.Approvers[login]
	if !ok && login != AutoApprover {
		return fmt.Errorf("%s not found in approvers", login)
	}

	if a.Decision != nil {
		return errors.New("decision already set")
	}

	editing := &ApproverEditingApp{
		Approver:    login,
		Comment:     params.Comment,
		Attachments: params.Attachments,
		CreatedAt:   time.Now(),
	}

	a.EditingAppLog = append(a.EditingAppLog, *editing)

	a.EditingApp = editing

	return nil
}

// if exists empty link, then true, else false
func (a *ApproverData) checkEmptyLinkIdAddInfo() bool {
	for i := range a.AddInfo {
		if a.AddInfo[i].LinkId == nil {
			return true
		}
	}

	return false
}

func (a *ApproverData) IncreaseSLA(addSla int) {
	a.SLA += addSla
}
