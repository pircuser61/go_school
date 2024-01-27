package pipeline

import (
	"context"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

func (gb *GoSignBlock) makeExpectedEvents(ctx context.Context, runCtx *BlockRunContext, name string, ef *entity.EriusFunc) error {
	status, _, _ := gb.GetTaskHumanStatus()

	event, err := runCtx.MakeNodeStartEvent(
		ctx,
		MakeNodeStartEventArgs{
			NodeName:      name,
			NodeShortName: ef.ShortTitle,
			HumanStatus:   status,
			NodeStatus:    gb.GetStatus(),
		},
	)
	if err != nil {
		return err
	}

	gb.happenedEvents = append(gb.happenedEvents, event)

	return nil
}
