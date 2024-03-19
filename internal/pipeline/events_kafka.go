package pipeline

import (
	c "context"
	"time"

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
