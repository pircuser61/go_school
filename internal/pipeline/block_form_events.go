package pipeline

import (
	"context"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

func (gb *GoFormBlock) makeNodeStartEventWithReentry(
	ctx context.Context,
	reEntry bool,
	runCtx *BlockRunContext,
	name string,
	ef *entity.EriusFunc,
) error {
	if reEntry {
		if err := gb.reEntry(ctx, ef); err != nil {
			return err
		}

		gb.RunContext.VarStore.AddStep(gb.Name)

		err := gb.makeNodeStartEventIfExpected(ctx, runCtx, name, ef)
		if err != nil {
			return err
		}
	}

	return nil
}

func (gb *GoFormBlock) makeNodeStartEventIfExpected(ctx context.Context, runCtx *BlockRunContext, name string, ef *entity.EriusFunc) error {
	if _, ok := gb.expectedEvents[eventStart]; ok {
		err := gb.makeNodeStartEvent(ctx, runCtx, name, ef)
		if err != nil {
			return err
		}
	}

	return nil
}

func (gb *GoFormBlock) makeNodeStartEvent(ctx context.Context, runCtx *BlockRunContext, name string, ef *entity.EriusFunc) error {
	status, _, _ := gb.GetTaskHumanStatus()

	event, err := runCtx.MakeNodeStartEvent(ctx, MakeNodeStartEventArgs{
		NodeName:      name,
		NodeShortName: ef.ShortTitle,
		HumanStatus:   status,
		NodeStatus:    gb.GetStatus(),
	})
	if err != nil {
		return err
	}

	gb.happenedEvents = append(gb.happenedEvents, event)

	return err
}
