package pipeline

import (
	"fmt"
	"time"

	"github.com/google/uuid"
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

var additionalInfoTypeMap = map[AdditionalInfoType]struct{}{
	RequestAddInfoType: {},
	ReplyAddInfoType:   {},
}

type AdditionalInfo struct {
	Id          string             `json:"id"`
	Login       string             `json:"login"`
	Comment     string             `json:"comment"`
	Attachments []string           `json:"attachments"`
	LinkId      *string            `json:"link_id,omitempty"`
	Type        AdditionalInfoType `json:"type"`
	CreatedAt   time.Time          `json:"created_at"`
}

type ApprovementRule string

const (
	AllOfApprovementRequired ApprovementRule = "AllOf"
	AnyOfApprovementRequired ApprovementRule = "AnyOf"
)

type ApproverLogEntry struct {
	Login     string
	Decision  ApproverDecision
	Comment   string
	CreatedAt time.Time
}

type ApproverData struct {
	Type            script.ApproverType `json:"type"`
	Approvers       map[string]struct{} `json:"approvers"`
	Decision        *ApproverDecision   `json:"decision,omitempty"`
	Comment         *string             `json:"comment,omitempty"`
	ActualApprover  *string             `json:"actual_approver,omitempty"`
	ApprovementRule ApprovementRule     `json:"approvement_rule,omitempty"`
	ApproverLog     []ApproverLogEntry  `json:"approver_log,omitempty"`

	SLA        int                `json:"sla"`
	AutoAction *script.AutoAction `json:"auto_action,omitempty"`

	DidSLANotification bool `json:"did_sla_notification"`

	LeftToNotify map[string]struct{} `json:"left_to_notify"`

	IsEditable             bool                     `json:"is_editable"`
	RepeatPrevDecision     bool                     `json:"repeat_prev_decision"`
	EditingApp             *EditingApp              `json:"editing_app,omitempty"`
	EditingAppLog          []EditingApp             `json:"editing_app_log,omitempty"`
	RequestApproverInfoLog []RequestApproverInfoLog `json:"request_approver_info_log,omitempty"`

	FormsAccessibility []script.FormAccessibility `json:"forms_accessibility,omitempty"`

	ApproversGroupID   string `json:"approvers_group_id"`
	ApproversGroupName string `json:"approvers_group_name"`

	AddInfo []AdditionalInfo `json:"additional_info,omitempty"`
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

	if approvementRule == AnyOfApprovementRequired {
		a.Decision = &decision
		a.Comment = &comment
		a.ActualApprover = &login
	}

	if approvementRule == AllOfApprovementRequired {
		var approvedCount = 0
		var membersCount = len(a.Approvers)
		var overallDecision ApproverDecision

		for _, approver := range a.ApproverLog {
			if approver.Decision == ApproverDecisionApproved {
				approvedCount++
			}

			switch approver.Decision {
			case ApproverDecisionApproved:
				approvedCount++
			case ApproverDecisionRejected:
				overallDecision = ApproverDecisionRejected
			default:
				return fmt.Errorf("unknown decision %s", decision.String())
			}
		}

		if approvedCount == membersCount {
			overallDecision = ApproverDecisionApproved
		}

		a.Decision = &overallDecision
	}

	return nil
}

func (a *ApproverData) setEditApp(login string, params updateEditingParams) error {
	_, ok := a.Approvers[login]
	if !ok && login != AutoApprover {
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

func (a *ApproverData) setApproverRequestInfo(login string, params updateExecutorInfoParams) error {
	if params.Type == RequestAddInfoType {
		_, ok := a.Approvers[login]
		if !ok && login != AutoApprover {
			return fmt.Errorf("%s not found in approvers", login)
		}
	}

	if a.Decision != nil {
		return errors.New("decision already set")
	}

	if _, ok := additionalInfoTypeMap[params.Type]; !ok {
		return errors.New("incorrect type additional Info")
	}

	if len(a.AddInfo) == 0 && params.Type == ReplyAddInfoType {
		return errors.New("don't answer after request")
	}

	var (
		id     = uuid.NewString()
		linkId *string
	)

	if params.Type == ReplyAddInfoType {
		if params.LinkId == nil {
			return errors.New("linkId is null when reply")
		}

		linkId = params.LinkId
		err := setLinkIdRequest(id, *params.LinkId, a.AddInfo)
		if err != nil {
			return err
		}
	}

	a.AddInfo = append(a.AddInfo, AdditionalInfo{
		Id:          id,
		Type:        params.Type,
		Comment:     params.Comment,
		Attachments: params.Attachments,
		LinkId:      linkId,
		Login:       login,
		CreatedAt:   time.Now(),
	})

	return nil
}

func setLinkIdRequest(replyId, linkId string, addInfo []AdditionalInfo) error {
	for i := range addInfo {
		if addInfo[i].Id == linkId {
			addInfo[i].LinkId = &replyId
			return nil
		}
	}

	return errors.New("not found request by linkId")
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

func (a *ApproverData) SetRequestApproverInfo(login, comment string, reqType AdditionalInfoType, attach []string) error {
	_, ok := a.Approvers[login]
	if !ok && reqType == RequestAddInfoType {
		return fmt.Errorf("%s not found in approvers", login)
	}

	if reqType != ReplyAddInfoType && reqType != RequestAddInfoType {
		return fmt.Errorf("request info type is not valid")
	}

	a.RequestApproverInfoLog = append(a.RequestApproverInfoLog, RequestApproverInfoLog{
		Approver:    login,
		Comment:     comment,
		CreatedAt:   time.Now(),
		Type:        reqType,
		Attachments: attach,
	})

	return nil
}

func (a *ApproverData) IncreaseSLA(addSla int) {
	a.SLA += addSla
}
