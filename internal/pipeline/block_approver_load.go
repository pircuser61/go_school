package pipeline

import (
	"context"
	"encoding/json"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

func (gb *GoApproverBlock) load(
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

	if reEntry {
		err := gb.reentryMakeExpectedEvents(ctx, runCtx, name, ef)
		if err != nil {
			return false, err
		}
	}

	return reEntry, nil
}
