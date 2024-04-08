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

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/abp/mail/pkg/email"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/kafka"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

func (ae *Env) FunctionReturnHandler(ctx c.Context, message kafka.RunnerInMessage) error {
	log := ae.Log.WithField("funcName", "FunctionReturnHandler").
		WithField("stepID", message.TaskID).
		WithField("method", "kafka")

	ctx = logger.WithLogger(ctx, log)

	messageTmp, err := json.Marshal(message)
	if err != nil {
		log.WithError(err).
			Error("error marshaling message from kafka")
	}

	messageString := string(messageTmp)

	log.WithField("body", messageString).
		Info("start handle message from kafka")

	defer func() {
		if r := recover(); r != nil {
			log.WithField("funcName", "recover").
				Error(r)
		}
	}()

	st, err := ae.getTaskStepWithRetry(ctx, message.TaskID)
	if err != nil {
		log.WithField("funcName", "GetTaskStepById").
			WithError(err).
			Error("get task step by id")

		return nil
	}

	log = log.WithField("WorkNumber", st.WorkNumber).
		WithField("stepName", st.Name).
		WithField("workID", st.WorkID)
	ctx = logger.WithLogger(ctx, log)

	if st.IsPaused {
		log.Error("block is paused")

		return nil
	}

	storage := &store.VariableStore{
		State:  st.State,
		Values: st.Storage,
		Steps:  st.Steps,
		Errors: st.Errors,
	}

	functionMapping := pipeline.FunctionUpdateParams{
		Mapping: message.FunctionMapping,
		DoRetry: message.DoRetry,
	}

	mapping, err := json.Marshal(functionMapping)
	if err != nil {
		log.WithField("funcName", "json.Marshal").
			WithField("functionMapping", functionMapping).
			WithError(err).
			Error("marshal functionMapping")

		return nil
	}

	runCtx := &pipeline.BlockRunContext{
		TaskID:     st.WorkID,
		WorkNumber: st.WorkNumber,
		Initiator:  st.Initiator,
		VarStore:   storage,
		Services: pipeline.RunContextServices{
			HTTPClient:    ae.HTTPClient,
			Storage:       ae.DB,
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
		IsTest:     st.IsTest,
		Productive: true,
	}

	runCtx.SetTaskEvents(ctx)

	blockFunc, err := ae.DB.GetBlockDataFromVersion(ctx, st.WorkNumber, st.Name)
	if err != nil {
		log.WithField("funcName", "GetBlockDataFromVersion").
			WithError(err).
			Error("get block data from pipeline version")

		return nil
	}

	workFinished, blockErr := pipeline.ProcessBlockWithEndMapping(ctx, st.Name, blockFunc, runCtx, true)
	if blockErr != nil {
		return nil
	}

	if workFinished {
		err = ae.Scheduler.DeleteAllTasksByWorkID(ctx, st.WorkID)
		if err != nil {
			log.WithError(err).Error("failed delete all tasks by work id in scheduler")
		}
	}

	runCtx.NotifyEvents(ctx)

	log.WithField("funcName", "FunctionReturnHandler").
		WithField("body", messageString).
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
			log.WithField("login", login).WithError(err).Error("failed to get mail for this login")

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

		st, err := ae.DB.GetActiveTaskStepByID(ctx, stepID)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}

		if err != nil {
			return nil, err
		}

		return st, nil
	}

	return nil, errors.New("step by stepID not found")
}
