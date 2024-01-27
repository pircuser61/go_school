package pipeline

import "context"

func (gb *GoExecutionBlock) returnToAdminForRevision(
	ctx context.Context,
	delegateFor string,
	updateParams executorUpdateEditParams,
) (err error) {
	err = gb.State.setEditAppToInitiator(
		gb.RunContext.UpdateData.ByLogin,
		delegateFor,
		updateParams,
	)
	if err != nil {
		return err
	}

	err = gb.notifyNeedRework(ctx)
	if err != nil {
		return err
	}

	err = gb.RunContext.Services.Storage.FinishTaskBlocks(ctx, gb.RunContext.TaskID, []string{gb.Name}, false)
	if err != nil {
		return err
	}

	return nil
}
