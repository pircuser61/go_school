package pipeline

import (
	c "context"
	"encoding/json"
	"errors"

	e "gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

func (gb *GoSignBlock) setEvents(ctx c.Context, signers map[string]struct{}) error {
	const (
		start = "start"
		end   = "end"
	)

	data := gb.RunContext.UpdateData

	humanStatus, _, _ := gb.GetTaskHumanStatus()

	switch data.Action {
	case string(e.TaskUpdateActionSign):
		var updateParams signSignatureParams

		if err := json.Unmarshal(data.Parameters, &updateParams); err != nil {
			return errors.New("can't assert provided data")
		}

		if updateParams.Username != "" {
			data.ByLogin = updateParams.Username
		}

		comment := ""
		if gb.State.Comment != nil {
			comment = *gb.State.Comment
		}

		decision := ""
		if gb.State.Decision != nil {
			decision = gb.State.Decision.String()
		}

		kafkaEvent, err := gb.RunContext.MakeNodeKafkaEvent(ctx, &MakeNodeKafkaEvent{
			EventName:      string(e.TaskUpdateActionSign),
			NodeName:       gb.Name,
			NodeShortName:  gb.ShortName,
			HumanStatus:    humanStatus,
			NodeStatus:     gb.GetStatus(),
			NodeType:       BlockGoSignID,
			SLA:            gb.State.Deadline.Unix(),
			Decision:       decision,
			Comment:        comment,
			ToAddLogins:    []string{},
			ToRemoveLogins: []string{data.ByLogin},
		})
		if err != nil {
			return err
		}

		gb.happenedKafkaEvents = append(gb.happenedKafkaEvents, kafkaEvent)
	case string(e.TaskUpdateActionSignChangeWorkStatus):
		updateParams := &changeStatusSignatureParams{}

		if gb.RunContext.UpdateData.Parameters != nil {
			err := json.Unmarshal(gb.RunContext.UpdateData.Parameters, updateParams)
			if err != nil {
				return errors.New("can't assert provided update data")
			}
		}

		if updateParams.Status == start {
			kafkaEvent, err := gb.RunContext.MakeNodeKafkaEvent(ctx, &MakeNodeKafkaEvent{
				EventName:      string(e.TaskUpdateActionSignChangeWorkStatus),
				NodeName:       gb.Name,
				NodeShortName:  gb.ShortName,
				HumanStatus:    humanStatus,
				NodeStatus:     gb.GetStatus(),
				NodeType:       BlockGoSignID,
				SLA:            gb.State.Deadline.Unix(),
				ToAddLogins:    []string{data.ByLogin},
				ToRemoveLogins: getSliceFromMap(signers),
			})
			if err != nil {
				return err
			}

			gb.happenedKafkaEvents = append(gb.happenedKafkaEvents, kafkaEvent)
		}

		if updateParams.Status == end {
			kafkaEvent, err := gb.RunContext.MakeNodeKafkaEvent(ctx, &MakeNodeKafkaEvent{
				EventName:      string(e.TaskUpdateActionSignChangeWorkStatus),
				NodeName:       gb.Name,
				NodeShortName:  gb.ShortName,
				HumanStatus:    humanStatus,
				NodeStatus:     gb.GetStatus(),
				NodeType:       BlockGoSignID,
				SLA:            gb.State.Deadline.Unix(),
				ToAddLogins:    getSliceFromMap(signers),
				ToRemoveLogins: []string{},
			})
			if err != nil {
				return err
			}

			gb.happenedKafkaEvents = append(gb.happenedKafkaEvents, kafkaEvent)
		}
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
			NodeType:       BlockGoSignID,
			SLA:            gb.State.Deadline.Unix(),
			ToRemoveLogins: getSliceFromMap(gb.State.Signers),
		})

		if eventErr != nil {
			return eventErr
		}

		gb.happenedKafkaEvents = append(gb.happenedKafkaEvents, kafkaEvent)
	}

	return nil
}
