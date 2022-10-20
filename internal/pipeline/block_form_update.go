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
)

type updateFillFormParams struct {
	Description     string                 `json:"description"`
	ApplicationBody map[string]interface{} `json:"application_body"`
}

func (a *updateFillFormParams) Validate() error {
	if a.Description == "" || len(a.ApplicationBody) == 0 {
		return errors.New("empty form data")
	}

	return nil
}

func (gb *GoFormBlock) Update(ctx c.Context, data *script.BlockUpdateData) (interface{}, error) {
	if data == nil {
		return nil, errors.New("empty data")
	}

	var updateParams updateFillFormParams
	err := json.Unmarshal(data.Parameters, &updateParams)
	if err != nil {
		return nil, errors.New("can't assert provided data")
	}

	step, err := gb.Pipeline.Storage.GetTaskStepById(ctx, data.Id)
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

	if _, ok = gb.State.Executors[data.ByLogin]; !ok {
		return nil, fmt.Errorf("%s not found in executors", data.ByLogin)
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

	err = gb.Pipeline.Storage.UpdateStepContext(ctx, &db.UpdateStepRequest{
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
