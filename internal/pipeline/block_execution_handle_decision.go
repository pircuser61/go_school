package pipeline

import (
	"context"

	"gitlab.services.mts.ru/abp/myosotis/logger"
)

func (gb *GoExecutionBlock) handleDecision(ctx context.Context, parentState *ExecutionData) error {
	const fn = "pipeline.execution.handleDecision"

	log := logger.GetLogger(ctx)

	var actualExecutor, comment string

	if parentState.ActualExecutor != nil {
		actualExecutor = *parentState.ActualExecutor
	}

	if parentState.DecisionComment != nil {
		comment = *parentState.DecisionComment
	}

	person, personErr := gb.RunContext.Services.ServiceDesc.GetSsoPerson(ctx, actualExecutor)
	if personErr != nil {
		log.Error(fn, "service couldn't get person by login: "+actualExecutor)

		return personErr
	}

	gb.RunContext.VarStore.SetValue(gb.Output[keyOutputExecutionLogin], person)
	gb.RunContext.VarStore.SetValue(gb.Output[keyOutputDecision], &parentState.Decision)
	gb.RunContext.VarStore.SetValue(gb.Output[keyOutputComment], comment)

	gb.State.ActualExecutor = &actualExecutor
	gb.State.DecisionComment = &comment
	gb.State.Decision = parentState.Decision

	_, ok := gb.expectedEvents[eventEnd]
	if ok {
		status, _, _ := gb.GetTaskHumanStatus()

		event, eventErr := gb.RunContext.MakeNodeEndEvent(ctx, MakeNodeEndEventArgs{
			NodeName:      gb.Name,
			NodeShortName: gb.ShortName,
			HumanStatus:   status,
			NodeStatus:    gb.GetStatus(),
		})
		if eventErr != nil {
			return eventErr
		}

		gb.happenedEvents = append(gb.happenedEvents, event)
	}

	return nil
}
