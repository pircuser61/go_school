package pipeline

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

func (gb *GoExecutionBlock) setExecutors(ctx context.Context, ef *entity.EriusFunc) error {
	var params script.ExecutionParams

	err := json.Unmarshal(ef.Params, &params)
	if err != nil {
		return errors.Wrap(err, "can not get execution parameters for block: "+gb.Name)
	}

	if params.ExecutorsGroupIDPath != nil && *params.ExecutorsGroupIDPath != "" {
		variableStorage, storageErr := gb.RunContext.VarStore.GrabStorage()
		if storageErr != nil {
			return storageErr
		}

		groupID := getVariable(variableStorage, *params.ExecutorsGroupIDPath)
		if groupID == nil {
			return errors.New("can't find group id in variables")
		}

		params.ExecutorsGroupID = fmt.Sprintf("%v", groupID)
	}

	if params.WorkType != nil {
		deadline, deadErr := gb.getDeadline(ctx, *params.WorkType)
		if deadErr != nil {
			return deadErr
		}

		gb.State.Deadline = deadline
	}

	err = gb.setExecutorsByParams(
		ctx,
		&setExecutorsByParamsDTO{
			Type:     params.Type,
			GroupID:  params.ExecutorsGroupID,
			Executor: params.Executors,
			WorkType: params.WorkType,
		},
	)
	if err != nil {
		return err
	}

	return nil
}
