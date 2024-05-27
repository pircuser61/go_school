package pipeline

import (
	c "context"
	"encoding/json"
	"fmt"
	"time"

	"gitlab.services.mts.ru/abp/myosotis/logger"
	"go.opencensus.io/trace"

	e "gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

type MakeNodeKafkaEvent struct {
	EventName      string
	NodeName       string
	NodeShortName  string
	HumanStatus    TaskHumanStatus
	NodeStatus     Status
	NodeType       string
	Rule           string
	Decision       string
	Comment        string
	SLA            int64
	ToAddLogins    []string
	ToRemoveLogins []string
}

func (runCtx *BlockRunContext) MakeNodeKafkaEvent(ctx c.Context, dto *MakeNodeKafkaEvent) (e.NodeKafkaEvent, error) {
	if dto.HumanStatus == "" {
		hStatus, err := runCtx.Services.Storage.GetTaskHumanStatus(ctx, runCtx.TaskID)
		if err != nil {
			return e.NodeKafkaEvent{}, nil
		}

		dto.HumanStatus = TaskHumanStatus(hStatus)
	}

	actionBody := make(map[string]interface{})
	if len(dto.ToAddLogins) > 0 {
		actionBody["toAdd"] = dto.ToAddLogins
	}

	if len(dto.ToRemoveLogins) > 0 {
		actionBody["toRemove"] = dto.ToRemoveLogins
	}

	if dto.Rule != "" {
		actionBody["rule"] = dto.Rule
	}

	if dto.Decision != "" {
		actionBody["decision"] = dto.Decision
	}

	if dto.Comment != "" {
		actionBody["comment"] = dto.Comment
	}

	return e.NodeKafkaEvent{
		TaskID:           runCtx.TaskID.String(),
		WorkNumber:       runCtx.WorkNumber,
		NodeName:         dto.NodeName,
		NodeShortName:    dto.NodeShortName,
		NodeStart:        time.Now().Unix(),
		TaskStatus:       string(dto.HumanStatus),
		NodeStatus:       string(dto.NodeStatus),
		Initiator:        runCtx.Initiator,
		CreatedAt:        time.Now().Unix(),
		NodeSLA:          dto.SLA,
		Action:           dto.EventName,
		NodeType:         dto.NodeType,
		ActionBody:       actionBody,
		AvailableActions: []string{},
	}, nil
}

func (runCtx *BlockRunContext) notifyKafkaEvents(ctx c.Context, log logger.Logger) {
	ctx, span := trace.StartSpan(ctx, "notify_kafka_events")
	defer span.End()

	for i := range runCtx.BlockRunResults.NodeKafkaEvents {
		event := runCtx.BlockRunResults.NodeKafkaEvents[i]

		err := runCtx.Services.Kafka.ProduceEventMessage(ctx, &event)
		if err != nil {
			log.WithError(err).Error(fmt.Sprintf("couldn't produce message: %+v", event))

			b, err := json.Marshal(&event)
			if err != nil {
				log.WithError(err).Error(fmt.Sprintf("couldn't marshal event: %+v", event))

				continue
			}

			_, errCreate := runCtx.Services.Storage.CreateEventToSend(ctx, &e.CreateEventToSend{
				WorkID:  event.TaskID,
				Message: b,
			})
			if errCreate != nil {
				log.WithError(errCreate).Error(fmt.Sprintf("couldn't create event to send: %+v", event))

				continue
			}

			continue
		}
	}
}
