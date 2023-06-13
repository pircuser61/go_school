package pipeline

import (
	"context"
	"time"

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
	return gb.State.Decision != nil
}

func (gb *GoExecutionBlock) executionActions() []MemberAction {
	if gb.State.Decision != nil {
		return nil
	}

	if gb.State.ExecutionType == script.ExecutionTypeGroup && !gb.State.IsTakenInWork {
		action := MemberAction{
			Id:   executionStartWorkAction,
			Type: ActionTypePrimary,
		}
		return []MemberAction{action}
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

//nolint:dupl,gocyclo //Need here
func (gb *GoExecutionBlock) Deadlines(ctx context.Context) ([]Deadline, error) {
	deadlines := make([]Deadline, 0, 2)

	if gb.State.Decision != nil && len(gb.State.RequestExecutionInfoLogs) > 0 &&
		gb.State.RequestExecutionInfoLogs[len(gb.State.RequestExecutionInfoLogs)-1].ReqType == RequestInfoQuestion {
		if gb.State.CheckDayBeforeSLARequestInfo {
			deadlines = append(deadlines, Deadline{
				Deadline: ComputeMaxDate(gb.State.RequestExecutionInfoLogs[len(gb.State.RequestExecutionInfoLogs)-1].CreatedAt,
					2*8, nil, nil, nil, nil),
				Action: entity.TaskUpdateActionDayBeforeSLARequestAddInfo,
			})
		}

		deadlines = append(deadlines, Deadline{
			Deadline: ComputeMaxDate(gb.State.RequestExecutionInfoLogs[len(gb.State.RequestExecutionInfoLogs)-1].CreatedAt, 3*8, nil, nil, nil, nil),
			Action:   entity.TaskUpdateActionSLABreachRequestAddInfo,
		})

		return deadlines, nil
	}

	if gb.State.CheckSLA {
		calendarDays, getCalendarDaysErr := gb.RunContext.HrGate.GetDefaultCalendarDaysForGivenTimeIntervals(ctx,
			[]entity.TaskCompletionInterval{{StartedAt: gb.RunContext.currBlockStartTime,
				FinishedAt: gb.RunContext.currBlockStartTime.Add(time.Hour * 24 * 100)}},
		)
		if getCalendarDaysErr != nil {
			return nil, getCalendarDaysErr
		}
		workHourType := WorkHourType(gb.State.WorkType)
		startWorkHour, endWorkHour, getWorkingHoursErr := workHourType.GetWorkingHours()
		if getWorkingHoursErr != nil {
			return nil, getWorkingHoursErr
		}
		weekends, getWeekendsErr := workHourType.GetWeekends()
		if getWeekendsErr != nil {
			return nil, getWeekendsErr
		}
		if !gb.State.SLAChecked {
			deadlines = append(deadlines,
				Deadline{Deadline: ComputeMaxDate(gb.RunContext.currBlockStartTime, float32(gb.State.SLA), calendarDays,
					&startWorkHour, &endWorkHour, weekends),
					Action: entity.TaskUpdateActionSLABreach,
				},
			)
		}

		if !gb.State.HalfSLAChecked {
			deadlines = append(deadlines,
				Deadline{Deadline: ComputeMaxDate(gb.RunContext.currBlockStartTime, float32(gb.State.SLA)/2,
					calendarDays, &startWorkHour, &endWorkHour, weekends),
					Action: entity.TaskUpdateActionHalfSLABreach,
				},
			)
		}
	}

	if gb.State.IsEditable && gb.State.CheckReworkSLA && gb.State.EditingApp != nil {
		deadlines = append(deadlines,
			Deadline{Deadline: ComputeMaxDate(gb.State.EditingApp.CreatedAt, float32(gb.State.ReworkSLA), nil, nil, nil, nil),
				Action: entity.TaskUpdateActionReworkSLABreach,
			},
		)
	}

	if len(gb.State.RequestExecutionInfoLogs) > 0 &&
		gb.State.RequestExecutionInfoLogs[len(gb.State.RequestExecutionInfoLogs)-1].ReqType == RequestInfoQuestion {
		if gb.State.CheckDayBeforeSLARequestInfo {
			deadlines = append(deadlines, Deadline{
				Deadline: ComputeMaxDate(gb.State.RequestExecutionInfoLogs[len(gb.State.RequestExecutionInfoLogs)-1].CreatedAt,
					2*8, nil, nil, nil, nil),
				Action: entity.TaskUpdateActionDayBeforeSLARequestAddInfo,
			})
		}

		deadlines = append(deadlines, Deadline{
			Deadline: ComputeMaxDate(gb.State.RequestExecutionInfoLogs[len(gb.State.RequestExecutionInfoLogs)-1].CreatedAt, 3*8, nil, nil, nil, nil),
			Action:   entity.TaskUpdateActionSLABreachRequestAddInfo,
		})
	}

	return deadlines, nil
}

func (gb *GoExecutionBlock) UpdateManual() bool {
	return true
}

// nolint:dupl // another block
func (gb *GoExecutionBlock) GetTaskHumanStatus() TaskHumanStatus {
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
		},
	}
}
