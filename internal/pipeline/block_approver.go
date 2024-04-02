package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sla"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

const (
	AutoActionComment = "Выполнено автоматическое действие по истечению SLA"
	AutoApprover      = "auto_approve"

	keyOutputApprover = "approver"
	keyOutputDecision = "decision"
	keyOutputComment  = "comment"

	approverAddApproversAction          = "add_approvers"
	approverSendEditAppAction           = "approver_send_edit_app"
	approverRequestAddInfoAction        = "request_add_info"
	approverAdditionalApprovementAction = "additional_approvement"
	approverAdditionalRejectAction      = "additional_reject"

	readWriteAccessType    = "ReadWrite"
	requiredFillAccessType = "RequiredFill"
)

type GoApproverBlock struct {
	Name      string
	ShortName string
	Title     string
	Input     map[string]string
	Output    map[string]string
	Sockets   []script.Socket
	State     *ApproverData

	expectedEvents      map[string]struct{}
	happenedEvents      []entity.NodeEvent
	happenedKafkaEvents []entity.NodeKafkaEvent

	RunContext *BlockRunContext
}

func (gb *GoApproverBlock) CurrentExecutorData() CurrentExecutorData {
	return CurrentExecutorData{}
}

func (gb *GoApproverBlock) GetNewEvents() []entity.NodeEvent {
	return gb.happenedEvents
}

func (gb *GoApproverBlock) GetNewKafkaEvents() []entity.NodeKafkaEvent {
	return gb.happenedKafkaEvents
}

func (gb *GoApproverBlock) Members() []Member {
	capacity := len(gb.State.Approvers) + len(gb.State.AdditionalApprovers)
	members := make([]Member, 0, capacity)
	addedMembers := make(map[string]struct{}, capacity)

	for login := range gb.State.Approvers {
		members = append(
			members,
			Member{
				Login:                login,
				Actions:              gb.approvementBaseActions(login),
				IsActed:              gb.isApprovementActed(login),
				ExecutionGroupMember: false,
				IsInitiator:          false,
			},
		)
		addedMembers[login] = struct{}{}
	}

	for i := 0; i < len(gb.State.AdditionalApprovers); i++ {
		addApprover := gb.State.AdditionalApprovers[i]

		members = append(
			members,
			Member{
				Login:                addApprover.ApproverLogin,
				Actions:              gb.approvementAddActions(&addApprover),
				IsActed:              gb.isApprovementActed(addApprover.ApproverLogin),
				ExecutionGroupMember: false,
				IsInitiator:          false,
			},
		)
		addedMembers[addApprover.ApproverLogin] = struct{}{}
	}

	for i := 0; i < len(gb.State.ApproverLog); i++ {
		log := gb.State.ApproverLog[i]
		if _, ok := addedMembers[log.Login]; ok {
			continue
		}

		members = append(members, Member{
			Login:                log.Login,
			Actions:              []MemberAction{},
			IsActed:              true,
			ExecutionGroupMember: false,
			IsInitiator:          false,
		})

		addedMembers[log.Login] = struct{}{}
	}

	for i := 0; i < len(gb.State.EditingAppLog); i++ {
		log := gb.State.EditingAppLog[i]
		if _, ok := addedMembers[log.Approver]; ok {
			continue
		}

		members = append(members, Member{
			Login:                log.Approver,
			Actions:              []MemberAction{},
			IsActed:              true,
			ExecutionGroupMember: false,
			IsInitiator:          false,
		})

		addedMembers[log.Approver] = struct{}{}
	}

	if gb.State.EditingApp != nil {
		members = append(members, Member{
			Login: gb.RunContext.Initiator,
			Actions: []MemberAction{
				{
					ID:     string(entity.TaskUpdateActionEditApp),
					Type:   ActionTypeCustom,
					Params: map[string]interface{}{},
				},
			},
			IsActed:              false,
			ExecutionGroupMember: false,
			IsInitiator:          true,
		})
	}

	for i := 0; i < len(gb.State.AddInfo); i++ {
		log := gb.State.AddInfo[i]
		if _, ok := addedMembers[log.Login]; ok {
			continue
		}

		if log.Type == RequestAddInfoType {
			members = append(members, Member{
				Login:                log.Login,
				Actions:              []MemberAction{},
				IsActed:              true,
				ExecutionGroupMember: false,
				IsInitiator:          false,
			})

			addedMembers[log.Login] = struct{}{}

			if !isQuestionAnswered(log.LinkID, gb.State.AddInfo) {
				members = append(members, Member{
					Login: gb.RunContext.Initiator,
					Actions: []MemberAction{
						{
							ID:   string(entity.TaskUpdateActionReplyApproverInfo),
							Type: ActionTypeCustom,
							Params: map[string]interface{}{
								"link_id": log.LinkID,
							},
						},
					},
					IsActed:              false,
					ExecutionGroupMember: false,
					IsInitiator:          true,
				})
			}
		}
	}

	return members
}

func isQuestionAnswered(questionLinkID *string, logReply []AdditionalInfo) bool {
	for i := range logReply {
		if logReply[i].Type == ReplyAddInfoType && logReply[i].LinkID == questionLinkID {
			return true
		}
	}

	return false
}

func (gb *GoApproverBlock) isApprovementActed(login string) bool {
	for i := 0; i < len(gb.State.ApproverLog); i++ {
		log := gb.State.ApproverLog[i]
		if log.Login == login || log.DelegateFor == login {
			return true
		}
	}

	for i := 0; i < len(gb.State.EditingAppLog); i++ {
		log := gb.State.EditingAppLog[i]
		if log.Approver == login || log.DelegateFor == login {
			return true
		}
	}

	for i := 0; i < len(gb.State.AddInfo); i++ {
		log := gb.State.AddInfo[i]
		if (log.Login == login || log.DelegateFor == login) && log.Type == RequestAddInfoType {
			return true
		}
	}

	return false
}

func (gb *GoApproverBlock) approvementBaseActions(login string) []MemberAction {
	if gb.State.Decision != nil || gb.State.EditingApp != nil {
		return []MemberAction{}
	}

	for i := 0; i < len(gb.State.ApproverLog); i++ {
		log := gb.State.ApproverLog[i]
		if (log.Login == login || log.DelegateFor == login) && log.LogType == ApproverLogDecision {
			return []MemberAction{}
		}
	}

	actions := make([]MemberAction, 0, len(gb.State.ActionList))

	for i := range gb.State.ActionList {
		actions = append(actions, MemberAction{
			ID:   gb.State.ActionList[i].ID,
			Type: gb.State.ActionList[i].Type,
		})
	}

	fillFormNames, existEmptyForm := gb.getFormNamesToFill()
	if existEmptyForm {
		for i := 0; i < len(actions); i++ {
			item := &actions[i]

			if item.ID == ApproverActionReject {
				continue
			}

			item.Params = map[string]interface{}{"disabled": true}
		}
	}

	if len(fillFormNames) != 0 {
		actions = append(actions, MemberAction{
			ID:   formFillFormAction,
			Type: ActionTypeCustom,
			Params: map[string]interface{}{
				formName: fillFormNames,
			},
		})
	}

	return append(actions, MemberAction{
		ID:   approverAddApproversAction,
		Type: ActionTypeOther,
	}, MemberAction{
		ID:   approverRequestAddInfoAction,
		Type: ActionTypeOther,
	})
}

func (gb *GoApproverBlock) approvementAddActions(a *AdditionalApprover) []MemberAction {
	if gb.State.Decision != nil || a.Decision != nil || gb.State.EditingApp != nil {
		return []MemberAction{}
	}

	return []MemberAction{
		{
			ID:   approverAdditionalApprovementAction,
			Type: ActionTypePrimary,
		},
		{
			ID:   approverAdditionalRejectAction,
			Type: ActionTypeSecondary,
		},
		{
			ID:   approverAddApproversAction,
			Type: ActionTypeOther,
		},
		{
			ID:   approverRequestAddInfoAction,
			Type: ActionTypeOther,
		},
	}
}

type qna struct {
	qCrAt time.Time
	aCrAt *time.Time
}

//nolint:dupl //its not duplicate
func (gb *GoApproverBlock) getFormNamesToFill() ([]string, bool) {
	var (
		actions   = make([]string, 0)
		emptyForm = false
		l         = logger.GetLogger(context.Background())
	)

	for _, form := range gb.State.FormsAccessibility {
		formState, ok := gb.RunContext.VarStore.State[form.NodeID]
		if !ok {
			continue
		}

		switch form.AccessType {
		case readWriteAccessType:
			actions = append(actions, form.NodeID)
		case requiredFillAccessType:
			actions = append(actions, form.NodeID)

			existEmptyForm := gb.checkForEmptyForm(formState, l)
			if existEmptyForm {
				emptyForm = true
			}
		}
	}

	return actions, emptyForm
}

func (gb *GoApproverBlock) checkForEmptyForm(formState json.RawMessage, l logger.Logger) bool {
	var formData FormData
	if err := json.Unmarshal(formState, &formData); err != nil {
		l.Error(err)

		return true
	}

	if !formData.IsFilled {
		return true
	}

	for _, v := range formData.ChangesLog {
		if _, findOk := gb.State.Approvers[v.Executor]; findOk {
			return false
		}
	}

	return true
}

func (gb *GoApproverBlock) getDeadline(ctx context.Context, workType string) (time.Time, error) {
	slaInfoPtr, getSLAInfoErr := gb.RunContext.Services.SLAService.GetSLAInfoPtr(ctx, sla.InfoDTO{
		TaskCompletionIntervals: []entity.TaskCompletionInterval{{
			StartedAt:  gb.RunContext.CurrBlockStartTime,
			FinishedAt: gb.RunContext.CurrBlockStartTime.Add(time.Hour * 24 * 100),
		}},
		WorkType: sla.WorkHourType(workType),
	})
	if getSLAInfoErr != nil {
		return time.Time{}, errors.Wrap(getSLAInfoErr, "can not get slaInfo")
	}

	return gb.getNewSLADeadline(slaInfoPtr, false), nil
}

func (gb *GoApproverBlock) getNewSLADeadline(slaInfoPtr *sla.Info, half bool) time.Time {
	qq := make(map[string]qna)

	for i := range gb.State.AddInfo {
		item := gb.State.AddInfo[i]
		if item.Type == RequestAddInfoType {
			qq[item.ID] = qna{qCrAt: item.CreatedAt}
		}
	}

	for i := range gb.State.AddInfo {
		item := gb.State.AddInfo[i]
		if item.Type == ReplyAddInfoType && item.LinkID != nil {
			data, ok := qq[*item.LinkID]
			if !ok {
				continue
			}

			data.aCrAt = &item.CreatedAt

			qq[*item.LinkID] = data
		}
	}

	newSLA := gb.State.SLA
	if half {
		newSLA /= 2
	}

	deadline := gb.RunContext.Services.SLAService.ComputeMaxDate(gb.RunContext.CurrBlockStartTime, float32(newSLA), slaInfoPtr)

	for _, q := range qq {
		if q.aCrAt == nil {
			continue
		}

		additionalHours := gb.RunContext.Services.SLAService.GetWorkHoursBetweenDates(q.qCrAt, *q.aCrAt, nil)
		deadline = gb.RunContext.Services.SLAService.ComputeMaxDate(deadline, float32(additionalHours), nil)
	}

	return deadline
}

//nolint:dupl,gocyclo //Need here
func (gb *GoApproverBlock) Deadlines(ctx context.Context) ([]Deadline, error) {
	deadlines := make([]Deadline, 0, 2)

	latestUnansweredRequest := gb.State.latestUnansweredAddInfoLogEntry()

	if gb.State.Decision != nil && latestUnansweredRequest != nil {
		if gb.State.CheckDayBeforeSLARequestInfo {
			deadlines = append(deadlines, Deadline{
				Deadline: gb.RunContext.Services.SLAService.ComputeMaxDate(
					latestUnansweredRequest.CreatedAt, 2*workingHours, nil),
				Action: entity.TaskUpdateActionDayBeforeSLARequestAddInfo,
			})
		}

		deadlines = append(deadlines, Deadline{
			Deadline: gb.RunContext.Services.SLAService.ComputeMaxDate(
				latestUnansweredRequest.CreatedAt, 3*workingHours, nil),
			Action: entity.TaskUpdateActionSLABreachRequestAddInfo,
		})

		return deadlines, nil
	}

	if gb.State.Decision != nil {
		return []Deadline{}, nil
	}

	if gb.State.CheckSLA && latestUnansweredRequest == nil {
		slaInfoPtr, getSLAInfoErr := gb.RunContext.Services.SLAService.GetSLAInfoPtr(ctx, sla.InfoDTO{
			TaskCompletionIntervals: []entity.TaskCompletionInterval{{
				StartedAt:  gb.RunContext.CurrBlockStartTime,
				FinishedAt: gb.RunContext.CurrBlockStartTime.Add(time.Hour * 24 * 100),
			}},
			WorkType: sla.WorkHourType(gb.State.WorkType),
		})

		if getSLAInfoErr != nil {
			return nil, getSLAInfoErr
		}

		if !gb.State.SLAChecked {
			deadlines = append(deadlines, Deadline{
				Deadline: gb.getNewSLADeadline(slaInfoPtr, false),
				Action:   entity.TaskUpdateActionSLABreach,
			},
			)
		}

		if !gb.State.HalfSLAChecked {
			deadlines = append(deadlines, Deadline{
				Deadline: gb.getNewSLADeadline(slaInfoPtr, true),
				Action:   entity.TaskUpdateActionHalfSLABreach,
			},
			)
		}
	}

	if gb.State.IsEditable && gb.State.CheckReworkSLA && gb.State.EditingApp != nil {
		deadlines = append(deadlines,
			Deadline{
				Deadline: gb.RunContext.Services.SLAService.ComputeMaxDate(
					gb.State.EditingApp.CreatedAt, float32(gb.State.ReworkSLA), nil),
				Action: entity.TaskUpdateActionReworkSLABreach,
			},
		)
	}

	if latestUnansweredRequest != nil {
		if gb.State.CheckDayBeforeSLARequestInfo {
			deadlines = append(deadlines, Deadline{
				Deadline: gb.RunContext.Services.SLAService.ComputeMaxDate(
					latestUnansweredRequest.CreatedAt, 2*workingHours, nil),
				Action: entity.TaskUpdateActionDayBeforeSLARequestAddInfo,
			})
		}

		deadlines = append(deadlines, Deadline{
			Deadline: gb.RunContext.Services.SLAService.ComputeMaxDate(
				latestUnansweredRequest.CreatedAt, 3*workingHours, nil),
			Action: entity.TaskUpdateActionSLABreachRequestAddInfo,
		})
	}

	return deadlines, nil
}

func (gb *GoApproverBlock) UpdateManual() bool {
	return true
}

func (gb *GoApproverBlock) GetStatus() Status {
	if gb.State != nil && gb.State.Decision != nil {
		if *gb.State.Decision == ApproverDecisionRejected {
			return StatusNoSuccess
		}

		if *gb.State.Decision == ApproverDecisionSentToEdit {
			return StatusNoSuccess
		}

		return StatusFinished
	}

	if gb.State.EditingApp != nil {
		return StatusIdle
	}

	if len(gb.State.AddInfo) != 0 {
		if gb.State.checkEmptyLinkIDAddInfo() {
			return StatusIdle
		}
	}

	return StatusRunning
}

func (gb *GoApproverBlock) GetTaskHumanStatus() (status TaskHumanStatus, comment, action string) {
	if gb.State != nil && gb.State.EditingApp != nil {
		return StatusWait, "", ""
	}

	if gb.State != nil && gb.State.Decision != nil {
		if *gb.State.Decision == ApproverDecisionRejected {
			return StatusApprovementRejected, "", ""
		}

		if *gb.State.Decision == ApproverDecisionSentToEdit {
			//nolint:goconst //не хочу внедрять миллион констант под каждую строку в проекте
			return StatusApprovementRejected, "", "отправлена на доработку"
		}

		return getPositiveFinishStatus(*gb.State.Decision), "", ""
	}

	if gb.State != nil && len(gb.State.AddInfo) != 0 {
		if gb.State.checkEmptyLinkIDAddInfo() {
			return StatusWait, "", ""
		}

		return getPositiveProcessingStatus(gb.State.ApproveStatusName), "", ""
	}

	lastIdx := len(gb.State.AddInfo) - 1
	if len(gb.State.AddInfo) > 0 && gb.State.AddInfo[lastIdx].Type == RequestAddInfoType {
		return StatusWait, "", ""
	}

	return getPositiveProcessingStatus(gb.State.ApproveStatusName), "", ""
}

func (gb *GoApproverBlock) Next(_ *store.VariableStore) ([]string, bool) {
	var key string
	if gb.State != nil && gb.State.Decision != nil {
		key = string(gb.State.Decision.ToAction())
	}

	if gb.State != nil && gb.State.Decision == nil && gb.State.EditingApp != nil {
		return nil, false
	}

	if gb.State != nil && gb.State.Decision == nil && len(gb.State.AddInfo) != 0 {
		key = requestAddInfoSocketID
	}

	nexts, ok := script.GetNexts(gb.Sockets, key)
	if !ok {
		return nil, false
	}

	return nexts, true
}

func (gb *GoApproverBlock) GetState() interface{} {
	return gb.State
}

func (gb *GoApproverBlock) Model() script.FunctionModel {
	return script.FunctionModel{
		ID:        BlockGoApproverID,
		BlockType: script.TypeGo,
		Title:     gb.Title,
		Inputs:    nil,
		Outputs: &script.JSONSchema{
			Type: "object",
			Properties: script.JSONSchemaProperties{
				keyOutputApprover: {
					Type:        "object",
					Description: "approver login which made a decision",
					Format:      "SsoPerson",
					Properties:  people.GetSsoPersonSchemaProperties(),
				},
				keyOutputDecision: {
					Type:        "string",
					Description: "block decision",
				},
				keyOutputComment: {
					Type:        "string",
					Description: "approver comment",
				},
			},
		},
		Params: &script.FunctionParams{
			Type: BlockGoApproverID,
			Params: &script.ApproverParams{
				Approver:           "",
				Type:               "",
				SLA:                0,
				IsEditable:         false,
				RepeatPrevDecision: false,
				ApproversGroupID:   "",
				ApproversGroupName: "",
				FormsAccessibility: []script.FormAccessibility{},
			},
		},
		Sockets: []script.Socket{
			script.ApproveSocket,
			script.RejectSocket,
		},
	}
}

func (gb *GoApproverBlock) BlockAttachments() (ids []string) {
	ids = make([]string, 0)

	for i := range gb.State.AddInfo {
		for j := range gb.State.AddInfo[i].Attachments {
			ids = append(ids, gb.State.AddInfo[i].Attachments[j].FileID)
		}
	}

	for i := range gb.State.ApproverLog {
		for j := range gb.State.ApproverLog[i].Attachments {
			ids = append(ids, gb.State.ApproverLog[i].Attachments[j].FileID)
		}
	}

	for i := range gb.State.EditingAppLog {
		for j := range gb.State.EditingAppLog[i].Attachments {
			ids = append(ids, gb.State.EditingAppLog[i].Attachments[j].FileID)
		}
	}

	return utils.UniqueStrings(ids)
}

func getPositiveProcessingStatus(decision string) (status TaskHumanStatus) {
	switch decision {
	case script.SettingStatusApprovement:
		return StatusApprovement
	case script.SettingStatusApproveConfirm:
		return StatusApproveConfirm
	case script.SettingStatusApproveView:
		return StatusApproveView
	case script.SettingStatusApproveInform:
		return StatusApproveInform
	case script.SettingStatusApproveSign:
		return StatusSigning
	case script.SettingStatusApproveSignUkep:
		return StatusApproveSignUkep
	default:
		return StatusApprovement
	}
}

func getPositiveFinishStatus(decision ApproverDecision) (status TaskHumanStatus) {
	// nolint:exhaustive //не хотим обрабатывать остальные случаи
	switch decision {
	case ApproverDecisionApproved:
		return StatusApproved
	case ApproverDecisionViewed:
		return StatusApproveViewed
	case ApproverDecisionInformed:
		return StatusApproveInformed
	case ApproverDecisionSigned, ApproverDecisionSignedUkep:
		return StatusSigned
	case ApproverDecisionConfirmed:
		return StatusApproveConfirmed
	default:
		return StatusApproved
	}
}

type ApproverOutput struct {
	Approver *servicedesc.SsoPerson
	Comment  *string
	Decision *ApproverDecision
}

func (gb *GoApproverBlock) UpdateStateUsingOutput(ctx context.Context, data []byte) (state map[string]interface{}, err error) {
	approverOutput := ApproverOutput{}

	unmErr := json.Unmarshal(data, &approverOutput)
	if unmErr != nil {
		return nil, fmt.Errorf("can't unmarshal into output struct")
	}

	if approverOutput.Decision != nil {
		gb.State.Decision = approverOutput.Decision
	}

	if approverOutput.Comment != nil {
		gb.State.Comment = approverOutput.Comment
	}

	if approverOutput.Approver != nil {
		gb.State.ActualApprover = &approverOutput.Approver.Username
	}

	jsonState, marshErr := json.Marshal(gb.State)
	if marshErr != nil {
		return nil, marshErr
	}

	unmarshErr := json.Unmarshal(jsonState, &state)
	if unmarshErr != nil {
		return nil, unmarshErr
	}

	return state, nil
}

func (gb *GoApproverBlock) UpdateOutputUsingState(ctx context.Context) (res map[string]interface{}, err error) {
	output := map[string]interface{}{}

	if gb.State.ActualApprover != nil {
		personData, ssoErr := gb.RunContext.Services.ServiceDesc.GetSsoPerson(ctx, *gb.State.ActualApprover)
		if ssoErr != nil {
			return nil, ssoErr
		}

		output[keyOutputApprover] = personData
	}

	if gb.State.Decision != nil {
		output[keyOutputDecision] = gb.State.Decision
	}

	if gb.State.Comment != nil {
		output[keyOutputComment] = gb.State.Comment
	}

	return output, nil
}
