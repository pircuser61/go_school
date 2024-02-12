package pipeline

import (
	"fmt"
	"time"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
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
	Approver    string              `json:"approver"`
	Comment     string              `json:"comment"`
	Attachments []entity.Attachment `json:"attachments"`
	CreatedAt   time.Time           `json:"created_at"`
	DelegateFor string              `json:"delegate_for"`
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
	ID          string              `json:"id"`
	Login       string              `json:"login"`
	Comment     string              `json:"comment"`
	Attachments []entity.Attachment `json:"attachments"`
	LinkID      *string             `json:"link_id,omitempty"`
	Type        AdditionalInfoType  `json:"type"`
	CreatedAt   time.Time           `json:"created_at"`
	DelegateFor string              `json:"delegate_for"`
}

type ApproverLogEntry struct {
	Login          string              `json:"login"`
	Decision       ApproverDecision    `json:"decision"`
	Comment        string              `json:"comment"`
	CreatedAt      time.Time           `json:"created_at"`
	Attachments    []entity.Attachment `json:"attachments"`
	AddedApprovers []string            `json:"added_approvers"`
	LogType        ApproverLogType     `json:"log_type"`
	DelegateFor    string              `json:"delegate_for"`
}

type ApproverData struct {
	Type                script.ApproverType    `json:"type"`
	Approvers           map[string]struct{}    `json:"approvers"`
	Decision            *ApproverDecision      `json:"decision,omitempty"`
	DecisionAttachments []entity.Attachment    `json:"decision_attachments,omitempty"`
	Comment             *string                `json:"comment,omitempty"`
	ActualApprover      *string                `json:"actual_approver,omitempty"`
	ApprovementRule     script.ApprovementRule `json:"approvementRule,omitempty"`
	ApproverLog         []ApproverLogEntry     `json:"approver_log,omitempty"`

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

type Action struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title"`
}

type AdditionalApprover struct {
	ApproverLogin     string              `json:"approver_login"`
	BaseApproverLogin string              `json:"base_approver_login"`
	Question          *string             `json:"question"`
	Comment           *string             `json:"comment"`
	Attachments       []entity.Attachment `json:"attachments"`
	Decision          *ApproverDecision   `json:"decision"`
	CreatedAt         time.Time           `json:"created_at"`
	DecisionTime      *time.Time          `json:"decision_time"`
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

func (a *ApproverData) SetDecision(login, comment string, ds ApproverDecision, attach []entity.Attachment, d ht.Delegations) error {
	if ds == "" {
		return errors.New("missing decision")
	}

	if a.Decision != nil {
		return errors.New("decision already set")
	}

	delegators := d.GetDelegators(login)

	delegateFor := a.delegateFor(delegators)

	_, founded := a.Approvers[login]

	if !(founded || len(delegateFor) > 0) && login != AutoApprover {
		return NewUserIsNotPartOfProcessErr()
	}

	if a.ApprovementRule == script.AnyOfApprovementRequired {
		a.Decision = &ds
		a.Comment = &comment
		a.ActualApprover = &login
		a.DecisionAttachments = attach

		approverLogEntry := ApproverLogEntry{
			Login:       login,
			Decision:    ds,
			Comment:     comment,
			Attachments: attach,
			CreatedAt:   time.Now(),
			LogType:     ApproverLogDecision,
		}
		if len(delegateFor) > 0 && !founded {
			approverLogEntry.DelegateFor = delegateFor[0]
		}

		a.ApproverLog = append(a.ApproverLog, approverLogEntry)
	}

	//nolint:nestif //TODO: fix
	if a.ApprovementRule == script.AllOfApprovementRequired {
		if a.isUserDecisionSet(login) {
			return fmt.Errorf("decision of user %s is already set", login)
		}

		var (
			overallDecision ApproverDecision
			isFinal         bool
		)

		if login == AutoApprover {
			a.ApproverLog = append(
				a.ApproverLog,
				ApproverLogEntry{
					Login:       AutoApprover,
					Decision:    ds,
					Comment:     comment,
					Attachments: attach,
					CreatedAt:   time.Now(),
					LogType:     ApproverLogDecision,
				},
			)

			overallDecision = ds
			isFinal = true
		} else {
			if founded {
				a.ApproverLog = append(
					a.ApproverLog,
					ApproverLogEntry{
						Login:       login,
						Decision:    ds,
						Comment:     comment,
						Attachments: attach,
						CreatedAt:   time.Now(),
						LogType:     ApproverLogDecision,
					},
				)
			}

			for _, dl := range delegateFor {
				a.ApproverLog = append(a.ApproverLog, ApproverLogEntry{
					Login:       login,
					Decision:    ds,
					Comment:     comment,
					Attachments: attach,
					CreatedAt:   time.Now(),
					LogType:     ApproverLogDecision,
					DelegateFor: dl,
				})
			}

			overallDecision, isFinal = a.getFinalGroupDecision(ds)
		}

		if !isFinal {
			return nil
		}

		a.Decision = &overallDecision
		a.Comment = &comment
		a.ActualApprover = &login
		a.DecisionAttachments = []entity.Attachment{}

		//nolint:gocritic //в этом проекте не принято использовать поинтеры в коллекциях
		for _, l := range a.ApproverLog {
			if l.LogType == ApproverLogDecision {
				a.DecisionAttachments = append(a.DecisionAttachments, l.Attachments...)
			}
		}
	}

	return nil
}

func (a *ApproverData) delegateFor(delegators []string) []string {
	delegateFor := make([]string, 0)

	for approver := range a.Approvers {
		for _, delegator := range delegators {
			if delegator == approver && !decisionForPersonExists(delegator, &a.ApproverLog) {
				delegateFor = append(delegateFor, delegator)
			}
		}
	}

	return delegateFor
}

func (a *ApproverData) getFinalGroupDecision(ds ApproverDecision) (res ApproverDecision, isFinal bool) {
	if ds == ApproverDecisionRejected {
		return ApproverDecisionRejected, true
	}

	decisionsCount := 0
	decisions := make(map[ApproverDecision]int)

	for i := range a.ApproverLog {
		log := a.ApproverLog[i]
		if log.LogType != ApproverLogDecision {
			continue
		}
		decisionsCount++

		if log.Decision != ApproverDecisionRejected {
			count, decisionExists := decisions[log.Decision]
			if !decisionExists {
				count = 0
			}

			decisions[log.Decision] = count + 1
		}
	}

	if decisionsCount < len(a.Approvers) {
		return res, false
	}

	maxC := 0
	for k, v := range decisions {
		if v > maxC {
			maxC = v
			res = k
		}
	}

	return res, true
}

func (a *ApproverData) isUserDecisionSet(login string) bool {
	for i := range a.ApproverLog {
		if a.ApproverLog[i].Login == login && a.ApproverLog[i].LogType == ApproverLogDecision {
			return true
		}
	}

	return false
}

func decisionForPersonExists(login string, logs *[]ApproverLogEntry) bool {
	for i := 0; i < len(*logs); i++ {
		logEntry := (*logs)[i]
		if (logEntry.Login == login || logEntry.DelegateFor == login) && logEntry.LogType == ApproverLogDecision {
			return true
		}
	}

	return false
}

//nolint:gocyclo //its ok here
func (a *ApproverData) SetDecisionByAdditionalApprover(login string,
	params additionalApproverUpdateParams, delegations ht.Delegations,
) ([]string, error) {
	checkForAdditionalApprover := func(login string) bool {
		for _, approver := range a.AdditionalApprovers {
			if login == approver.ApproverLogin {
				return true
			}
		}

		return false
	}

	approverFound := checkForAdditionalApprover(login)

	delegateFor, isDelegate := delegations.FindDelegatorFor(login, a.getAdditionalApproversSlice())
	if !(approverFound || isDelegate) {
		return nil, NewUserIsNotPartOfProcessErr()
	}

	if a.Decision != nil {
		return nil, errors.New("decision already set")
	}

	loginsToNotify := make([]string, 0)
	couldUpdateOne := false
	timeNow := time.Now()

	for i := range a.AdditionalApprovers {
		var (
			additionalApprover              = a.AdditionalApprovers[i].ApproverLogin
			isDelegateForAdditionalApprover = delegations.IsLoginDelegateFor(login, additionalApprover)
		)

		if (login != additionalApprover && !isDelegateForAdditionalApprover) ||
			a.AdditionalApprovers[i].Decision != nil {
			continue
		}

		a.AdditionalApprovers[i].Decision = &params.Decision
		a.AdditionalApprovers[i].Comment = &params.Comment
		a.AdditionalApprovers[i].Attachments = params.Attachments

		if a.AdditionalApprovers[i].DecisionTime == nil {
			a.AdditionalApprovers[i].DecisionTime = &timeNow
		}

		if approverFound {
			delegateFor = ""
		}

		approverLogEntry := ApproverLogEntry{
			Login:       login,
			Decision:    params.Decision,
			Comment:     params.Comment,
			Attachments: params.Attachments,
			CreatedAt:   time.Now(),
			LogType:     AdditionalApproverLogDecision,
			DelegateFor: delegateFor,
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
