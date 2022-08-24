package pipeline

import (
	c "context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
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
		if err := gb.updateExecutionDecision(ctx, data, step); err != nil {
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

type ExecutorChangeParams struct {
	NewExecutorLogin string `json:"new_executor_login"`
	Comment          string `json:"comment"`
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
	gb.State.LeftToNotify[updateParams.NewExecutorLogin] = struct{}{}

	step.State[gb.Name], err = json.Marshal(gb.State)
	if err != nil {
		return err
	}

	var content []byte
	content, err = json.Marshal(store.NewFromStep(step))
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

type ExecutionUpdateParams struct {
	Decision ExecutionDecision `json:"decision"`
	Comment  string            `json:"comment"`
}

func (gb *GoExecutionBlock) updateExecutionDecision(ctx c.Context, in *script.BlockUpdateData, step *entity.Step) error {
	var updateParams ExecutionUpdateParams
	err := json.Unmarshal(in.Parameters, &updateParams)
	if err != nil {
		return errors.New("can't assert provided update data")
	}

	if errSet := gb.State.SetDecision(
		in.ByLogin,
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
	content, err = json.Marshal(store.NewFromStep(step))
	if err != nil {
		return err
	}

	err = gb.Pipeline.Storage.UpdateStepContext(ctx, &db.UpdateStepRequest{
		Id:          in.Id,
		Content:     content,
		BreakPoints: step.BreakPoints,
		Status:      step.Status,
	})

	return err
}

type updateRequestExecutionInfoDto struct {
	data *script.BlockUpdateData
	step *entity.Step
}

type RequestInfoUpdateParams struct {
	Comment     string          `json:"comment"`
	ReqType     RequestInfoType `json:"req_type"`
	Attachments []string        `json:"attachments"`
}

func (gb *GoExecutionBlock) updateRequestExecutionInfo(ctx c.Context, dto updateRequestExecutionInfoDto) (err error) {
	var updateParams RequestInfoUpdateParams
	err = json.Unmarshal(dto.data.Parameters, &updateParams)
	if err != nil {
		return errors.New("can't assert provided update requestExecutionInfo data")
	}

	if errSet := gb.State.SetRequestExecutionInfo(
		dto.data.ByLogin,
		updateParams.Comment,
		updateParams.ReqType,
		updateParams.Attachments,
	); errSet != nil {
		return errSet
	}

	status := string(StatusIdle)
	if updateParams.ReqType == RequestInfoAnswer {
		status = string(StatusRunning)
		if len(gb.State.RequestExecutionInfoLogs) > 0 {
			workHours := getWorkWorkHoursBetweenDates(
				gb.State.RequestExecutionInfoLogs[len(gb.State.RequestExecutionInfoLogs)-1].CreatedAt,
				time.Now(),
			)
			gb.State.IncreaseSLA(workHours)
		}
	}

	dto.step.State[gb.Name], err = json.Marshal(gb.State)
	if err != nil {
		return err
	}

	var content []byte
	content, err = json.Marshal(store.NewFromStep(dto.step))
	if err != nil {
		return err
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

	if updateParams.ReqType == RequestInfoQuestion {
		authorEmail, emailErr := gb.Pipeline.People.GetUserEmail(ctx, dto.data.Author)
		if emailErr != nil {
			return emailErr
		}

		tpl := mail.NewRequestExecutionInfoTemplate(dto.data.WorkNumber, dto.data.WorkTitle, gb.Pipeline.Sender.SdAddress)
		err = gb.Pipeline.Sender.SendNotification(ctx, []string{authorEmail}, nil, tpl)
		if err != nil {
			return err
		}
	}

	if updateParams.ReqType == RequestInfoAnswer {
		emails := make([]string, 0, len(gb.State.Executors))
		for executor := range gb.State.Executors {
			email, emailErr := gb.Pipeline.People.GetUserEmail(ctx, executor)
			if emailErr != nil {
				continue
			}

			emails = append(emails, email)
		}

		tpl := mail.NewAnswerExecutionInfoTemplate(dto.data.WorkNumber, dto.data.WorkTitle, gb.Pipeline.Sender.SdAddress)
		err = gb.Pipeline.Sender.SendNotification(ctx, emails, nil, tpl)
		if err != nil {
			return err
		}
	}

	return err
}
