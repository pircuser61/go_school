package api

import (
	c "context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"gitlab.services.mts.ru/abp/mail/pkg/email"
	"gitlab.services.mts.ru/abp/myosotis/logger"
	"go.opencensus.io/trace"

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

	step, err := ae.DB.GetTaskStepByID(ctx, message.TaskID)
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

func (ae *Env) PostPipelinesNotifyNewFunction(w http.ResponseWriter, r *http.Request) {
	ctx, s := trace.StartSpan(r.Context(), "get_versions_by_function")
	defer s.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	data, err := io.ReadAll(r.Body)
	if err != nil {
		errorHandler.handleError(http.StatusInternalServerError, err)

		return
	}

	var b PostPipelinesNotifyNewFunctionJSONRequestBody
	err = json.Unmarshal(data, &b)
	if err != nil {
		errorHandler.handleError(http.StatusInternalServerError, err)

		return
	}

	versions, err := ae.DB.GetVersionsByFunction(ctx, *b.FunctionId)
	if err != nil {
		errorHandler.handleError(http.StatusInternalServerError, err)

		return
	}

	logins := make(map[string][]script.VersionsByFunction, 0)
	for index := range versions {
		logins[versions[index].Author] = append(logins[versions[index].Author], script.VersionsByFunction{
			Name: versions[index].Name,
			Link: fmt.Sprintf("https://dev.jocasta.mts-corp.ru/scenarios/%s", versions[index].VersionID.String()),
		})
	}

	latestFunction, err := ae.FunctionStore.GetFunction(ctx, *b.FunctionId)
	if err != nil {
		errorHandler.handleError(http.StatusInternalServerError, err)

		return
	}

	for login, version := range logins {
		emailToNotify, err := ae.People.GetUserEmail(ctx, login)
		if err != nil {
			errorHandler.handleError(http.StatusInternalServerError, err)

			return
		}

		em := mail.NewFunctionNotify("https://dev.jocasta.mts-corp.ru/funcs", latestFunction.Name, latestFunction.Version, version)

		file, ok := ae.Mail.Images[em.Image]
		if !ok {
			log.Error("couldn't find images: ", em.Image)
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
