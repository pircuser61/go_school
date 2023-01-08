package pipeline

import (
	c "context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	human_tasks "gitlab.services.mts.ru/jocasta/pipeliner/internal/human-tasks"
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

// nolint:dupl // another action
func (gb *GoFormBlock) cancelPipeline(ctx c.Context, delegations human_tasks.Delegations) error {
	var currentLogin = gb.RunContext.UpdateData.ByLogin
	var initiator = gb.RunContext.Initiator

	var delegateFor = delegations.DelegateFor(currentLogin)

	if currentLogin != initiator && delegateFor != "" {
		return fmt.Errorf("%s is not an initiator or delegate", currentLogin)
	}

	gb.State.IsRevoked = true
	if stopErr := gb.RunContext.Storage.StopTaskBlocks(ctx, gb.RunContext.TaskID); stopErr != nil {
		return stopErr
	}
	if stopErr := gb.RunContext.updateTaskStatus(ctx, db.RunStatusFinished); stopErr != nil {
		return stopErr
	}

	stateBytes, err := json.Marshal(gb.State)
	if err != nil {
		return err
	}

	gb.RunContext.VarStore.ReplaceState(gb.Name, stateBytes)
	return nil
}

//nolint:gocyclo //ok
func (gb *GoFormBlock) Update(ctx c.Context) (interface{}, error) {
	delegates := getDelegates(gb.RunContext.VarStore)

	data := gb.RunContext.UpdateData
	if data == nil {
		return nil, errors.New("empty data")
	}
	if data.Action == string(entity.TaskUpdateActionCancelApp) {
		if errUpdate := gb.cancelPipeline(ctx, delegates); errUpdate != nil {
			return nil, errUpdate
		}
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
		isAllowed, checkEditErr := gb.RunContext.Storage.CheckUserCanEditForm(ctx, gb.RunContext.WorkNumber,
			gb.Name, data.ByLogin)
		if checkEditErr != nil {
			return nil, checkEditErr
		}

		if !isAllowed {
			return nil, fmt.Errorf("%s have not permission to edit form", data.ByLogin)
		}
	} else {
		_, executorFound := gb.State.Executors[data.ByLogin]

		var delegateFor = delegates.DelegateFor(data.ByLogin)
		if !(executorFound || delegateFor != "") {
			return nil, fmt.Errorf("%s not found in approvers or delegates", data.ByLogin)
		}

		gb.State.ActualExecutor = &data.ByLogin
		gb.State.IsFilled = true
	}

	gb.State.ApplicationBody = updateParams.ApplicationBody
	gb.State.Description = updateParams.Description

	gb.State.ChangesLog = append([]ChangesLogItem{
		{
			Description:     updateParams.Description,
			ApplicationBody: updateParams.ApplicationBody,
			CreatedAt:       time.Now(),
			Executor:        data.ByLogin,
			DelegateFor:     delegates.DelegateFor(data.ByLogin),
		},
	}, gb.State.ChangesLog...)

	personData, err := gb.RunContext.ServiceDesc.GetSsoPerson(ctx, *gb.State.ActualExecutor)
	if err != nil {
		return nil, err
	}

	gb.RunContext.VarStore.SetValue(gb.Output[keyOutputFormExecutor], personData)
	gb.RunContext.VarStore.SetValue(gb.Output[keyOutputFormBody], gb.State.ApplicationBody)

	var stateBytes []byte
	stateBytes, err = json.Marshal(gb.State)
	if err != nil {
		return nil, err
	}

	gb.RunContext.VarStore.ReplaceState(gb.Name, stateBytes)
	return nil, nil
}
