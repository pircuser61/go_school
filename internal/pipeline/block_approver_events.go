package pipeline

import (
	c "context"
	"encoding/json"
	"errors"

	e "gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

func (gb *GoApproverBlock) setEvents(ctx c.Context) error {
	data := gb.RunContext.UpdateData

	humanStatus, _, _ := gb.GetTaskHumanStatus()

	switch data.Action {
	case string(e.TaskUpdateActionApprovement):
		byLogin := data.ByLogin

		var updateParams approverUpdateParams

		if err := json.Unmarshal(data.Parameters, &updateParams); err != nil {
			return errors.New("can't assert provided data")
		}

		if byLogin == ServiceAccount || byLogin == ServiceAccountStage || byLogin == ServiceAccountDev {
			byLogin = updateParams.Username
		}

		comment := ""
		if gb.State.Comment != nil {
			comment = *gb.State.Comment
		}

		decision := ""
		if gb.State.Decision != nil {
			decision = gb.State.Decision.String()
		}

		delegator, ok := gb.RunContext.Delegations.FindDelegatorFor(data.ByLogin, getSliceFromMap(gb.State.Approvers))
		if ok {
			byLogin = delegator
		}

		kafkaEvent, err := gb.RunContext.MakeNodeKafkaEvent(ctx, &MakeNodeKafkaEvent{
			EventName:      string(e.TaskUpdateActionApprovement),
			NodeName:       gb.Name,
			NodeShortName:  gb.ShortName,
			HumanStatus:    humanStatus,
			NodeStatus:     gb.GetStatus(),
			NodeType:       BlockGoApproverID,
			SLA:            gb.State.Deadline.Unix(),
			Decision:       decision,
			Comment:        comment,
			ToAddLogins:    []string{},
			ToRemoveLogins: []string{byLogin},
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
