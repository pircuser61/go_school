package pipeline

import (
	c "context"

	e "gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

type setFormEventsDto struct {
	wasAlreadyFilled bool
	executorsLogins  map[string]struct{}
}

func (gb *GoFormBlock) setEvents(ctx c.Context, dto *setFormEventsDto) error {
	data := gb.RunContext.UpdateData

	humanStatus, _, _ := gb.GetTaskHumanStatus()

	switch data.Action {
	case string(e.TaskUpdateActionRequestFillForm):
		if !gb.State.IsTakenInWork {
			break
		}

		kafkaEvent, err := gb.RunContext.MakeNodeKafkaEvent(ctx, &MakeNodeKafkaEvent{
			EventName:      string(e.TaskUpdateActionRequestFillForm),
			NodeName:       gb.Name,
			NodeShortName:  gb.ShortName,
			HumanStatus:    humanStatus,
			NodeStatus:     gb.GetStatus(),
			NodeType:       BlockGoFormID,
			SLA:            gb.State.Deadline.Unix(),
			ToAddLogins:    []string{},
			ToRemoveLogins: []string{data.ByLogin},
		})
		if err != nil {
			return err
		}

		gb.happenedKafkaEvents = append(gb.happenedKafkaEvents, kafkaEvent)

	case string(e.TaskUpdateActionFormExecutorStartWork):
		if gb.State.IsTakenInWork {
			break
		}

		kafkaEvent, err := gb.RunContext.MakeNodeKafkaEvent(ctx, &MakeNodeKafkaEvent{
			EventName:      string(e.TaskUpdateActionFormExecutorStartWork),
			NodeName:       gb.Name,
			NodeShortName:  gb.ShortName,
			HumanStatus:    humanStatus,
			NodeStatus:     gb.GetStatus(),
			NodeType:       BlockGoFormID,
			SLA:            gb.State.Deadline.Unix(),
			ToAddLogins:    []string{data.ByLogin},
			ToRemoveLogins: getSliceFromMap(getDifMaps(dto.executorsLogins, map[string]struct{}{data.ByLogin: {}})),
		})
		if err != nil {
			return err
		}

		gb.happenedKafkaEvents = append(gb.happenedKafkaEvents, kafkaEvent)
	}

	//nolint:all //its ok here
	if len(gb.State.ApplicationBody) > 0 && !dto.wasAlreadyFilled {
		if _, ok := gb.expectedEvents[eventEnd]; ok {
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
				NodeType:       BlockGoFormID,
				SLA:            gb.State.Deadline.Unix(),
				ToRemoveLogins: getSliceFromMap(gb.State.Executors),
			})
			if eventErr != nil {
				return eventErr
			}

			gb.happenedKafkaEvents = append(gb.happenedKafkaEvents, kafkaEvent)
		}
	}

	return nil
}
