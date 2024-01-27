package pipeline

import (
	"context"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

//nolint:dupl //another block
func (gb *GoWaitForAllInputsBlock) createExpectedEvents(
	ctx context.Context,
	runCtx *BlockRunContext,
	name string,
	ef *entity.EriusFunc,
) error {
	if err := gb.createState(ctx); err != nil {
		return err
	}

	gb.RunContext.VarStore.AddStep(gb.Name)

	if _, ok := gb.expectedEvents[eventStart]; ok {
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
	}

	return nil
}
