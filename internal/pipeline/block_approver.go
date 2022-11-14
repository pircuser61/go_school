package pipeline

import (
	c "context"
	"github.com/google/uuid"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

const (
	AutoActionComment = "Выполнено автоматическое действие по истечению SLA"
	AutoApprover      = "auto_approve"

	keyOutputApprover = "approver"
	keyOutputDecision = "decision"
	keyOutputComment  = "comment"
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

	if gb.State != nil && len(gb.State.AddInfo) != 0 {
		if gb.State.checkEmptyLinkIdAddInfo() {
			return StatusWait
		}
		return StatusApprovement
	}

	if gb.State != nil && gb.State.Decision != nil {
		if *gb.State.Decision == ApproverDecisionApproved {
			return StatusApproved
		}
		if *gb.State.Decision == ApproverDecisionRejected {
			return StatusApprovementRejected
		}
	}

	var lastIdx = len(gb.State.RequestApproverInfoLog) - 1
	if len(gb.State.RequestApproverInfoLog) > 0 && gb.State.RequestApproverInfoLog[lastIdx].Type == RequestAddInfoType {
		return StatusWait
	}

	return StatusApprovement
}

func (gb *GoApproverBlock) GetType() string {
	return BlockGoApproverID
}

func (gb *GoApproverBlock) Inputs() map[string]string {
	return gb.Input
}

func (gb *GoApproverBlock) Outputs() map[string]string {
	return gb.Output
}

func (gb *GoApproverBlock) IsScenario() bool {
	return false
}

func (gb *GoApproverBlock) handleSLA(ctx c.Context, id uuid.UUID, stepCtx *stepCtx) (bool, error) {
	//const workHoursDay = 8
	//
	//if gb.State.DidSLANotification {
	//	return false, nil
	//}
	//if CheckBreachSLA(stepCtx.stepStart, time.Now(), gb.State.SLA) {
	//	l := logger.GetLogger(ctx)
	//
	//	// nolint:dupl // handle approvers
	//	if gb.State.SLA > workHoursDay {
	//		emails := make([]string, 0, len(gb.State.Approvers))
	//		for approver := range gb.State.Approvers {
	//			email, err := gb.RunContext.People.GetUserEmail(ctx, approver)
	//			if err != nil {
	//				l.WithError(err).Error("couldn't get email")
	//			}
	//			emails = append(emails, email)
	//		}
	//		if len(emails) == 0 {
	//			return false, nil
	//		}
	//
	//		tpl := mail.NewApprovementSLATemplate(stepCtx.workNumber, stepCtx.workTitle, gb.RunContext.Sender.SdAddress)
	//		err := gb.RunContext.Sender.SendNotification(ctx, emails, nil, tpl)
	//		if err != nil {
	//			return false, err
	//		}
	//	}
	//
	//	gb.State.DidSLANotification = true
	//
	//	if gb.State.AutoAction != nil {
	//		if err := gb.setApproverDecision(ctx,
	//			id,
	//			AutoApprover,
	//			approverUpdateParams{
	//				Decision: decisionFromAutoAction(*gb.State.AutoAction),
	//				Comment:  AutoActionComment,
	//			}); err != nil {
	//			l.WithError(err).Error("couldn't set auto decision")
	//			return false, err
	//		}
	//	} else {
	//		//if err := gb.dumpCurrState(ctx, id); err != nil {
	//		//	l.WithError(err).Error("couldn't dump state with id: " + id.String())
	//		//	return false, err
	//		//}
	//	}
	//	return true, nil
	//}
	//
	return false, nil
}

//nolint:gocyclo //ok
func (gb *GoApproverBlock) DebugRun(_ c.Context, _ *stepCtx, _ *store.VariableStore) (err error) {
	return nil
}

type getPreviousDecisionDTO struct {
	step     *entity.Step
	id       uuid.UUID
	runCtx   *store.VariableStore
	workID   uuid.UUID
	stepName string
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
