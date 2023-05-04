package api

import (
	c "context"
	"encoding/json"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/kafka"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

func (ae *APIEnv) FunctionReturnHandler(ctx c.Context, message kafka.RunnerInMessage) error {
	log := ae.Log.WithField("step_id", message.TaskID).WithField("mainFuncName", "FunctionReturnHandler")
	ctx = logger.WithLogger(ctx, log)

	txStorage, transactionErr := ae.DB.StartTransaction(ctx)
	if transactionErr != nil {
		return transactionErr
	}

	if message.Err != "" {
		if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
			log.WithField("funcName", "message has err").
				WithError(errors.New("couldn't rollback tx")).
				Error(txErr)
		}
		log.Error(message.Err)
		return nil
	}

	step, err := ae.DB.GetTaskStepById(ctx, message.TaskID)
	if err != nil {
		if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
			log.WithField("funcName", "GetTaskStepById").
				WithError(errors.New("couldn't rollback tx")).
				Error(txErr)
		}
		log.Error(err)
		return nil
	}

	storage := &store.VariableStore{
		State:  step.State,
		Values: step.Storage,
		Steps:  step.Steps,
		Errors: step.Errors,
	}

	functionMapping := pipeline.FunctionUpdateParams{Mapping: message.FunctionMapping}

	mapping, err := json.Marshal(functionMapping)
	if err != nil {
		if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
			log.WithField("funcName", "marshal mapping").
				WithError(errors.New("couldn't rollback tx")).
				Error(txErr)
		}
		log.Error(err)
		return nil
	}

	runCtx := &pipeline.BlockRunContext{
		TaskID:     step.WorkID,
		WorkNumber: step.WorkNumber,
		Initiator:  step.Initiator,
		VarStore:   storage,

		Storage:       ae.DB,
		Sender:        ae.Mail,
		Kafka:         ae.Kafka,
		People:        ae.People,
		ServiceDesc:   ae.ServiceDesc,
		FunctionStore: ae.FunctionStore,
		HumanTasks:    ae.HumanTasks,
		Integrations:  ae.Integrations,
		FaaS:          ae.FaaS,

		UpdateData: &script.BlockUpdateData{
			Parameters: mapping,
		},
	}

	blockFunc, err := ae.DB.GetBlockDataFromVersion(ctx, step.WorkNumber, step.Name)
	if err != nil {
		if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
			log.WithField("funcName", "GetBlockDataFromVersion").
				WithError(errors.New("couldn't rollback tx")).
				Error(txErr)
		}
		log.WithError(err).Error("couldn't get block to update")
		return nil
	}

	blockErr := pipeline.ProcessBlockLogic(ctx, step.Name, blockFunc, runCtx, true)
	if blockErr != nil {
		if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
			log.WithField("funcName", "ProcessBlock").
				WithError(errors.New("couldn't rollback tx")).
				Error(txErr)
		}
		log.WithError(blockErr).Error("couldn't update block")
		return nil
	}

	log.Info("trying to commit transaction")
	if commitErr := txStorage.CommitTransaction(ctx); commitErr != nil {
		log.WithError(commitErr).Error("couldn't commit transaction")
		return commitErr
	}

	return nil
}
