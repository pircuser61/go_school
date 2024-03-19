package api

import (
	c "context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/jackc/pgx/v4"

	"github.com/pkg/errors"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/mail/pkg/email"
	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/kafka"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

func (ae *Env) FunctionReturnHandler(ctx c.Context, message kafka.RunnerInMessage) error {
	log := ae.Log

	messageString, err := json.Marshal(message)
	if err != nil {
		log.WithField("taskID", message.TaskID).
			WithError(err).
			Error("error marshaling message from kafka")
	}

	log.WithField("funcName", "FunctionReturnHandler").
		WithField("message", messageString).
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

	workFinished, blockErr := pipeline.ProcessBlockWithEndMapping(ctx, step.Name, blockFunc, runCtx, true)
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

	if workFinished {
		err = ae.Scheduler.DeleteAllTasksByWorkID(ctx, step.WorkID)
		if err != nil {
			log.WithError(err).Error("failed delete all tasks by work id in scheduler")
		}
	}

	runCtx.NotifyEvents(ctx)

	log.WithField("funcName", "FunctionReturnHandler").
		WithField("message", messageString).
		Info("message from kafka successfully handled")

	return nil
}

func (ae *Env) NotifyNewFunctionVersion(w http.ResponseWriter, r *http.Request) {
	ctx, s := trace.StartSpan(r.Context(), "notify_new_function_version")
	defer s.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	var b NotifyNewFunctionVersionJSONRequestBody

	err := json.NewDecoder(r.Body).Decode(&b)
	if err != nil {
		errorHandler.handleError(http.StatusInternalServerError, err)

		return
	}

	latestFunctionVersion, err := ae.FunctionStore.GetFunction(ctx, b.FunctionId)
	if err != nil {
		errorHandler.handleError(http.StatusInternalServerError, err)

		return
	}

	dbVersions, err := ae.DB.GetVersionsByFunction(ctx, b.FunctionId, b.VersionId)
	if err != nil {
		errorHandler.handleError(http.StatusInternalServerError, err)

		return
	}

	versions := make(map[string][]script.VersionsByFunction)
	for i := range dbVersions {
		versions[dbVersions[i].Author] = append(versions[dbVersions[i].Author], script.VersionsByFunction{
			Name:   dbVersions[i].Name,
			Status: dbVersions[i].Status,
			Link:   fmt.Sprintf("%s/scenarios/%s", ae.HostURL, dbVersions[i].VersionID.String()),
		})
	}

	for login, v := range versions {
		emailToNotify, err := ae.People.GetUserEmail(ctx, login)
		if err != nil {
			log.WithField("failed to get mail for this login", login).Error(err)

			continue
		}

		em := mail.NewFunctionNotify(latestFunctionVersion.Name, latestFunctionVersion.Version, v)

		file, ok := ae.Mail.Images[em.Image]
		if !ok {
			err = errors.New("couldn't find image")
			log.Error(err.Error(), em.Image)
			errorHandler.handleError(http.StatusInternalServerError, err)

			return
		}

		files := []email.Attachment{
			{
				Name:    headImg,
				Content: file,
				Type:    email.EmbeddedAttachment,
			},
		}

		err = ae.Mail.SendNotification(ctx, []string{emailToNotify}, files, em)
		if err != nil {
			errorHandler.handleError(http.StatusInternalServerError, err)

			return
		}
	}
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

	return nil, errors.New("step by stepID not found")
}
