package pipeline

import c "context"

func (runCtx *BlockRunContext) updateTaskStatus(ctx c.Context, taskStatus int, comment, author string) error {
	errChange := runCtx.Services.Storage.UpdateTaskStatus(ctx, runCtx.TaskID, taskStatus, comment, author)
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
	_, err := runCtx.Services.Storage.UpdateTaskHumanStatus(ctx, runCtx.TaskID, string(status))
	return err
}
