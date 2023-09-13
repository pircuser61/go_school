package pipeline

import (
	c "context"
	"time"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

const (
	eventStart = "start"
	eventEnd   = "end"
)

func (runCtx *BlockRunContext) makeNodeStartEvent(ctx c.Context, node string, humanStatus TaskHumanStatus,
	nodeStatus Status) (entity.NodeEvent, error) {
	if humanStatus == "" {
		hStatus, err := runCtx.Services.Storage.GetTaskHumanStatus(ctx, runCtx.TaskID)
		if err != nil {
			return entity.NodeEvent{}, nil
		}
		humanStatus = TaskHumanStatus(hStatus)
	}

	return entity.NodeEvent{
		TaskID:     runCtx.TaskID.String(),
		WorkNumber: runCtx.WorkNumber,
		NodeName:   node,
		NodeStart:  time.Now().Format(time.RFC3339),
		TaskStatus: string(humanStatus),
		NodeStatus: string(nodeStatus),
	}, nil
}

func (runCtx *BlockRunContext) makeNodeEndEvent(ctx c.Context, node string, humanStatus TaskHumanStatus,
	nodeStatus Status) (entity.NodeEvent, error) {
	if humanStatus == "" {
		hStatus, err := runCtx.Services.Storage.GetTaskHumanStatus(ctx, runCtx.TaskID)
		if err != nil {
			return entity.NodeEvent{}, nil
		}
		humanStatus = TaskHumanStatus(hStatus)
	}

	outputs := getBlockOutput(runCtx.VarStore, node)

	return entity.NodeEvent{
		TaskID:     runCtx.TaskID.String(),
		WorkNumber: runCtx.WorkNumber,
		NodeName:   node,
		NodeStart:  runCtx.currBlockStartTime.Format(time.RFC3339),
		NodeEnd:    time.Now().Format(time.RFC3339),
		TaskStatus: string(humanStatus),
		NodeStatus: string(nodeStatus),
		NodeOutput: outputs,
	}, nil
}
