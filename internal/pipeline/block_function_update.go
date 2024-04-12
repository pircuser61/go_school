package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	"github.com/pkg/errors"
	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/kafka"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

func (gb *ExecutableFunctionBlock) updateFunctionResult(ctx context.Context, log logger.Logger) error {
	var updateData FunctionUpdateParams

	log.Info("update function action: " + gb.RunContext.UpdateData.Action)

	if gb.RunContext.UpdateData.Action == string(entity.TaskUpdateActionReload) {
		if !gb.State.Started {
			err := gb.runFunction(ctx, log)
			if err != nil {
				return fmt.Errorf("failed run function, %w", err)
			}
		}

		return nil
	}

	updateDataUnmarshalErr := json.Unmarshal(gb.RunContext.UpdateData.Parameters, &updateData)
	if updateDataUnmarshalErr != nil {
		return updateDataUnmarshalErr
	}

	if gb.RunContext.UpdateData.Action == string(entity.TaskUpdateActionFuncSLAExpired) {
		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputFunctionDecision], TimeoutDecision)
		gb.State.TimeExpired = true

		return nil
	}

	if gb.RunContext.UpdateData.Action == string(entity.TaskUpdateActionRetry) {
		if gb.State.CurrRetryCount >= gb.State.RetryCount {
			return errors.New("retry count exceeded")
		}

		err := gb.runFunction(ctx, log)
		if err != nil {
			return err
		}

		gb.State.CurrRetryCount++
		gb.State.RetryTimeouts = append(gb.State.RetryTimeouts, gb.State.CurrRetryTimeout)

		gb.updateRetryTimeout()

		return nil
	}

	err := gb.setStateByResponse(ctx, log, &updateData)

	return err
}

func (gb *ExecutableFunctionBlock) updateRetryTimeout() {
	switch gb.State.RetryPolicy {
	case script.FunctionRetryPolicySimple:
		return
	case script.FunctionRetryPolicyFibonacci:
		prevRetryTimeout := 0
		if len(gb.State.RetryTimeouts) > 1 {
			prevRetryTimeout = gb.State.RetryTimeouts[len(gb.State.RetryTimeouts)-2]
		}

		gb.State.CurrRetryTimeout += prevRetryTimeout

		return
	case script.FunctionRetryPolicyExponential:
		gb.State.CurrRetryTimeout = int(math.Pow(float64(gb.State.RetryTimeouts[0]), float64(gb.State.CurrRetryCount+1)))

		return
	}
}

func (gb *ExecutableFunctionBlock) runFunction(ctx context.Context, log logger.Logger) error {
	if gb.State.HasResponse {
		return nil
	}

	gb.State.Started = true

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

	err = script.FillFuncMapWithConstants(gb.State.Constants, functionMapping)
	if err != nil {
		return err
	}

	jsonString, err := json.Marshal(functionMapping)
	if err != nil {
		return err
	}

	schema := gb.State.GetSchema()
	if validErr := script.ValidateJSONByJSONSchema(string(jsonString), schema); validErr != nil {
		return validErr
	}

	if !gb.RunContext.skipProduce {
		err = gb.RunContext.Services.Kafka.ProduceFuncMessage(ctx,
			&kafka.RunnerOutMessage{
				TaskID:          taskStep.ID,
				PipelineID:      gb.RunContext.PipelineID,
				VersionID:       gb.RunContext.VersionID,
				ClientID:        gb.RunContext.ClientID,
				WorkNumber:      gb.RunContext.WorkNumber,
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
