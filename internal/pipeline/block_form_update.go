package pipeline

import (
	c "context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

type updateFillFormParams struct {
	Description     string                 `json:"description"`
	ApplicationBody map[string]interface{} `json:"application_body"`
	BlockId         string                 `json:"block_id"`
}

func (a *updateFillFormParams) Validate() error {
	if a.BlockId == "" || a.Description == "" || len(a.ApplicationBody) == 0 {
		return errors.New("empty form data")
	}

	return nil
}

//nolint:gocyclo //ok
func (gb *GoFormBlock) Update(ctx c.Context) (interface{}, error) {
	data := gb.RunContext.UpdateData
	if data == nil {
		return nil, errors.New("empty data")
	}
	if data.Action == string(entity.TaskUpdateActionCancelApp) {
		gb.State.IsRevoked = true
		if err := gb.RunContext.Storage.StopTaskBlocks(ctx, gb.RunContext.TaskID); err != nil {
			return nil, err
		}
		if err := gb.RunContext.changeTaskStatus(ctx, db.RunStatusFinished); err != nil {
			return nil, err
		}

		var stateBytes []byte
		stateBytes, err := json.Marshal(gb.State)
		if err != nil {
			return nil, err
		}

		gb.RunContext.VarStore.ReplaceState(gb.Name, stateBytes)
		return nil, nil
	}
	var updateParams updateFillFormParams
	err := json.Unmarshal(data.Parameters, &updateParams)
	if err != nil {
		return nil, errors.New("can't assert provided data")
	}

	if updateParams.BlockId != gb.Name {
		return nil, fmt.Errorf("wrong form id: %s, gb.Name: %s", updateParams.BlockId, gb.Name)
	}

	if gb.State.IsFilled {
		isAllowed, checkEditErr := gb.RunContext.Storage.CheckUserCanEditForm(ctx, data.WorkNumber, gb.Name, data.ByLogin)
		if checkEditErr != nil {
			return nil, err
		}

		if !isAllowed {
			return nil, fmt.Errorf("%s have not permission to edit form", data.ByLogin)
		}
	} else {
		if _, ok := gb.State.Executors[data.ByLogin]; !ok {
			return nil, fmt.Errorf("%s not found in executors", data.ByLogin)
		}
	}

	gb.State.ActualExecutor = &data.ByLogin
	gb.State.ApplicationBody = updateParams.ApplicationBody
	gb.State.Description = updateParams.Description
	gb.State.IsFilled = true

	gb.State.ChangesLog = append([]ChangesLogItem{
		{
			Description:     updateParams.Description,
			ApplicationBody: updateParams.ApplicationBody,
			CreatedAt:       time.Now(),
			Executor:        data.ByLogin,
		},
	}, gb.State.ChangesLog...)

	gb.RunContext.VarStore.SetValue(gb.Output[keyOutputFormExecutor], &data.ByLogin)
	gb.RunContext.VarStore.SetValue(gb.Output[keyOutputFormBody], gb.State.ApplicationBody)

	var stateBytes []byte
	stateBytes, err = json.Marshal(gb.State)
	if err != nil {
		return nil, err
	}

	gb.RunContext.VarStore.ReplaceState(gb.Name, stateBytes)
	return nil, nil
}
