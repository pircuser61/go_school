package pipeline

import (
	c "context"
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

func (gb *GoExecutionBlock) Update(ctx c.Context, data *script.BlockUpdateData) (interface{}, error) {
	if data == nil {
		return nil, errors.New("update data is empty")
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

	var state ExecutionData
	if err = json.Unmarshal(stepData, &state); err != nil {
		return nil, errors.Wrap(err, "invalid format of go-execution-block state")
	}

	gb.State = &state

	if data.Action == string(entity.TaskUpdateActionExecution) {
		if err := gb.updateExecution(ctx, data, step); err != nil {
			return nil, err
		}
	}

	if data.Action == string(entity.TaskUpdateActionChangeExecutor) {
		if err := gb.changeExecutor(ctx, data, step); err != nil {
			return nil, err
		}
	}

	if data.Action == string(entity.TaskUpdateActionRequestExecutionInfo) {
		if err := gb.updateRequestExecutionInfo(ctx, updateRequestExecutionInfoDto{data, step}); err != nil {
			return nil, err
		}
	}

	return nil, nil
}

func (gb *GoExecutionBlock) changeExecutor(ctx c.Context, data *script.BlockUpdateData, step *entity.Step) (err error) {
	if _, isExecutor := gb.State.Executors[data.ByLogin]; !isExecutor {
		return fmt.Errorf("can't change executor, user %s in not executor", data.ByLogin)
	}

	var updateParams ExecutorChangeParams
	err = json.Unmarshal(data.Parameters, &updateParams)
	if err != nil {
		return errors.New("can't assert provided update data")
	}

	err = gb.State.SetChangeExecutor(data.ByLogin, updateParams.NewExecutorLogin, updateParams.Comment)
	if err != nil {
		return errors.New("can't assert provided change executor data")
	}

	delete(gb.State.Executors, data.ByLogin)
	gb.State.Executors[updateParams.NewExecutorLogin] = struct{}{}

	step.State[gb.Name], err = json.Marshal(gb.State)
	if err != nil {
		return err
	}

	var content []byte
	content, err = json.Marshal(step)
	if err != nil {
		return err
	}

	err = gb.Pipeline.Storage.UpdateStepContext(ctx, &db.UpdateStepRequest{
		Id:          data.Id,
		Content:     content,
		BreakPoints: step.BreakPoints,
		Status:      string(StatusRunning),
	})

	return err
}

func (gb *GoExecutionBlock) updateExecution(ctx c.Context, data *script.BlockUpdateData, step *entity.Step) (err error) {
	var updateParams ExecutionUpdateParams
	err = json.Unmarshal(data.Parameters, &updateParams)
	if err != nil {
		return errors.New("can't assert provided update data")
	}

	if errSet := gb.State.SetDecision(
		data.ByLogin,
		updateParams.Decision,
		updateParams.Comment,
	); errSet != nil {
		return errSet
	}

	step.State[gb.Name], err = json.Marshal(gb.State)
	if err != nil {
		return err
	}

	var content []byte
	content, err = json.Marshal(step)
	if err != nil {
		return err
	}

	err = gb.Pipeline.Storage.UpdateStepContext(ctx, &db.UpdateStepRequest{
		Id:          data.Id,
		Content:     content,
		BreakPoints: step.BreakPoints,
		Status:      string(StatusFinished),
	})

	return err
}

type updateRequestExecutionInfoDto struct {
	data *script.BlockUpdateData
	step *entity.Step
}

func (gb *GoExecutionBlock) updateRequestExecutionInfo(ctx c.Context, dto updateRequestExecutionInfoDto) (err error) {
	var updateParams RequestInfoUpdateParams
	err = json.Unmarshal(dto.data.Parameters, &updateParams)
	if err != nil {
		return errors.New("can't assert provided update data")
	}

	if errSet := gb.State.SetRequestExecutionInfo(
		dto.data.ByLogin,
		updateParams.Comment,
		updateParams.ReqType,
	); errSet != nil {
		return errSet
	}

	dto.step.State[gb.Name], err = json.Marshal(gb.State)
	if err != nil {
		return err
	}

	var content []byte
	content, err = json.Marshal(dto.step)
	if err != nil {
		return err
	}

	status := string(StatusIdle)
	if updateParams.ReqType == RequestInfoAnswer {
		status = string(StatusRunning)
	}

	err = gb.Pipeline.Storage.UpdateStepContext(ctx, &db.UpdateStepRequest{
		Id:          dto.data.Id,
		Content:     content,
		BreakPoints: dto.step.BreakPoints,
		Status:      status,
	})
	if err != nil {
		return err
	}

	tpl := mail.NewRequestExecutionInfoTemplate(dto.data.WorkNumber, dto.data.WorkTitle, gb.Pipeline.Sender.SdAddress)
	err = gb.Pipeline.Sender.SendNotification(ctx, []string{dto.data.Author}, tpl)
	if err != nil {
		return err
	}

	return err
}
