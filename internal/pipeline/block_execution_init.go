package pipeline

import (
	"context"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

func (gb *GoExecutionBlock) init(
	ctx context.Context,
	runCtx *BlockRunContext,
	name string,
	ef *entity.EriusFunc,
) error {
	if err := gb.createState(ctx, ef); err != nil {
		return err
	}

	gb.RunContext.VarStore.AddStep(gb.Name)

	err := gb.makeExpectedEvents(ctx, runCtx, name, ef)
	if err != nil {
		return err
	}

	// это для возврата на доработку при которой мы создаем новый процесс
	// и пытаемся взять решение из прошлого процесса
	gb.setPrevDecision(ctx)

	return nil
}
