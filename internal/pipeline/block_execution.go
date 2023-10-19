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
	keyOutputExecutionLogin    = "login"
	keyOutputExecutionDecision = "decision"
	keyOutputExecutionComment  = "comment"

	ExecutionDecisionExecuted ExecutionDecision = "executed"
	ExecutionDecisionRejected ExecutionDecision = "rejected"
	ExecutionDecisionSentEdit ExecutionDecision = "sent_edit"

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

	expectedEvents map[string]struct{}
	happenedEvents []entity.NodeEvent
}

func (gb *GoExecutionBlock) GetNewEvents() []entity.NodeEvent {
	return gb.happenedEvents
}

func (gb *GoExecutionBlock) Members() []Member {
	members := []Member{}
	addedMembers := make(map[string]struct{}, 0)
	for login := range gb.State.Executors {
		members = append(members, Member{
			Login:                login,
			Actions:              gb.executionActions(),
			IsActed:              gb.isExecutionActed(login),
			ExecutionGroupMember: gb.isPartOfExecutionGroup(login),
		})
		addedMembers[login] = struct{}{}
	}
	for i := 0; i < len(gb.State.EditingAppLog); i++ {
		log := gb.State.EditingAppLog[i]
		if _, ok := addedMembers[log.Executor]; !ok {
			continue
		}
		members = append(members, Member{
			Login:                log.Executor,
			Actions:              []MemberAction{},
			IsActed:              true,
			ExecutionGroupMember: gb.isPartOfExecutionGroup(log.Executor),
		})
		addedMembers[log.Executor] = struct{}{}
	}

	for i := 0; i < len(gb.State.RequestExecutionInfoLogs); i++ {
		log := gb.State.RequestExecutionInfoLogs[i]
		if _, ok := addedMembers[log.Login]; !ok {
			continue
		}
		if log.ReqType == RequestInfoQuestion {
			members = append(members, Member{
				Login:                log.Login,
				Actions:              []MemberAction{},
				IsActed:              true,
				ExecutionGroupMember: gb.isPartOfExecutionGroup(log.Login),
			})
			addedMembers[log.Login] = struct{}{}
		}
	}

	for i := range gb.State.ChangedExecutorsLogs {
		if _, ok := addedMembers[gb.State.ChangedExecutorsLogs[i].OldLogin]; !ok {
			continue
		}
		members = append(members, Member{
			Login:                gb.State.ChangedExecutorsLogs[i].OldLogin,
			Actions:              []MemberAction{},
			IsActed:              true,
			ExecutionGroupMember: gb.isPartOfExecutionGroup(gb.State.ChangedExecutorsLogs[i].OldLogin),
		})
		addedMembers[gb.State.ChangedExecutorsLogs[i].OldLogin] = struct{}{}
	}

	if gb.State.ActualExecutor != nil {
		if _, ok := addedMembers[*gb.State.ActualExecutor]; !ok {
			members = append(members, Member{
				Login:                *gb.State.ActualExecutor,
				Actions:              []MemberAction{},
				IsActed:              true,
				ExecutionGroupMember: gb.isPartOfExecutionGroup(*gb.State.ActualExecutor),
			})
			addedMembers[*gb.State.ActualExecutor] = struct{}{}
		}
	}

	for key := range gb.State.InitialExecutors {
		if _, ok := addedMembers[key]; ok {
			continue
		}
		members = append(members, Member{
			Login:                key,
			Actions:              []MemberAction{},
			IsActed:              !gb.isPartOfExecutionGroup(key),
			ExecutionGroupMember: gb.isPartOfExecutionGroup(key),
		})
		addedMembers[key] = struct{}{}
	}
	return members
}

func (gb *GoExecutionBlock) isExecutionActed(login string) bool {

	if (gb.State.ActualExecutor != nil && *gb.State.ActualExecutor == login) && gb.State.Decision != nil {
		return true
	}

	for i := 0; i < len(gb.State.EditingAppLog); i++ {
		log := gb.State.EditingAppLog[i]
		if log.Executor == login || log.DelegateFor == login {
			return true
		}
	}
	for i := 0; i < len(gb.State.ChangedExecutorsLogs); i++ {
		log := gb.State.ChangedExecutorsLogs[i]
		if log.OldLogin == login {
			return true
		}
	}

	for i := 0; i < len(gb.State.RequestExecutionInfoLogs); i++ {
		log := gb.State.RequestExecutionInfoLogs[i]
		if (log.Login == login || log.DelegateFor == login) && log.ReqType == RequestInfoQuestion {
			return true
		}
	}
	if gb.State.IsTakenInWork {
		return true
	}
	return false
}

func (gb *GoExecutionBlock) isPartOfExecutionGroup(login string) bool {
	switch gb.State.ExecutionType {
	case script.ExecutionTypeGroup:
		if _, ok := gb.State.InitialExecutors[login]; ok {
			return true
		}
	case script.ExecutionTypeFromSchema:
		if len(gb.State.InitialExecutors) > 1 {
			if _, ok := gb.State.InitialExecutors[login]; ok {
				return true
			}
		}
	default:
		return false
	}
	return false
}

func (gb *GoExecutionBlock) executionActions() []MemberAction {
	if gb.State.Decision != nil {
		return nil
	}

	if !gb.State.IsTakenInWork {
		action := MemberAction{
			Id:   executionStartWorkAction,
			Type: ActionTypePrimary,
		}
		return []MemberAction{action}
	}

	actions := []MemberAction{
		{
			Id:   executionExecuteAction,
			Type: ActionTypePrimary,
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
		},
	}
	if gb.State.IsEditable {
		actions = append(actions, MemberAction{
			Id:   executionSendEditAppAction,
			Type: ActionTypeOther,
		})
	}

	return actions
}

func (gb *GoExecutionBlock) getNewSLADeadline(slaInfoPtr *sla.SLAInfo, half bool) time.Time {
	newSLA := gb.State.SLA
	if half {
		newSLA /= 2
	}
	deadline := gb.RunContext.Services.SLAService.ComputeMaxDate(gb.RunContext.CurrBlockStartTime, float32(newSLA), slaInfoPtr)

	var qTime time.Time
	for _, item := range gb.State.RequestExecutionInfoLogs {
		if qTime.IsZero() {
			qTime = item.CreatedAt
			continue
		}

		additionalHours := gb.RunContext.Services.SLAService.GetWorkHoursBetweenDates(qTime, item.CreatedAt, nil)
		deadline = gb.RunContext.Services.SLAService.ComputeMaxDate(deadline, float32(additionalHours), nil)
		qTime = time.Time{}
	}
	return deadline
}

//nolint:dupl,gocyclo //Need here
func (gb *GoExecutionBlock) Deadlines(ctx context.Context) ([]Deadline, error) {
	deadlines := make([]Deadline, 0, 2)

	latestInfoRequest := gb.State.latestUnansweredAddInfoLogEntry()

	if gb.State.Decision != nil && latestInfoRequest != nil && latestInfoRequest.ReqType == RequestInfoQuestion {
		if gb.State.CheckDayBeforeSLARequestInfo {
			deadlines = append(deadlines, Deadline{
				Deadline: gb.RunContext.Services.SLAService.ComputeMaxDate(latestInfoRequest.CreatedAt,
					2*8, nil),
				Action: entity.TaskUpdateActionDayBeforeSLARequestAddInfo,
			})
		}

		deadlines = append(deadlines, Deadline{
			Deadline: gb.RunContext.Services.SLAService.ComputeMaxDate(latestInfoRequest.CreatedAt,
				3*8, nil),
			Action: entity.TaskUpdateActionSLABreachRequestAddInfo,
		})

		return deadlines, nil
	}

	if gb.State.CheckSLA && (latestInfoRequest == nil || latestInfoRequest.ReqType == RequestInfoAnswer) {
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
				Deadline{Deadline: gb.getNewSLADeadline(slaInfoPtr, false),
					Action: entity.TaskUpdateActionSLABreach,
				},
			)
		}

		if !gb.State.HalfSLAChecked {
			deadlines = append(deadlines,
				Deadline{Deadline: gb.getNewSLADeadline(slaInfoPtr, true),
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

	if latestInfoRequest != nil && latestInfoRequest.ReqType == RequestInfoQuestion {
		if gb.State.CheckDayBeforeSLARequestInfo {
			deadlines = append(deadlines, Deadline{
				Deadline: gb.RunContext.Services.SLAService.ComputeMaxDate(latestInfoRequest.CreatedAt,
					2*8, nil),
				Action: entity.TaskUpdateActionDayBeforeSLARequestAddInfo,
			})
		}

		deadlines = append(deadlines, Deadline{
			Deadline: gb.RunContext.Services.SLAService.ComputeMaxDate(latestInfoRequest.CreatedAt,
				3*8, nil),
			Action: entity.TaskUpdateActionSLABreachRequestAddInfo,
		})
	}

	return deadlines, nil
}

func (gb *GoExecutionBlock) UpdateManual() bool {
	return true
}

// nolint:dupl // another block
func (gb *GoExecutionBlock) GetTaskHumanStatus() (status TaskHumanStatus, comment string) {
	if gb.State != nil && gb.State.Decision != nil {
		if *gb.State.Decision == ExecutionDecisionExecuted {
			return StatusDone, ""
		}
		return StatusExecutionRejected, ""
	}

	if gb.State.EditingApp != nil {
		return StatusWait, ""
	}

	if len(gb.State.RequestExecutionInfoLogs) > 0 &&
		gb.State.RequestExecutionInfoLogs[len(gb.State.RequestExecutionInfoLogs)-1].ReqType == RequestInfoQuestion {
		return StatusWait, ""
	}

	return StatusExecution, ""
}

// nolint:dupl // another block
func (gb *GoExecutionBlock) GetStatus() Status {
	if gb.State != nil && gb.State.Decision != nil {
		if *gb.State.Decision == ExecutionDecisionRejected {
			return StatusNoSuccess
		}

		if *gb.State.Decision == ExecutionDecisionSentEdit {
			return StatusNoSuccess
		}

		return StatusFinished
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

	if gb.State != nil && gb.State.Decision != nil && *gb.State.Decision == ExecutionDecisionSentEdit {
		key = executionEditAppSocketID
	}

	// возврат заявки инициатору. эта заявка дальше не пойдет по процессу
	if gb.State != nil && gb.State.Decision == nil && gb.State.EditingApp != nil {
		return nil, false
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
		Outputs: &script.JSONSchema{
			Type: "object",
			Properties: script.JSONSchemaProperties{
				keyOutputExecutionLogin: {
					Type:        "string",
					Description: "executor login",
				},
				keyOutputExecutionDecision: {
					Type:        "string",
					Description: "execution status",
				},
				keyOutputExecutionComment: {
					Type:        "string",
					Description: "execution status comment",
				},
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
		},
	}
}
