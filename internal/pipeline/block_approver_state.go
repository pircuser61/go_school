package pipeline

import (
	"fmt"
	"time"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

type ApproverDecision string

func (a ApproverDecision) String() string {
	return string(a)
}

const (
	ApproverDecisionApproved ApproverDecision = "approved"
	ApproverDecisionRejected ApproverDecision = "rejected"
)

func decisionFromAutoAction(action script.AutoAction) ApproverDecision {
	if action == script.AutoActionApprove {
		return ApproverDecisionApproved
	}
	return ApproverDecisionRejected
}

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
	Login       string           `json:"login"`
	Decision    ApproverDecision `json:"decision"`
	Comment     string           `json:"comment"`
	CreatedAt   time.Time        `json:"created_at"`
	Attachments []string         `json:"attachments"`
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

	SLA        int                `json:"sla"`
	AutoAction *script.AutoAction `json:"auto_action,omitempty"`

	DidSLANotification bool `json:"did_sla_notification"`

	IsEditable             bool                     `json:"is_editable"`
	RepeatPrevDecision     bool                     `json:"repeat_prev_decision"`
	EditingApp             *ApproverEditingApp      `json:"editing_app,omitempty"`
	EditingAppLog          []ApproverEditingApp     `json:"editing_app_log,omitempty"`
	RequestApproverInfoLog []RequestApproverInfoLog `json:"request_approver_info_log,omitempty"`

	FormsAccessibility []script.FormAccessibility `json:"forms_accessibility,omitempty"`

	ApproversGroupID   string `json:"approvers_group_id"`
	ApproversGroupName string `json:"approvers_group_name"`

	AddInfo []AdditionalInfo `json:"additional_info,omitempty"`

	IsCanceled bool `json:"is_revoked"`
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

//nolint:gocyclo //its ok here
func (a *ApproverData) SetDecision(login string, decision ApproverDecision, comment string, attach []string) error {
	_, ok := a.Approvers[login]
	if !ok && login != AutoApprover {
		return fmt.Errorf("%s not found in approvers", login)
	}

	if decision != ApproverDecisionApproved && decision != ApproverDecisionRejected {
		return fmt.Errorf("unknown decision %s", decision.String())
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
			if entry.Login == login {
				return errors.New(fmt.Sprintf("decision of user %s is already set", login))
			}
		}

		var approverLogEntry = ApproverLogEntry{
			Login:       login,
			Decision:    decision,
			Comment:     comment,
			Attachments: attach,
			CreatedAt:   time.Now(),
		}

		a.ApproverLog = append(a.ApproverLog, approverLogEntry)

		var overallDecision ApproverDecision
		if decision == ApproverDecisionRejected {
			overallDecision = ApproverDecisionRejected
		}

		var approvedCount = 0
		for _, entry := range a.ApproverLog {
			if entry.Decision == ApproverDecisionApproved {
				approvedCount++
			}
		}

		if approvedCount == len(a.Approvers) {
			overallDecision = ApproverDecisionApproved
		}

		a.Decision = &overallDecision

		if overallDecision != ApproverDecisionRejected && overallDecision != ApproverDecisionApproved {
			a.Decision = nil
		}
	}

	return nil
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
