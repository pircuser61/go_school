package pipeline

import (
	"context"
	"encoding/json"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

func (gb *GoExecutionBlock) load(
	ctx context.Context,
	rawState json.RawMessage,
	runCtx *BlockRunContext,
	name string,
	ef *entity.EriusFunc,
) (reEntry bool, err error) {
	if err := gb.loadState(rawState); err != nil {
		return false, err
	}

	reEntry = runCtx.UpdateData == nil

	// это для возврата в рамках одного процесса
	if reEntry {
		if err := gb.reEntry(ctx, ef); err != nil {
			return false, err
		}

		gb.RunContext.VarStore.AddStep(gb.Name)

		err := gb.makeExpectedEvents(ctx, runCtx, name, ef)
		if err != nil {
			return false, err
		}
	}

	return reEntry, nil
}
