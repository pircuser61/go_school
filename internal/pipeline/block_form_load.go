package pipeline

import (
	"context"
	"encoding/json"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

func (gb *GoFormBlock) load(
	ctx context.Context,
	rawState json.RawMessage,
	runCtx *BlockRunContext,
	name string,
	ef *entity.EriusFunc,
) (reEntry bool, err error) {
	err = gb.loadState(rawState)
	if err != nil {
		return false, err
	}

	reEntry = runCtx.UpdateData == nil

	err = gb.makeNodeStartEventWithReentry(ctx, reEntry, runCtx, name, ef)
	if err != nil {
		return false, err
	}

	return reEntry, nil
}
