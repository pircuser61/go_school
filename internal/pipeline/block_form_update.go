package pipeline

import (
	c "context"
	"encoding/json"
	"fmt"

	"time"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"

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
		step, err := gb.RunContext.Storage.GetTaskStepById(ctx, data.Id)
		if err != nil {
			return nil, err
		}

		if step == nil {
			return nil, errors.New("can't get step from database")
		}
		if errUpdate := gb.formCancelPipeline(ctx, data, step); errUpdate != nil {
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

	step, err := gb.RunContext.Storage.GetTaskStepById(ctx, data.Id)
	if err != nil {
		return nil, err
	} else if step == nil {
		return nil, errors.New("can't get step from database")
	}

	stepData, ok := step.State[gb.Name]
	if !ok {
		return nil, errors.New("can't get step state")
	}

	var state FormData
	err = json.Unmarshal(stepData, &state)
	if err != nil {
		return nil, errors.Wrap(err, "invalid format of go-form-block state")
	}

	state.DidSLANotification = gb.State.DidSLANotification
	gb.State = &state

	if gb.State.IsFilled {
		isAllowed, checkEditErr := gb.RunContext.Storage.CheckUserCanEditForm(ctx, data.WorkNumber, gb.Name, data.ByLogin)
		if checkEditErr != nil {
			return nil, err
		}

		if !isAllowed {
			return nil, fmt.Errorf("%s have not permission to edit form", data.ByLogin)
		}
	} else {
		if _, ok = gb.State.Executors[data.ByLogin]; !ok {
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

	step.State[gb.Name], err = json.Marshal(gb.State)
	if err != nil {
		return nil, err
	}

	content, err := json.Marshal(store.NewFromStep(step))
	if err != nil {
		return nil, err
	}

	err = gb.RunContext.Storage.UpdateStepContext(ctx, &db.UpdateStepRequest{
		Id:          data.Id,
		Content:     content,
		BreakPoints: step.BreakPoints,
		HasError:    false,
		Status:      step.Status,
	})
	if err != nil {
		return nil, err
	}

	return nil, nil
}

//nolint:dupl // different block
func (gb *GoFormBlock) formCancelPipeline(ctx c.Context, in *script.BlockUpdateData, step *entity.Step) (err error) {
	gb.State.IsRevoked = true

	if step.State[gb.Name], err = json.Marshal(gb.State); err != nil {
		return err
	}
	var content []byte
	if content, err = json.Marshal(store.NewFromStep(step)); err != nil {
		return err
	}
	err = gb.RunContext.Storage.UpdateStepContext(ctx, &db.UpdateStepRequest{
		Id:          in.Id,
		Content:     content,
		BreakPoints: step.BreakPoints,
		Status:      string(StatusCancel),
	})
	return err
}
