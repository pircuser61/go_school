package pipeline

import (
	"context"
	"github.com/google/uuid"

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

func (gb *GoExecutionBlock) Members() map[string]struct{} {
	return gb.State.Executors
}

func (gb *GoExecutionBlock) CheckSLA() bool {
	return true
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

func (gb *GoExecutionBlock) GetType() string {
	return BlockGoExecutionID
}

func (gb *GoExecutionBlock) Inputs() map[string]string {
	return gb.Input
}

func (gb *GoExecutionBlock) Outputs() map[string]string {
	return gb.Output
}

func (gb *GoExecutionBlock) IsScenario() bool {
	return false
}

func (gb *GoExecutionBlock) handleSLA(ctx context.Context, id uuid.UUID, stepCtx *stepCtx) (bool, error) {
	//if gb.State.DidSLANotification {
	//	return false, nil
	//}
	//if CheckBreachSLA(stepCtx.stepStart, time.Now(), gb.State.SLA) {
	//	l := logger.GetLogger(ctx)
	//
	//	// nolint:dupl // handle executors
	//	if gb.State.SLA > 8 {
	//		emails := make([]string, 0, len(gb.State.Executors))
	//		for executor := range gb.State.Executors {
	//			email, err := gb.RunContext.People.GetUserEmail(ctx, executor)
	//			if err != nil {
	//				l.WithError(err).Error("couldn't get email")
	//			}
	//			emails = append(emails, email)
	//		}
	//		if len(emails) == 0 {
	//			return false, nil
	//		}
	//		err := gb.RunContext.Sender.SendNotification(ctx, emails, nil,
	//			mail.NewExecutionSLATemplate(stepCtx.workNumber, stepCtx.workTitle, gb.RunContext.Sender.SdAddress))
	//		if err != nil {
	//			return false, err
	//		}
	//	}
	//
	//	gb.State.DidSLANotification = true
	//
	//	if err := gb.dumpCurrState(ctx, id); err != nil {
	//		gb.State.DidSLANotification = false
	//		return false, err
	//	}
	//
	//	return true, nil
	//}

	return false, nil
}

//nolint:gocyclo // later
func (gb *GoExecutionBlock) DebugRun(ctx context.Context, stepCtx *stepCtx, runCtx *store.VariableStore) (err error) {
	return nil
}

func (gb *GoExecutionBlock) Next(_ *store.VariableStore) ([]string, bool) {
	key := notExecutedSocketID
	if gb.State != nil && gb.State.Decision != nil && *gb.State.Decision == ExecutionDecisionExecuted {
		key = executedSocketID
	}

	if gb.State != nil && gb.State.Decision == nil && gb.State.EditingApp != nil {
		key = editAppSocketID
	}

	nexts, ok := script.GetNexts(gb.Sockets, key)
	if !ok {
		return nil, false
	}
	return nexts, true
}

func (gb *GoExecutionBlock) Skipped(_ *store.VariableStore) []string {
	key := executedSocketID
	if gb.State != nil && gb.State.Decision != nil && *gb.State.Decision == ExecutionDecisionExecuted {
		key = notExecutedSocketID
	}
	var next, ok = script.GetNexts(gb.Sockets, key)
	if !ok {
		return nil
	}

	return next
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
			script.EditAppSocket,
		},
	}
}
