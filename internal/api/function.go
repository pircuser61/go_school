package api

import (
	c "context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/jackc/pgx/v4"

	"github.com/pkg/errors"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/kafka"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

func (ae *Env) FunctionReturnHandler(ctx c.Context, message kafka.RunnerInMessage) error {
	log := ae.Log
	log.WithField("funcName", "FunctionReturnHandler").
		WithField("message", message).
		Info("start handle message from kafka")

	ctx = logger.WithLogger(ctx, log)

	txStorage, transactionErr := ae.DB.StartTransaction(ctx)
	if transactionErr != nil {
		log.WithField("funcName", "DB.StartTransaction").
			WithError(transactionErr).
			Error("start transaction")

		return transactionErr
	}

	defer func() {
		if r := recover(); r != nil {
			log.WithField("funcName", "recover").
				Error(r)

			if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
				log.WithField("funcName", "RollbackTransaction").
					WithError(txErr).
					Error("rollback transaction")
			}
		}
	}()

	if message.Err != "" {
		log.WithField("message.Err", message.Err).
			Error("message from kafka has error")

		if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
			log.WithField("funcName", "RollbackTransaction").
				WithError(txErr).
				Error("rollback transaction")
		}

		return nil
	}

	step, err := ae.getTaskStepWithRetry(ctx, message.TaskID)
	if err != nil {
		log.WithField("funcName", "GetTaskStepById").
			WithError(err).
			Error("get task step by id")

		if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
			log.WithField("funcName", "RollbackTransaction").
				WithError(txErr).
				Error("rollback transaction")
		}

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
		log.WithField("funcName", "json.Marshal").
			WithField("functionMapping", functionMapping).
			WithError(err).
			Error("marshal functionMapping")

		if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
			log.WithField("funcName", "RollbackTransaction").
				WithError(txErr).
				Error("rollback transaction")
		}

		return nil
	}

	runCtx := &pipeline.BlockRunContext{
		TaskID:     step.WorkID,
		WorkNumber: step.WorkNumber,
		Initiator:  step.Initiator,
		VarStore:   storage,
		Services: pipeline.RunContextServices{
			HTTPClient:    ae.HTTPClient,
			Storage:       txStorage,
			Sender:        ae.Mail,
			Kafka:         ae.Kafka,
			People:        ae.People,
			ServiceDesc:   ae.ServiceDesc,
			FunctionStore: ae.FunctionStore,
			HumanTasks:    ae.HumanTasks,
			Integrations:  ae.Integrations,
			FileRegistry:  ae.FileRegistry,
			FaaS:          ae.FaaS,
			HrGate:        ae.HrGate,
			Scheduler:     ae.Scheduler,
			SLAService:    ae.SLAService,
		},
		BlockRunResults: &pipeline.BlockRunResults{},

		UpdateData: &script.BlockUpdateData{
			Parameters: mapping,
		},
		IsTest: step.IsTest,
	}

	runCtx.SetTaskEvents(ctx)

	blockFunc, err := ae.DB.GetBlockDataFromVersion(ctx, step.WorkNumber, step.Name)
	if err != nil {
		log.WithField("funcName", "GetBlockDataFromVersion").
			WithField("step.WorkNumber", step.WorkNumber).
			WithField("step.Name", step.Name).
			WithError(err).
			Error("get block data from pipeline version")

		if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
			log.WithField("funcName", "RollbackTransaction").
				WithError(txErr).
				Error("rollback transaction")
		}

		return nil
	}

	blockErr := pipeline.ProcessBlockWithEndMapping(ctx, step.Name, blockFunc, runCtx, true)
	if blockErr != nil {
		log.WithField("funcName", "ProcessBlockWithEndMapping").
			WithField("step.WorkNumber", step.WorkNumber).
			WithField("step.Name", step.Name).
			WithError(blockErr).
			Error("process block with end mapping")

		if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
			log.WithField("funcName", "RollbackTransaction").
				WithError(txErr).
				Error("rollback transaction")
		}

		return nil
	}

	if commitErr := txStorage.CommitTransaction(ctx); commitErr != nil {
		log.WithField("funcName", "CommitTransaction").
			WithError(commitErr).
			Error("commit transaction")

		return commitErr
	}

	runCtx.NotifyEvents(ctx)

	log.WithField("funcName", "FunctionReturnHandler").
		WithField("message", message).
		Info("message from kafka successfully handled")

	return nil
}

const (
	getTaskStepTimeout    = 2
	getTaskStepRetryCount = 5
)

func (ae *Env) getTaskStepWithRetry(ctx c.Context, stepID uuid.UUID) (*entity.Step, error) {
	for i := 0; i < getTaskStepRetryCount; i++ {
		<-time.After(getTaskStepTimeout * time.Second)

		step, err := ae.DB.GetTaskStepByID(ctx, stepID)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}

		if err != nil {
			return nil, err
		}

		return step, nil
	}

	return nil, errors.New("step by stepId not found")
}
