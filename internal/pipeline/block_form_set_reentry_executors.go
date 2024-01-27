package pipeline

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
)

func (gb *GoFormBlock) setReentryExecutors(ctx context.Context) error {
	if gb.State.ReEnterSettings.GroupPath != nil && *gb.State.ReEnterSettings.GroupPath != "" {
		variableStorage, grabStorageErr := gb.RunContext.VarStore.GrabStorage()
		if grabStorageErr != nil {
			return grabStorageErr
		}

		groupID := getVariable(variableStorage, *gb.State.ReEnterSettings.GroupPath)
		if groupID == nil {
			return errors.New("can't find group id in variables")
		}

		gb.State.ReEnterSettings.Value = fmt.Sprintf("%v", groupID)
	}

	setErr := gb.setExecutorsByParams(
		ctx,
		&setFormExecutorsByParamsDTO{
			FormExecutorType: gb.State.ReEnterSettings.FormExecutorType,
			Value:            gb.State.ReEnterSettings.Value,
		},
	)
	if setErr != nil {
		return setErr
	}

	gb.State.FormExecutorType = gb.State.ReEnterSettings.FormExecutorType

	return nil
}
