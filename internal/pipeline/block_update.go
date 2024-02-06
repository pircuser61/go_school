package pipeline

import (
	"context"
	"encoding/json"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/kafka"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

func (gb *ExecutableFunctionBlock) updateData(log logger.Logger) error {
	var updateData FunctionUpdateParams

	updateDataUnmarshalErr := json.Unmarshal(gb.RunContext.UpdateData.Parameters, &updateData)
	if updateDataUnmarshalErr != nil {
		return updateDataUnmarshalErr
	}

	log.Info("update function action: " + gb.RunContext.UpdateData.Action)

	if gb.RunContext.UpdateData.Action == string(entity.TaskUpdateActionFuncSLAExpired) {
		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputFunctionDecision], TimeoutDecision)
		gb.State.TimeExpired = true
	} else {
		err := gb.setStateByResponse(&updateData)
		if err != nil {
			return err
		}
	}

	return nil
}

func (gb *ExecutableFunctionBlock) updateWithNilData(ctx context.Context, log logger.Logger) error {
	if gb.State.HasResponse {
		return nil
	}

	taskStep, errTask := gb.RunContext.Services.Storage.GetTaskStepByName(ctx, gb.RunContext.TaskID, gb.Name)
	if errTask != nil {
		return errTask
	}

	if gb.State.Async {
		err := gb.sendUpdateNotification(ctx, log)
		if err != nil {
			return err
		}
	}

	variables, err := getVariables(gb.RunContext.VarStore)
	if err != nil {
		return err
	}

	variables = script.RestoreMapStructure(variables)

	functionMapping, err := script.MapData(gb.State.Mapping, variables, nil)
	if err != nil {
		return err
	}

	err = gb.fillMapWithConstants(functionMapping)
	if err != nil {
		return err
	}

	if !gb.RunContext.skipProduce {
		err = gb.RunContext.Services.Kafka.Produce(ctx,
			&kafka.RunnerOutMessage{
				TaskID:          taskStep.ID,
				FunctionMapping: functionMapping,
				Contracts:       gb.State.Contracts,
				RetryPolicy:     string(SimpleFunctionRetryPolicy),
				FunctionName:    gb.State.Name,
				FunctionVersion: gb.State.Version,
			},
		)

		if err != nil {
			return err
		}
	}

	return nil
}

func (gb *ExecutableFunctionBlock) sendUpdateNotification(ctx context.Context, log logger.Logger) error {
	isFirstStart, firstStart, errFirstStart := gb.isFirstStart(ctx, gb.RunContext.TaskID, gb.Name)
	if errFirstStart != nil {
		return errFirstStart
	}

	// эта функция уже запускалась и время ожидания корректного ответа закончилось
	if !isFirstStart && firstStart != nil && !isTimeToWaitAnswer(firstStart.Time, gb.State.WaitCorrectRes) {
		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputFunctionDecision], TimeoutDecision)

		em, errEmail := gb.RunContext.Services.People.GetUserEmail(ctx, gb.RunContext.Initiator)
		if errEmail != nil {
			log.WithField("login", gb.RunContext.Initiator).Error(errEmail)
		}

		emails := []string{em}

		tpl := mail.NewInvalidFunctionResp(gb.RunContext.WorkNumber, gb.RunContext.NotifName, gb.RunContext.Services.Sender.SdAddress)

		filesList := []string{tpl.Image}

		files, iconEerr := gb.RunContext.GetIcons(filesList)
		if iconEerr != nil {
			return iconEerr
		}

		errSend := gb.RunContext.Services.Sender.SendNotification(ctx, emails, files, tpl)
		if errSend != nil {
			log.WithField("emails", emails).Error(errSend)
		}
	}

	return nil
}
