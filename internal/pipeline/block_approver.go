package pipeline

import (
	"time"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

const (
	AutoActionComment = "Выполнено автоматическое действие по истечению SLA"
	AutoApprover      = "auto_approve"

	keyOutputApprover = "approver"
	keyOutputDecision = "decision"
	keyOutputComment  = "comment"

	approverSendEditAppAction           = "send_edit_app"
	approverAddApproversAction          = "add_approvers"
	approverRequestAddInfoAction        = "request_add_info"
	approverApproveAction               = "approve"
	approverAdditionalApprovementAction = "additional_approvement"
	approverBaseRejectAction            = "reject"
	approverAdditionalRejectAction      = "additional_reject"
)

type GoApproverBlock struct {
	Name    string
	Title   string
	Input   map[string]string
	Output  map[string]string
	Sockets []script.Socket
	State   *ApproverData

	RunContext *BlockRunContext
}

func (gb *GoApproverBlock) Members() []Member {
	members := []Member{}
	for login := range gb.State.Approvers {
		members = append(members, Member{
			Login:      login,
			IsFinished: gb.isApprovementBaseFinished(login),
			Actions:    gb.approvementBaseActions(login),
		})
	}
	for _, addApprover := range gb.State.AdditionalApprovers {
		members = append(members, Member{
			Login:      addApprover.ApproverLogin,
			IsFinished: gb.isApprovementAddFinished(addApprover),
			Actions:    gb.approvementAddActions(addApprover),
		})
	}
	return members
}

func (gb *GoApproverBlock) isApprovementBaseFinished(login string) bool {
	if gb.State.Decision != nil || gb.State.IsRevoked {
		return true
	}
	for _, log := range gb.State.ApproverLog {
		if log.Login == login && log.LogType == ApproverLogDecision {
			return true
		}
	}
	return false
}

func (gb *GoApproverBlock) approvementBaseActions(login string) []string {
	if gb.State.Decision != nil || gb.State.IsRevoked {
		return []string{}
	}
	for _, log := range gb.State.ApproverLog {
		if log.Login == login && log.LogType == ApproverLogDecision {
			return []string{}
		}
	}
	return []string{approverSendEditAppAction, approverAddApproversAction,
		approverRequestAddInfoAction, approverApproveAction, approverBaseRejectAction}
}

func (gb *GoApproverBlock) isApprovementAddFinished(a AdditionalApprover) bool {
	if gb.State.Decision != nil || gb.State.IsRevoked || a.Decision != "" {
		return true
	}
	return false
}

func (gb *GoApproverBlock) approvementAddActions(a AdditionalApprover) []string {
	if gb.State.Decision != nil || gb.State.IsRevoked || a.Decision != "" {
		return []string{}
	}
	return []string{approverAddApproversAction, approverRequestAddInfoAction,
		approverAdditionalApprovementAction, approverAdditionalRejectAction}
}

func (gb *GoApproverBlock) CheckSLA() (bool, time.Time) {
	return !gb.State.SLAChecked, computeMaxDate(gb.RunContext.currBlockStartTime, gb.State.SLA)
}

func (gb *GoApproverBlock) UpdateManual() bool {
	return true
}

func (gb *GoApproverBlock) GetStatus() Status {
	if gb.State != nil && gb.State.IsRevoked {
		return StatusCancel
	}
	if gb.State != nil && gb.State.Decision != nil {
		if *gb.State.Decision == ApproverDecisionApproved {
			return StatusFinished
		}

		if *gb.State.Decision == ApproverDecisionRejected {
			return StatusNoSuccess
		}
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
	if gb.State != nil && gb.State.IsRevoked {
		return StatusRevoke
	}
	if gb.State != nil && gb.State.EditingApp != nil {
		return StatusWait
	}

	if gb.State != nil && gb.State.Decision != nil {
		if *gb.State.Decision == ApproverDecisionApproved {
			return StatusApproved
		}
		if *gb.State.Decision == ApproverDecisionRejected {
			return StatusApprovementRejected
		}
	}

	if gb.State != nil && len(gb.State.AddInfo) != 0 {
		if gb.State.checkEmptyLinkIdAddInfo() {
			return StatusWait
		}
		return StatusApprovement
	}

	var lastIdx = len(gb.State.RequestApproverInfoLog) - 1
	if len(gb.State.RequestApproverInfoLog) > 0 && gb.State.RequestApproverInfoLog[lastIdx].Type == RequestAddInfoType {
		return StatusWait
	}

	return StatusApprovement
}

func (gb *GoApproverBlock) Next(_ *store.VariableStore) ([]string, bool) {
	key := rejectedSocketID
	if gb.State != nil && gb.State.Decision != nil && *gb.State.Decision == ApproverDecisionApproved {
		key = approvedSocketID
	}

	if gb.State != nil && gb.State.Decision == nil && gb.State.EditingApp != nil {
		key = editAppSocketID
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

func (gb *GoApproverBlock) Skipped(_ *store.VariableStore) []string {
	key := approvedSocketID
	if gb.State != nil && gb.State.Decision != nil && *gb.State.Decision == ApproverDecisionApproved {
		key = rejectedSocketID
	}
	var nexts, ok = script.GetNexts(gb.Sockets, key)
	if !ok {
		return nil
	}

	return nexts
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
		Outputs: []script.FunctionValueModel{
			{
				Name:    keyOutputApprover,
				Type:    "string",
				Comment: "approver login which made a decision",
			},
			{
				Name:    keyOutputDecision,
				Type:    "string",
				Comment: "block decision",
			},
			{
				Name:    keyOutputComment,
				Type:    "string",
				Comment: "approver comment",
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
			script.ApprovedSocket,
			script.RejectedSocket,
			script.EditAppSocket,
		},
	}
}
