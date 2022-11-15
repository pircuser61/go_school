package pipeline

import c "context"

func (runCtx *BlockRunContext) changeTaskStatus(ctx c.Context, taskStatus int) error {
	errChange := runCtx.Storage.ChangeTaskStatus(ctx, runCtx.Tx, runCtx.TaskID, taskStatus)
	if errChange != nil {
		runCtx.VarStore.AddError(errChange)

		return errChange
	}

	return nil
}

func (runCtx *BlockRunContext) updateStatusByStep(ctx c.Context, status TaskHumanStatus) error {
	if status == "" {
		return nil
	}
	return runCtx.Storage.UpdateTaskHumanStatus(ctx, runCtx.Tx, runCtx.TaskID, string(status))
}
