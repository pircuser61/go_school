package pipeline

import (
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

const (
	keyOutputExecutionLogin    = "login"
	keyOutputExecutionDecision = "decision"
	keyOutputExecutionComment  = "comment"

	ExecutionDecisionExecuted ExecutionDecision = "executed"
	ExecutionDecisionRejected ExecutionDecision = "rejected"

	RequestInfoQuestion RequestInfoType = "question"
	RequestInfoAnswer   RequestInfoType = "answer"

	executionStartWorkAction            = "executor_start_work"
	executionSendEditAppAction          = "executor_send_edit_app"
	executionChangeExecutorAction       = "change_executor"
	executionRequestExecutionInfoAction = "request_execution_info"
	executionExecuteAction              = "execution"
	executionDeclineAction              = "decline"
)

type GoExecutionBlock struct {
	Name    string
	Title   string
	Input   map[string]string
	Output  map[string]string
	Sockets []script.Socket
	State   *ExecutionData

	RunContext *BlockRunContext
}

func (gb *GoExecutionBlock) Members() []Member {
	members := []Member{}
	for login := range gb.State.Executors {
		members = append(members, Member{
			Login:      login,
			IsFinished: gb.isExecutionFinished(),
			Actions:    gb.executionActions(),
		})
	}
	return members
}

func (gb *GoExecutionBlock) isExecutionFinished() bool {
	if gb.State.Decision != nil || gb.State.IsRevoked {
		return true
	}
	return false
}

func (gb *GoExecutionBlock) executionActions() []MemberAction {
	if gb.State.Decision != nil || gb.State.IsRevoked {
		if gb.State.ExecutionType == script.ExecutionTypeGroup && !gb.State.IsTakenInWork {
			action := MemberAction{
				Id:   executionStartWorkAction,
				Type: ActionTypePrimary,
			}
			return []MemberAction{action}
		}
	}
	return []MemberAction{
		{
			Id:   executionExecuteAction,
			Type: ActionTypePrimary,
		},
		{
			Id:   executionSendEditAppAction,
			Type: ActionTypeOther,
		},
		{
			Id:   executionDeclineAction,
			Type: ActionTypeSecondary,
		},
		{
			Id:   executionChangeExecutorAction,
			Type: ActionTypeOther,
		},
		{
			Id:   executionRequestExecutionInfoAction,
			Type: ActionTypeOther,
		}}
}

func (gb *GoExecutionBlock) Deadlines() []Deadline {
	if gb.State.IsRevoked || gb.State.Decision != nil {
		return []Deadline{}
	}

	deadlines := make([]Deadline, 0, 2)
	if !gb.State.SLAChecked {
		deadlines = append(deadlines,
			Deadline{Deadline: ComputeMaxDate(gb.RunContext.currBlockStartTime, float32(gb.State.SLA)),
				Action: entity.TaskUpdateActionSLABreach,
			},
		)
	}

	if !gb.State.HalfSLAChecked {
		deadlines = append(deadlines,
			Deadline{Deadline: ComputeMaxDate(gb.RunContext.currBlockStartTime, float32(gb.State.SLA)/2),
				Action: entity.TaskUpdateActionHalfSLABreach,
			},
		)
	}

	if gb.State.IsEditable && gb.State.CheckReworkSLA && gb.State.EditingApp != nil {
		deadlines = append(deadlines,
			Deadline{Deadline: ComputeMaxDate(gb.State.EditingApp.CreatedAt, float32(gb.State.ReworkSLA)),
				Action: entity.TaskUpdateActionReworkSLABreach,
			},
		)
	}

	//if len(gb.State.RequestExecutionInfoLogs) > 0 &&
	//	gb.State.RequestExecutionInfoLogs[len(gb.State.RequestExecutionInfoLogs)-1].ReqType == RequestInfoQuestion {
	//	if gb.State.CheckDayBeforeSLARequestInfo {
	//		deadlines = append(deadlines, Deadline{
	//			Deadline: ComputeMaxDate(gb.State.RequestExecutionInfoLogs[len(gb.State.RequestExecutionInfoLogs)-1].CreatedAt, 2*8),
	//			Action:   entity.TaskUpdateActionDayBeforeSLARequestAddInfo,
	//		})
	//	}
	//
	//	deadlines = append(deadlines, Deadline{
	//		Deadline: ComputeMaxDate(gb.State.RequestExecutionInfoLogs[len(gb.State.RequestExecutionInfoLogs)-1].CreatedAt, 3*8),
	//		Action:   entity.TaskUpdateActionSLABreachRequestAddInfo,
	//	})
	//}

	return deadlines
}

func (gb *GoExecutionBlock) UpdateManual() bool {
	return true
}

// nolint:dupl // another block
func (gb *GoExecutionBlock) GetTaskHumanStatus() TaskHumanStatus {
	if gb.State != nil && gb.State.IsRevoked {
		return StatusRevoke
	}

	if gb.State != nil && gb.State.Decision != nil {
		if *gb.State.Decision == ExecutionDecisionExecuted {
			return StatusDone
		}
		return StatusExecutionRejected
	}

	if gb.State.EditingApp != nil {
		return StatusWait
	}

	if len(gb.State.RequestExecutionInfoLogs) > 0 &&
		gb.State.RequestExecutionInfoLogs[len(gb.State.RequestExecutionInfoLogs)-1].ReqType == RequestInfoQuestion {
		return StatusWait
	}

	return StatusExecution
}

// nolint:dupl // another block
func (gb *GoExecutionBlock) GetStatus() Status {
	if gb.State != nil && gb.State.IsRevoked {
		return StatusCancel
	}

	if gb.State != nil && gb.State.Decision != nil {
		if *gb.State.Decision == ExecutionDecisionExecuted {
			return StatusFinished
		}
		return StatusNoSuccess
	}

	if gb.State.EditingApp != nil {
		return StatusIdle
	}

	if len(gb.State.RequestExecutionInfoLogs) > 0 &&
		gb.State.RequestExecutionInfoLogs[len(gb.State.RequestExecutionInfoLogs)-1].ReqType == RequestInfoQuestion {
		return StatusIdle
	}

	return StatusRunning
}

func (gb *GoExecutionBlock) Next(_ *store.VariableStore) ([]string, bool) {
	key := notExecutedSocketID
	if gb.State != nil && gb.State.Decision != nil && *gb.State.Decision == ExecutionDecisionExecuted {
		key = executedSocketID
	}

	if gb.State != nil && gb.State.Decision == nil && gb.State.EditingApp != nil {
		key = executionEditAppSocketID
	}

	nexts, ok := script.GetNexts(gb.Sockets, key)
	if !ok {
		return nil, false
	}
	return nexts, true
}

func (gb *GoExecutionBlock) GetState() interface{} {
	return gb.State
}

func (gb *GoExecutionBlock) Model() script.FunctionModel {
	return script.FunctionModel{
		ID:        BlockGoExecutionID,
		BlockType: script.TypeGo,
		Title:     gb.Title,
		Inputs:    nil,
		Outputs: []script.FunctionValueModel{
			{
				Name:    keyOutputExecutionLogin,
				Type:    "string",
				Comment: "executor login",
			},
			{
				Name:    keyOutputExecutionDecision,
				Type:    "string",
				Comment: "execution status",
			},
			{
				Name:    keyOutputExecutionComment,
				Type:    "string",
				Comment: "execution status comment",
			},
		},
		Params: &script.FunctionParams{
			Type: BlockGoExecutionID,
			Params: &script.ExecutionParams{
				Executors:          "",
				Type:               "",
				SLA:                0,
				FormsAccessibility: []script.FormAccessibility{},
			},
		},
		Sockets: []script.Socket{
			script.ExecutedSocket,
			script.NotExecutedSocket,
			script.ExecutorEditAppSocket,
		},
	}
}
