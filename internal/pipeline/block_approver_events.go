package pipeline

import (
	c "context"

	e "gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

func (gb *GoApproverBlock) setEvents(ctx c.Context) error {
	data := gb.RunContext.UpdateData

	humanStatus, _, _ := gb.GetTaskHumanStatus()

	switch data.Action {
	case string(e.TaskUpdateActionApprovement):
		comment := ""
		if gb.State.Comment != nil {
			comment = *gb.State.Comment
		}

		decision := ""
		if gb.State.Decision != nil {
			decision = gb.State.Decision.String()
		}

		delegateFor, _ := gb.RunContext.Delegations.FindDelegatorFor(data.ByLogin, getSliceFromMap(gb.State.Approvers))

		kafkaEvent, err := gb.RunContext.MakeNodeKafkaEvent(ctx, &MakeNodeKafkaEvent{
			EventName:      string(e.TaskUpdateActionApprovement),
			NodeName:       gb.Name,
			NodeShortName:  gb.ShortName,
			HumanStatus:    humanStatus,
			NodeStatus:     gb.GetStatus(),
			NodeType:       BlockGoApproverID,
			SLA:            gb.State.Deadline.Unix(),
			Decision:       decision,
			DelegateFor:    delegateFor,
			Comment:        comment,
			ToAddLogins:    []string{},
			ToRemoveLogins: []string{data.ByLogin},
		})
		if err != nil {
			return err
		}

		gb.happenedKafkaEvents = append(gb.happenedKafkaEvents, kafkaEvent)
	case string(e.TaskUpdateActionReworkSLABreach):
		kafkaEvent, err := gb.RunContext.MakeNodeKafkaEvent(ctx, &MakeNodeKafkaEvent{
			EventName:      string(e.TaskUpdateActionReworkSLABreach),
			NodeName:       gb.Name,
			NodeShortName:  gb.ShortName,
			HumanStatus:    humanStatus,
			NodeStatus:     gb.GetStatus(),
			NodeType:       BlockGoApproverID,
			SLA:            gb.State.Deadline.Unix(),
			Decision:       gb.State.Decision.String(),
			ToAddLogins:    []string{},
			ToRemoveLogins: getSliceFromMap(gb.State.Approvers),
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
			NodeType:       BlockGoApproverID,
			SLA:            gb.State.Deadline.Unix(),
			ToRemoveLogins: getSliceFromMap(gb.State.Approvers),
		})

		if eventErr != nil {
			return eventErr
		}

		gb.happenedKafkaEvents = append(gb.happenedKafkaEvents, kafkaEvent)
	}

	return nil
}
