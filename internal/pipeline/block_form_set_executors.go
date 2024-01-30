package pipeline

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

func (gb *GoFormBlock) setExecutors(ctx context.Context, ef *entity.EriusFunc) error {
	if gb.State.FormExecutorType == script.FormExecutorTypeFromSchema {
		var params script.FormParams

		err := json.Unmarshal(ef.Params, &params)
		if err != nil {
			return errors.Wrap(err, "can not get form parameters in reentry")
		}

		setErr := gb.setExecutorsByParams(ctx, &setFormExecutorsByParamsDTO{
			FormExecutorType: gb.State.FormExecutorType,
			Value:            params.Executor,
		})
		if setErr != nil {
			return setErr
		}
	} else {
		gb.State.Executors = gb.State.InitialExecutors
		gb.State.IsTakenInWork = len(gb.State.InitialExecutors) == 1
	}

	return nil
}
