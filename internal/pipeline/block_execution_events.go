package pipeline

import (
	c "context"

	e "gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

func (gb *GoExecutionBlock) setEvents(ctx c.Context, executors map[string]struct{}) error {
	data := gb.RunContext.UpdateData

	humanStatus, _, _ := gb.GetTaskHumanStatus()

	switch data.Action {
	case string(e.TaskUpdateActionExecution):
		comment := ""
		if gb.State.DecisionComment != nil {
			comment = *gb.State.DecisionComment
		}

		delegateFor, _ := gb.RunContext.Delegations.FindDelegatorFor(data.ByLogin, getSliceFromMap(gb.State.Executors))

		kafkaEvent, err := gb.RunContext.MakeNodeKafkaEvent(ctx, &MakeNodeKafkaEvent{
			EventName:      string(e.TaskUpdateActionExecution),
			NodeName:       gb.Name,
			NodeShortName:  gb.ShortName,
			HumanStatus:    humanStatus,
			NodeStatus:     gb.GetStatus(),
			NodeType:       BlockGoExecutionID,
			SLA:            gb.State.Deadline.Unix(),
			Decision:       gb.State.Decision.String(),
			DelegateFor:    delegateFor,
			Comment:        comment,
			ToAddLogins:    []string{},
			ToRemoveLogins: []string{data.ByLogin},
		})
		if err != nil {
			return err
		}

		gb.happenedKafkaEvents = append(gb.happenedKafkaEvents, kafkaEvent)
	case string(e.TaskUpdateActionChangeExecutor):
		kafkaEvent, err := gb.RunContext.MakeNodeKafkaEvent(ctx, &MakeNodeKafkaEvent{
			EventName:      string(e.TaskUpdateActionChangeExecutor),
			NodeName:       gb.Name,
			NodeShortName:  gb.ShortName,
			HumanStatus:    humanStatus,
			NodeStatus:     gb.GetStatus(),
			NodeType:       BlockGoExecutionID,
			SLA:            gb.State.Deadline.Unix(),
			ToAddLogins:    []string{data.ByLogin},
			ToRemoveLogins: getSliceFromMap(getDifMaps(executors, map[string]struct{}{data.ByLogin: {}})),
		})
		if err != nil {
			return err
		}

		gb.happenedKafkaEvents = append(gb.happenedKafkaEvents, kafkaEvent)
	case string(e.TaskUpdateActionExecutorStartWork):
		kafkaEvent, err := gb.RunContext.MakeNodeKafkaEvent(ctx, &MakeNodeKafkaEvent{
			EventName:      string(e.TaskUpdateActionExecutorStartWork),
			NodeName:       gb.Name,
			NodeShortName:  gb.ShortName,
			HumanStatus:    humanStatus,
			NodeStatus:     gb.GetStatus(),
			NodeType:       BlockGoExecutionID,
			SLA:            gb.State.Deadline.Unix(),
			ToAddLogins:    []string{data.ByLogin},
			ToRemoveLogins: getSliceFromMap(getDifMaps(executors, map[string]struct{}{data.ByLogin: {}})),
		})
		if err != nil {
			return err
		}

		gb.happenedKafkaEvents = append(gb.happenedKafkaEvents, kafkaEvent)
	case string(e.TaskUpdateActionReworkSLABreach):
		decision := ""
		if gb.State.Decision != nil {
			decision = gb.State.Decision.String()
		}

		kafkaEvent, err := gb.RunContext.MakeNodeKafkaEvent(ctx, &MakeNodeKafkaEvent{
			EventName:      string(e.TaskUpdateActionReworkSLABreach),
			NodeName:       gb.Name,
			NodeShortName:  gb.ShortName,
			HumanStatus:    humanStatus,
			NodeStatus:     gb.GetStatus(),
			NodeType:       BlockGoExecutionID,
			SLA:            gb.State.Deadline.Unix(),
			Decision:       decision,
			ToAddLogins:    []string{},
			ToRemoveLogins: getSliceFromMap(gb.State.Executors),
		})
		if err != nil {
			return err
		}

		gb.happenedKafkaEvents = append(gb.happenedKafkaEvents, kafkaEvent)
	}

	if gb.State.Decision != nil {
		_, ok := gb.expectedEvents[eventEnd]
		if !ok {
			return nil
		}

		event, eventErr := gb.RunContext.MakeNodeEndEvent(ctx, MakeNodeEndEventArgs{
			NodeName:      gb.Name,
			NodeShortName: gb.ShortName,
			HumanStatus:   humanStatus,
			NodeStatus:    gb.GetStatus(),
		})
		if eventErr != nil {
			return eventErr
		}

		gb.happenedEvents = append(gb.happenedEvents, event)

		kafkaEvent, eventErr := gb.RunContext.MakeNodeKafkaEvent(ctx, &MakeNodeKafkaEvent{
			EventName:      eventEnd,
			NodeName:       gb.Name,
			NodeShortName:  gb.ShortName,
			HumanStatus:    humanStatus,
			NodeStatus:     gb.GetStatus(),
			NodeType:       BlockGoExecutionID,
			SLA:            gb.State.Deadline.Unix(),
			ToRemoveLogins: getSliceFromMap(gb.State.Executors),
		})

		if eventErr != nil {
			return eventErr
		}

		gb.happenedKafkaEvents = append(gb.happenedKafkaEvents, kafkaEvent)
	}

	return nil
}
