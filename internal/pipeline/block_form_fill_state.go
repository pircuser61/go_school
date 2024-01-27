package pipeline

import (
	"context"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

func (gb *GoFormBlock) handleStateFullness(ctx context.Context, data *script.BlockUpdateData) error {
	if gb.State.IsFilled {
		err := gb.handleFilledState(ctx, data)
		if err != nil {
			return err
		}
	} else {
		err := gb.handleEmptyState(data)
		if err != nil {
			return err
		}
	}

	return nil
}

func (gb *GoFormBlock) handleFilledState(ctx context.Context, data *script.BlockUpdateData) error {
	isAllowed, checkEditErr := gb.RunContext.Services.Storage.CheckUserCanEditForm(
		ctx,
		gb.RunContext.WorkNumber,
		gb.Name,
		data.ByLogin,
	)
	if checkEditErr != nil {
		return checkEditErr
	}

	if !isAllowed {
		return NewUserIsNotPartOfProcessErr()
	}

	isActualUserEqualAutoFillUser := gb.State.ActualExecutor != nil && *gb.State.ActualExecutor == AutoFillUser

	if isActualUserEqualAutoFillUser {
		gb.State.ActualExecutor = &data.ByLogin
	}

	return nil
}

func (gb *GoFormBlock) handleEmptyState(data *script.BlockUpdateData) error {
	_, executorFound := gb.State.Executors[data.ByLogin]
	if !executorFound {
		return NewUserIsNotPartOfProcessErr()
	}

	gb.State.ActualExecutor = &data.ByLogin
	gb.State.IsFilled = true

	return nil
}
