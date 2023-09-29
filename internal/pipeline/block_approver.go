package pipeline

import (
	"context"
	"time"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sla"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

const (
	AutoActionComment = "Выполнено автоматическое действие по истечению SLA"
	AutoApprover      = "auto_approve"

	keyOutputApprover = "approver"
	keyOutputDecision = "decision"
	keyOutputComment  = "comment"

	approverAddApproversAction          = "add_approvers"
	approverRequestAddInfoAction        = "request_add_info"
	approverAdditionalApprovementAction = "additional_approvement"
	approverAdditionalRejectAction      = "additional_reject"
)

type GoApproverBlock struct {
	Name    string
	Title   string
	Input   map[string]string
	Output  map[string]string
	Sockets []script.Socket
	State   *ApproverData

	expectedEvents map[string]struct{}
	happenedEvents []entity.NodeEvent

	RunContext *BlockRunContext
}

func (gb *GoApproverBlock) GetNewEvents() []entity.NodeEvent {
	return gb.happenedEvents
}

func (gb *GoApproverBlock) Members() []Member {
	members := make([]Member, 0)
	for login := range gb.State.Approvers {
		members = append(members, Member{
			Login:   login,
			Actions: gb.approvementBaseActions(login),
			IsActed: gb.isApprovementActed(login),
		})
	}

	for i := 0; i < len(gb.State.AdditionalApprovers); i++ {
		addApprover := gb.State.AdditionalApprovers[i]
		members = append(members, Member{
			Login:   addApprover.ApproverLogin,
			Actions: gb.approvementAddActions(&addApprover),
			IsActed: gb.isApprovementActed(addApprover.ApproverLogin),
		})
	}
	return members
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
	actions := make([]MemberAction, 0)
	for i := range gb.State.ActionList {
		actions = append(actions, MemberAction{
			Id:   gb.State.ActionList[i].Id,
			Type: gb.State.ActionList[i].Type,
		})
	}
	return append(actions, MemberAction{
		Id:   approverAddApproversAction,
		Type: ActionTypeOther,
	}, MemberAction{
		Id:   approverRequestAddInfoAction,
		Type: ActionTypeOther,
	})
}

func (gb *GoApproverBlock) approvementAddActions(a *AdditionalApprover) []MemberAction {
	if gb.State.Decision != nil || a.Decision != nil || gb.State.EditingApp != nil {
		return []MemberAction{}
	}
	return []MemberAction{{
		Id:   approverAdditionalApprovementAction,
		Type: ActionTypePrimary,
	},
		{
			Id:   approverAdditionalRejectAction,
			Type: ActionTypeSecondary,
		},
		{
			Id:   approverAddApproversAction,
			Type: ActionTypeOther,
		},
		{
			Id:   approverRequestAddInfoAction,
			Type: ActionTypeOther,
		}}
}

//nolint:dupl,gocyclo //Need here
func (gb *GoApproverBlock) Deadlines(ctx context.Context) ([]Deadline, error) {
	deadlines := make([]Deadline, 0, 2)

	if gb.State.Decision != nil && len(gb.State.AddInfo) > 0 &&
		gb.State.AddInfo[len(gb.State.AddInfo)-1].Type == RequestAddInfoType {
		if gb.State.CheckDayBeforeSLARequestInfo {
			deadlines = append(deadlines, Deadline{
				Deadline: gb.RunContext.Services.SLAService.ComputeMaxDate(
					gb.State.AddInfo[len(gb.State.AddInfo)-1].CreatedAt, 2*8, nil),
				Action: entity.TaskUpdateActionDayBeforeSLARequestAddInfo,
			})
		}

		deadlines = append(deadlines, Deadline{
			Deadline: gb.RunContext.Services.SLAService.ComputeMaxDate(
				gb.State.AddInfo[len(gb.State.AddInfo)-1].CreatedAt, 3*8, nil),
			Action: entity.TaskUpdateActionSLABreachRequestAddInfo,
		})

		return deadlines, nil
	}

	if gb.State.CheckSLA {
		slaInfoPtr, getSlaInfoErr := gb.RunContext.Services.SLAService.GetSLAInfoPtr(ctx, sla.InfoDto{
			TaskCompletionIntervals: []entity.TaskCompletionInterval{{StartedAt: gb.RunContext.CurrBlockStartTime,
				FinishedAt: gb.RunContext.CurrBlockStartTime.Add(time.Hour * 24 * 100)}},
			WorkType: sla.WorkHourType(gb.State.WorkType),
		})

		if getSlaInfoErr != nil {
			return nil, getSlaInfoErr
		}
		if !gb.State.SLAChecked {
			deadlines = append(deadlines,
				Deadline{Deadline: gb.RunContext.Services.SLAService.ComputeMaxDate(
					gb.RunContext.CurrBlockStartTime, float32(gb.State.SLA), slaInfoPtr),
					Action: entity.TaskUpdateActionSLABreach,
				},
			)
		}

		if !gb.State.HalfSLAChecked {
			deadlines = append(deadlines,
				Deadline{Deadline: gb.RunContext.Services.SLAService.ComputeMaxDate(
					gb.RunContext.CurrBlockStartTime, float32(gb.State.SLA)/2, slaInfoPtr),
					Action: entity.TaskUpdateActionHalfSLABreach,
				},
			)
		}
	}

	if gb.State.IsEditable && gb.State.CheckReworkSLA && gb.State.EditingApp != nil {
		deadlines = append(deadlines,
			Deadline{Deadline: gb.RunContext.Services.SLAService.ComputeMaxDate(
				gb.State.EditingApp.CreatedAt, float32(gb.State.ReworkSLA), nil),
				Action: entity.TaskUpdateActionReworkSLABreach,
			},
		)
	}

	if len(gb.State.AddInfo) > 0 &&
		gb.State.AddInfo[len(gb.State.AddInfo)-1].Type == RequestAddInfoType {
		if gb.State.CheckDayBeforeSLARequestInfo {
			deadlines = append(deadlines, Deadline{
				Deadline: gb.RunContext.Services.SLAService.ComputeMaxDate(
					gb.State.AddInfo[len(gb.State.AddInfo)-1].CreatedAt, 2*8, nil),
				Action: entity.TaskUpdateActionDayBeforeSLARequestAddInfo,
			})
		}

		deadlines = append(deadlines, Deadline{
			Deadline: gb.RunContext.Services.SLAService.ComputeMaxDate(
				gb.State.AddInfo[len(gb.State.AddInfo)-1].CreatedAt, 3*8, nil),
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
		if gb.State.checkEmptyLinkIdAddInfo() {
			return StatusIdle
		}
	}

	return StatusRunning
}

func (gb *GoApproverBlock) GetTaskHumanStatus() TaskHumanStatus {
	if gb.State != nil && gb.State.EditingApp != nil {
		return StatusWait
	}

	if gb.State != nil && gb.State.Decision != nil {
		if *gb.State.Decision == ApproverDecisionRejected {
			return StatusApprovementRejected
		}

		if *gb.State.Decision == ApproverDecisionSentToEdit {
			return StatusApprovementRejected
		}

		return getPositiveFinishStatus(*gb.State.Decision)
	}

	if gb.State != nil && len(gb.State.AddInfo) != 0 {
		if gb.State.checkEmptyLinkIdAddInfo() {
			return StatusWait
		}
		return getPositiveProcessingStatus(gb.State.ApproveStatusName)
	}

	var lastIdx = len(gb.State.AddInfo) - 1
	if len(gb.State.AddInfo) > 0 && gb.State.AddInfo[lastIdx].Type == RequestAddInfoType {
		return StatusWait
	}

	return getPositiveProcessingStatus(gb.State.ApproveStatusName)
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
					Type:        "string",
					Description: "approver login which made a decision",
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

//nolint:gocyclo //its ok here
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

//nolint:gocyclo //its ok here
func getPositiveFinishStatus(decision ApproverDecision) (status TaskHumanStatus) {
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
