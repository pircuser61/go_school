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

type EditingApp struct {
	Approver    string    `json:"approver"`
	Comment     string    `json:"comment"`
	Attachments []string  `json:"attachments"`
	CreatedAt   time.Time `json:"created_at"`
}

type RequestAddInfo struct {
	Initiator *AdditionalInfo `json:"initiator"`
	Approver  *AdditionalInfo `json:"approver"`
}

type AdditionalInfo struct {
	Login       string    `json:"login"`
	Comment     string    `json:"comment"`
	Attachments []string  `json:"attachments"`
	CreatedAt   time.Time `json:"created_at"`
}

type ApproverData struct {
	Type           script.ApproverType `json:"type"`
	Approvers      map[string]struct{} `json:"approvers"`
	Decision       *ApproverDecision   `json:"decision,omitempty"`
	Comment        *string             `json:"comment,omitempty"`
	ActualApprover *string             `json:"actual_approver,omitempty"`

	SLA        int                `json:"sla"`
	AutoAction *script.AutoAction `json:"auto_action,omitempty"`

	DidSLANotification bool `json:"did_sla_notification"`

	LeftToNotify map[string]struct{} `json:"left_to_notify"`

	IsEditable         bool         `json:"is_editable"`
	RepeatPrevDecision bool         `json:"repeat_prev_decision"`
	EditingApp         *EditingApp  `json:"editing_app,omitempty"`
	EditingAppLog      []EditingApp `json:"editing_app_log,omitempty"`

	ApproversGroupID   string `json:"approvers_group_id"`
	ApproversGroupName string `json:"approvers_group_name"`

	RequestAddInfo    *RequestAddInfo  `json:"request_additional_info,omitempty"`
	RequestAddInfoLog []RequestAddInfo `json:"request_additional_info_log,omitempty"`
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

func (a *ApproverData) SetDecision(login string, decision ApproverDecision, comment string) error {
	_, ok := a.Approvers[login]
	if !ok {
		return fmt.Errorf("%s not found in approvers", login)
	}

	if a.Decision != nil {
		return errors.New("decision already set")
	}

	if decision != ApproverDecisionApproved && decision != ApproverDecisionRejected {
		return fmt.Errorf("unknown decision %s", decision.String())
	}

	a.Decision = &decision
	a.Comment = &comment
	a.ActualApprover = &login

	return nil
}

func (a *ApproverData) setEditApp(login string, params updateEditingParams) error {
	_, ok := a.Approvers[login]
	if !ok {
		return fmt.Errorf("%s not found in approvers", login)
	}

	if a.Decision != nil {
		return errors.New("decision already set")
	}

	editing := &EditingApp{
		Approver:    login,
		Comment:     params.Comment,
		Attachments: params.Attachments,
		CreatedAt:   time.Now(),
	}

	a.EditingAppLog = append(a.EditingAppLog, *editing)

	a.EditingApp = editing

	return nil
}

func (a *ApproverData) setRequestAddInfo(login string, params updateAddInfoParams) error {
	_, ok := a.Approvers[login]
	if !ok {
		return fmt.Errorf("%s not found in approvers", login)
	}

	if a.Decision != nil {
		return errors.New("decision already set")
	}

	if a.RequestAddInfo != nil {
		if a.RequestAddInfo.Initiator != nil && a.RequestAddInfo.Approver != nil {
			a.RequestAddInfoLog = append(a.RequestAddInfoLog, *a.RequestAddInfo)
			addInfo := createAddInfo(login, params)
			a.RequestAddInfo = addInfo
		}
		if a.RequestAddInfo.Approver != nil {
			addInfo := createAddInfo(login, params)
			a.RequestAddInfo.Initiator = addInfo.Initiator
		}
		return nil
	}

	addIfno := createAddInfo(login, params)
	a.RequestAddInfo = addIfno

	return nil
}

func createAddInfo(login string, params updateAddInfoParams) *RequestAddInfo {
	addInfo := AdditionalInfo{
		Login:       login,
		Comment:     params.Comment,
		Attachments: params.Attachments,
		CreatedAt:   time.Now(),
	}

	var addInfoApplication RequestAddInfo

	if params.Author == "initiator" {
		addInfoApplication.Initiator = &addInfo
	}
	if params.Author == "approver" {
		addInfoApplication.Approver = &addInfo
	}

	return &addInfoApplication
}
