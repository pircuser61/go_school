package pipeline

import (
	c "context"
	"encoding/json"
	"errors"

	e "gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

//nolint:all //its ok here
func (gb *GoApproverBlock) setEvents(ctx c.Context) error {
	data := gb.RunContext.UpdateData

	humanStatus, _, _ := gb.GetTaskHumanStatus()

	if data.Action == string(e.TaskUpdateActionApprovement) {
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

		toRemoveLogins := make([]string, 0)

		delegators := gb.RunContext.Delegations.GetDelegators(byLogin)
		delegateFor := gb.State.delegateFor(delegators)

		for i := range delegateFor {
			_, founded := gb.State.Approvers[delegateFor[i]]
			if founded {
				toRemoveLogins = append(toRemoveLogins, delegateFor[i])
			}
		}

		_, founded := gb.State.Approvers[byLogin]
		if len(toRemoveLogins) == 0 || founded {
			toRemoveLogins = append(toRemoveLogins, byLogin)
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
			ToRemoveLogins: toRemoveLogins,
		})
		if err != nil {
			return err
		}

		gb.happenedKafkaEvents = append(gb.happenedKafkaEvents, kafkaEvent)
	}

	if gb.State.Decision != nil {
		_, ok := gb.expectedEvents[eventEnd]
		if ok {
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
		}

		loginsNotYetMadeDecision := make([]string, 0)

		if len(gb.State.ApproverLog) > 0 {
			for i := range gb.State.Approvers {
				a := false
				for j := range gb.State.ApproverLog {
					if i == gb.State.ApproverLog[j].Login {
						a = true
						break
					}
				}
				if !a {
					loginsNotYetMadeDecision = append(loginsNotYetMadeDecision, i)
				}
			}
		} else {
			loginsNotYetMadeDecision = getSliceFromMap(gb.State.Approvers)
		}

		kafkaEvent, eventErr := gb.RunContext.MakeNodeKafkaEvent(ctx, &MakeNodeKafkaEvent{
			EventName:      eventEnd,
			NodeName:       gb.Name,
			NodeShortName:  gb.ShortName,
			HumanStatus:    humanStatus,
			NodeStatus:     gb.GetStatus(),
			NodeType:       BlockGoApproverID,
			SLA:            gb.State.Deadline.Unix(),
			ToRemoveLogins: loginsNotYetMadeDecision,
		})

		if eventErr != nil {
			return eventErr
		}

		gb.happenedKafkaEvents = append(gb.happenedKafkaEvents, kafkaEvent)
	}

	return nil
}
