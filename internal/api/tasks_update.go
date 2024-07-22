package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/mail/pkg/email"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	ht "gitlab.services.mts.ru/jocasta/pipeliner/internal/humantasks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/metrics"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/user"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

const headImg = "header.png"

func (ae *Env) UpdateTasksByMails(w http.ResponseWriter, req *http.Request) {
	const funcName = "update_tasks_by_mails"

	ctx, s := trace.StartSpan(req.Context(), funcName)
	defer s.End()

	log := logger.GetLogger(ctx).WithField("funcName", "UpdateTasksByMails")
	errorHandler := newHTTPErrorHandler(log, w)

	log.Info(funcName, ", started")

	emails, err := ae.MailFetcher.FetchEmails(ctx)
	if err != nil {
		e := ParseMailsError

		log.WithField(funcName, "parse parsedEmails failed").Error(err)

		errorHandler.sendError(e)

		return
	}

	if emails == nil {
		return
	}

	token := req.Header.Get(AuthorizationHeader)

	for i := range emails {
		log = log.WithField("workNumber", emails[i].Action.WorkNumber).
			WithField("login", emails[i].Action.Login).
			WithField("email", emails[i].From).
			WithField("action", emails[i].Action.ActionName)

		log.Info("start update task by email")

		usr, errGetUser := ae.People.GetUser(ctx, emails[i].Action.Login, true)
		if errGetUser != nil {
			log.Error(errGetUser)

			continue
		}

		useInfo, errToUserinfo := usr.ToUserinfo()
		if errToUserinfo != nil {
			log.Error(errToUserinfo)

			continue
		}

		log = log.WithField("userEmailByLogin", useInfo.Email).
			WithField("emailFromEmail", emails[i].From).
			WithField("proxyEmails", useInfo.ProxyEmails)

		if !strings.EqualFold(useInfo.Email, emails[i].From) && !utils.IsContainsInSlice(emails[i].From, useInfo.ProxyEmails) {
			log.Error(errors.New("login from email not eq or not in proxyAddresses"))

			continue
		}

		clientID, tokenParseErr := ae.getClientIDFromToken(token)
		if tokenParseErr != nil {
			log.WithError(tokenParseErr).Info("failed to get client id for file registry metrics")
		}

		log = log.WithField("clientID", clientID)

		for fileName, fileData := range emails[i].Action.Attachments {
			id, errSave := ae.FileRegistry.SaveFile(ctx, token, clientID, fileName, fileData.Raw, emails[i].Action.WorkNumber)
			if errSave != nil {
				log.WithField("fileName", fileName).Error(errSave)

				continue
			}

			emails[i].Action.AttachmentsIds = append(emails[i].Action.AttachmentsIds, entity.Attachment{FileID: id})
		}

		jsonBody, errParse := json.Marshal(emails[i].Action)
		if errParse != nil {
			log.Error(errParse)

			continue
		}

		updateData := entity.TaskUpdate{
			Action:     entity.TaskUpdateAction(emails[i].Action.ActionName),
			Parameters: jsonBody,
		}

		errUpdate := ae.updateTaskBlockInternal(ctx, emails[i].Action.WorkNumber, emails[i].Action.Login, &updateData)
		if errUpdate != nil {
			log.Error(errUpdate)

			continue
		}
	}

	err = sendResponse(w, http.StatusOK, nil)
	if err != nil {
		errorHandler.handleError(UnknownError, err)
	}
}

func (ae *Env) UpdateTask(w http.ResponseWriter, req *http.Request, workNumber string) {
	start := time.Now()
	ctx, s := trace.StartSpan(req.Context(), "update_task")

	requestInfo := metrics.NewPostRequestInfo(taskPath)

	defer func() {
		s.End()

		requestInfo.Duration = time.Since(start)

		ae.Metrics.RequestsIncrease(requestInfo)
	}()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)
	errorHandler.setMetricsRequestInfo(requestInfo)

	if workNumber == "" {
		errorHandler.handleError(ValidateWorkNumberError, ValidateWorkNumberError)
		requestInfo.Status = ValidateWorkNumberError.Status()

		return
	}

	requestInfo.WorkNumber = workNumber

	b, err := io.ReadAll(req.Body)
	defer req.Body.Close()

	if err != nil {
		errorHandler.handleError(RequestReadError, err)

		return
	}

	var updateData entity.TaskUpdate
	if err = json.Unmarshal(b, &updateData); err != nil {
		e := newHTTPErrorHandler(log.WithField("body", string(b)), w)
		e.setMetricsRequestInfo(requestInfo)
		e.handleError(UpdateTaskParsingError, err)

		return
	}

	ui, err := user.GetUserInfoFromCtx(ctx)
	if err != nil {
		errorHandler.handleError(NoUserInContextError, err)

		return
	}

	log = log.WithField("workNumber", workNumber)
	ctx = logger.WithLogger(ctx, log)

	log.WithField("body", string(b)).Info("updating block")

	err = ae.updateTask(ctx, workNumber, ui.Username, &updateData)
	if err != nil {
		httpErr := getErr(err)

		errorHandler.handleError(httpErr, err)
		requestInfo.Status = httpErr.Status()

		return
	}

	if err = sendResponse(w, http.StatusOK, nil); err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

func (ae *Env) updateTask(ctx context.Context, workNumber, userLogin string, updateData *entity.TaskUpdate) (err error) {
	switch {
	case pipeline.IsServiceAccount(userLogin) && updateData.IsSchedulerTaskUpdateAction():
		err = ae.updateTaskBlockBySchedulerRequest(ctx, workNumber, userLogin, updateData)
	case updateData.IsApplicationAction():
		err = ae.updateTaskInternal(ctx, workNumber, userLogin, updateData)
	default:
		err = ae.updateTaskBlockInternal(ctx, workNumber, userLogin, updateData)
	}

	return err
}

func (ae *Env) updateTaskBlockBySchedulerRequest(ctx context.Context, workNumber, userLogin string, in *entity.TaskUpdate) (err error) {
	ctxLocal, span := trace.StartSpan(ctx, "update_task_block_by_scheduler_request")
	defer span.End()

	log := logger.GetLogger(ctx).
		WithField("funcName", "updateTaskBlockBySchedulerRequest")

	delegations, getDelegationsErr := ae.HumanTasks.GetDelegationsToLogin(ctxLocal, userLogin)
	if getDelegationsErr != nil {
		log.Error(getDelegationsErr)

		return getDelegationsErr
	}

	if validateErr := in.Validate(); validateErr != nil {
		log.Error(validateErr)

		return validateErr
	}

	stepTypes := getTaskStepNameByAction(in.Action)
	if len(stepTypes) == 0 {
		log.Error(entity.ErrEmptyStepTypes)

		return entity.ErrEmptyStepTypes
	}

	dbTask, err := ae.GetTaskForUpdate(ctxLocal, workNumber)
	if err != nil {
		log.Error(err)

		return GetTaskError.Join(err)
	}

	if !dbTask.IsRun() {
		log.Info("db task is not running, exit with nil error")

		return nil
	}

	scenario, err := ae.DB.GetPipelineVersion(ctxLocal, dbTask.VersionID, false)
	if err != nil {
		log.Error(err)

		return GetVersionError.Join(err)
	}

	var steps entity.TaskSteps

	for _, stepType := range stepTypes {
		stepsByBlock, stepErr := ae.DB.GetUnfinishedTaskSteps(ctxLocal, &entity.GetUnfinishedTaskSteps{
			ID:        dbTask.ID,
			StepType:  stepType,
			Action:    in.Action,
			StepNames: in.StepNames,
		})
		if stepErr != nil {
			log.Error(stepErr)

			return GetTaskError.Join(stepErr)
		}

		steps = append(steps, stepsByBlock...)
	}

	if len(steps) == 0 {
		log.Info("zero length unfinished task steps, exit with nil error")

		return nil
	}

	couldUpdateOne := false

	for _, item := range steps {
		success := ae.updateStepInternal(
			ctxLocal,
			&updateStepData{
				scenario:    scenario,
				task:        dbTask,
				step:        item,
				updData:     in,
				delegations: delegations,
				workNumber:  workNumber,
				login:       userLogin,
			},
		)
		if success {
			couldUpdateOne = true
		}
	}

	if !couldUpdateOne {
		log.Error(UpdateBlockError.JoinString("couldn't update work"))

		return UpdateBlockError.JoinString("couldn't update work")
	}

	return nil
}

func (ae *Env) updateTaskBlockInternal(ctx context.Context, workNumber, userLogin string, in *entity.TaskUpdate) (err error) {
	ctxLocal, span := trace.StartSpan(ctx, "update_task_block_internal")
	defer span.End()

	log := logger.GetLogger(ctx).WithField("funcName", "updateTaskBlockInternal")

	delegations, getDelegationsErr := ae.HumanTasks.GetDelegationsToLogin(ctxLocal, userLogin)
	if getDelegationsErr != nil {
		log.WithField("funcName", "GetDelegationsToLogin").Error(getDelegationsErr)

		return getDelegationsErr
	}

	if validateErr := in.Validate(); validateErr != nil {
		log.Error(validateErr)

		return validateErr
	}

	stepTypes := getTaskStepNameByAction(in.Action)
	if len(stepTypes) == 0 {
		log.Error(entity.ErrEmptyStepTypes)

		return entity.ErrEmptyStepTypes
	}

	dbTask, err := ae.GetTaskForUpdate(ctxLocal, workNumber)
	if err != nil {
		log.WithField("funcName", "GetTaskForUpdate").Error(err)

		return GetTaskError.Join(err)
	}

	log = log.WithField("workID", dbTask.ID).
		WithField("versionID", dbTask.VersionID)

	if !dbTask.IsRun() {
		log.Error(UpdateNotRunningTaskError.JoinString("task is not running"))

		return UpdateNotRunningTaskError.JoinString("task is not running")
	}

	scenario, err := ae.DB.GetPipelineVersion(ctxLocal, dbTask.VersionID, false)
	if err != nil {
		log.Error(err)

		return GetVersionError.Join(err)
	}

	var steps entity.TaskSteps

	for _, stepType := range stepTypes {
		stepsByBlock, stepErr := ae.DB.GetUnfinishedTaskSteps(ctxLocal, &entity.GetUnfinishedTaskSteps{
			ID:        dbTask.ID,
			StepType:  stepType,
			Action:    in.Action,
			StepNames: in.StepNames,
		})
		if stepErr != nil {
			log.Error(stepErr)

			return GetTaskError.Join(stepErr)
		}

		steps = append(steps, stepsByBlock...)
	}

	if len(steps) == 0 {
		log.Error(GetTaskError.JoinString("zero length task steps"))

		return GetTaskError.JoinString("zero length task steps")
	}

	couldUpdateOne := false

	ctxLocal = logger.WithLogger(ctxLocal, log)

	for _, item := range steps {
		success := ae.updateStepInternal(
			ctxLocal,
			&updateStepData{
				scenario:    scenario,
				task:        dbTask,
				step:        item,
				updData:     in,
				delegations: delegations,
				workNumber:  workNumber,
				login:       userLogin,
			},
		)
		if success {
			couldUpdateOne = true
		}
	}

	if !couldUpdateOne {
		log.Error(GetTaskError.JoinString("couldn't update work"))

		return UpdateBlockError.JoinString("couldn't update work")
	}

	return nil
}

type updateStepData struct {
	scenario    *entity.EriusScenario
	task        *entity.EriusTask
	step        *entity.Step
	updData     *entity.TaskUpdate
	delegations ht.Delegations
	workNumber  string
	login       string
}

func (ae *Env) updateStepInternal(ctx context.Context, data *updateStepData) bool {
	log := logger.GetLogger(ctx).WithField("funcName", "updateStepInternal")

	defer func() {
		if r := recover(); r != nil {
			log = log.WithField("funcName", "updateStepInternal").
				WithField("panic handle", true)
			log.Error(r)
		}
	}()

	storage, getErr := ae.DB.GetVariableStorageForStep(ctx, data.task.ID, data.step.Name)
	if getErr != nil {
		log.WithError(getErr).Error("couldn't get block to update")

		return false
	}

	runCtx := &pipeline.BlockRunContext{
		TaskID:      data.task.ID,
		WorkNumber:  data.workNumber,
		WorkTitle:   data.task.Name,
		PipelineID:  data.scenario.PipelineID,
		VersionID:   data.scenario.VersionID,
		Initiator:   data.task.Author,
		VarStore:    storage,
		Delegations: data.delegations,

		Services: pipeline.RunContextServices{
			HTTPClient:    ae.HTTPClient,
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
			Storage:       ae.DB,
			JocastaURL:    ae.HostURL,
		},
		BlockRunResults: &pipeline.BlockRunResults{},

		UpdateData: &script.BlockUpdateData{
			ByLogin:    data.login,
			Action:     string(data.updData.Action),
			Parameters: data.updData.Parameters,
		},

		IsTest:     data.task.IsTest,
		NotifName:  data.task.Name,
		Productive: true,
	}

	blockFunc, ok := data.scenario.Pipeline.Blocks[data.step.Name]
	if !ok {
		log.
			WithError(errors.New("couldn't get block from pipeline")).
			Error("couldn't get block to update")

		return false
	}

	runCtx.SetTaskEvents(ctx)

	_, workFinished, blockErr := pipeline.ProcessBlockWithEndMapping(ctx, data.step.Name, blockFunc, runCtx, true)
	if blockErr != nil {
		log.
			WithError(blockErr).
			Error("couldn't process block with end mapping")

		runCtx.NotifyEvents(ctx) // events for successfully processed nodes

		return false
	}

	if workFinished {
		err := ae.Scheduler.DeleteAllTasksByWorkID(ctx, data.task.ID)
		if err != nil {
			log.WithError(err).Error("failed delete all tasks by work id in scheduler")
		}
	}

	runCtx.NotifyEvents(ctx)

	return true
}

func (ae *Env) getAuthorAndMembersToNotify(ctx context.Context, workNumber, userLogin string) ([]string, error) {
	taskMembers, err := ae.DB.GetTaskMembers(ctx, workNumber, true)
	if err != nil {
		return nil, err
	}

	executors := make([]string, 0, len(taskMembers))
	approvers := make([]string, 0, len(taskMembers))
	formexec := make([]string, 0, len(taskMembers))

	for _, m := range taskMembers {
		switch m.Type {
		case "execution":
			executors = append(executors, m.Login)
		case "approver":
			approvers = append(approvers, m.Login)
		case "form":
			formexec = append(formexec, m.Login)
		}
	}

	execDelegates, getDelegatesErr := ae.HumanTasks.GetDelegationsByLogins(ctx, executors)
	if getDelegatesErr != nil {
		return nil, getDelegatesErr
	}

	execDelegates = execDelegates.FilterByType("execution")
	executorDelegates := (&execDelegates).GetUniqueLogins()

	apprDelegates, getDelegatesErr := ae.HumanTasks.GetDelegationsByLogins(ctx, approvers)
	if getDelegatesErr != nil {
		return nil, getDelegatesErr
	}

	apprDelegates = apprDelegates.FilterByType("approvement")
	approverDelegates := (&execDelegates).GetUniqueLogins()

	uniquePeople := make(map[string]struct{})

	peopleGroups := [][]string{
		executors,
		approvers,
		executorDelegates,
		approverDelegates,
		formexec,
	}

	uniquePeople[userLogin] = struct{}{}

	for _, g := range peopleGroups {
		for _, p := range g {
			uniquePeople[p] = struct{}{}
		}
	}

	res := make([]string, 0, len(uniquePeople))
	for k := range uniquePeople {
		res = append(res, k)
	}

	return res, nil
}

// nolint:gocyclo // ok
func (ae *Env) updateTaskInternal(ctx context.Context, workNumber, userLogin string, in *entity.TaskUpdate) (err error) {
	ctxLocal, span := trace.StartSpan(ctx, "update_task_internal")
	defer span.End()

	log := ae.Log.WithField("funcName", "updateTaskInternal")

	dbTask, err := ae.DB.GetTask(ctxLocal, []string{userLogin}, []string{userLogin}, userLogin, workNumber)
	if err != nil {
		return err
	}

	if dbTask.FinishedAt != nil {
		return errors.New("task is already finished")
	}

	if dbTask.Author != userLogin {
		return errors.New("you have no access for this action")
	}

	logins, err := ae.getAuthorAndMembersToNotify(ctxLocal, workNumber, userLogin)
	if err != nil {
		return err
	}

	emails := make([]string, 0, len(logins))

	for _, login := range logins {
		userEmail, getUserEmailErr := ae.People.GetUserEmail(ctxLocal, login)
		if getUserEmailErr != nil {
			continue
		}

		emails = append(emails, userEmail)
	}

	cancelAppParams := entity.CancelAppParams{}
	if err = json.Unmarshal(in.Parameters, &cancelAppParams); err != nil {
		return errors.New("can't assert provided data")
	}

	txStorage, transactionErr := ae.DB.StartTransaction(ctx)
	if transactionErr != nil {
		return transactionErr
	}

	defer func() {
		if r := recover(); r != nil {
			log = log.WithField("funcName", "updateTaskInternal").
				WithField("panic handle", true)
			log.Error(r)

			if txErr := txStorage.RollbackTransaction(ctxLocal); txErr != nil {
				log.WithError(errors.New("couldn't rollback tx")).
					Error(txErr)
			}
		}
	}()

	err = ae.stopTaskBlocks(ctx, dbTask, cancelAppParams, userLogin)
	if err != nil {
		if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
			log.WithField("funcName", "Env.updateTasks").
				WithError(errors.New("couldn't rollback tx")).
				Error(txErr)
		}

		return err
	}

	params := struct {
		Steps []string `json:"steps"`
	}{Steps: []string{}}

	jsonParams, err := json.Marshal(params)
	if err != nil {
		log.Error(err)
	}

	_, err = txStorage.CreateTaskEvent(ctx, &entity.CreateTaskEvent{
		WorkID:    dbTask.ID.String(),
		EventType: "pause",
		Author:    userLogin,
		Params:    jsonParams,
	})
	if err != nil {
		if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
			log.WithField("funcName", "CreateTaskEvent").
				WithError(errors.New("couldn't rollback tx")).
				Error(txErr)
		}

		return err
	}

	if commitErr := txStorage.CommitTransaction(ctxLocal); commitErr != nil {
		log.WithError(commitErr).Error("couldn't commit transaction")

		return commitErr
	}

	err = ae.Scheduler.DeleteAllTasksByWorkID(ctx, dbTask.ID)
	if err != nil {
		log.WithError(err).Error("failed delete all tasks by work id in scheduler")
	}

	runCtx := pipeline.BlockRunContext{
		WorkNumber: workNumber,
		TaskID:     dbTask.ID,
		Services: pipeline.RunContextServices{
			HTTPClient:   ae.HTTPClient,
			Integrations: ae.Integrations,
			Storage:      ae.DB,
		},
		BlockRunResults: &pipeline.BlockRunResults{},
	}

	runCtx.SetTaskEvents(ctx)

	nodeEvents, nodeKafkaEvents, err := runCtx.GetCancelledStepsEvents(ctxLocal)
	if err != nil {
		return err
	}

	runCtx.BlockRunResults.NodeEvents = nodeEvents
	runCtx.BlockRunResults.NodeKafkaEvents = nodeKafkaEvents
	runCtx.NotifyEvents(ctxLocal)

	em := mail.NewRejectPipelineGroupTemplate(dbTask.WorkNumber, dbTask.Name, ae.Mail.SdAddress)

	file, ok := ae.Mail.Images[em.Image]
	if !ok {
		log.Error("couldn't find images: ", em.Image)

		return nil
	}

	files := []email.Attachment{
		{
			Name:    headImg,
			Content: file,
			Type:    email.EmbeddedAttachment,
		},
	}

	err = ae.Mail.SendNotification(ctxLocal, emails, files, em)
	if err != nil {
		return err
	}

	return nil
}

func (ae *Env) stopTaskBlocks(
	ctx context.Context,
	dbTask *entity.EriusTask,
	cancelAppParams entity.CancelAppParams,
	userLogin string,
) error {
	err := ae.DB.StopTaskBlocks(ctx, dbTask.ID)
	if err != nil {
		return fmt.Errorf("failed StopTasksBlocks, err: %w", err)
	}

	err = ae.DB.UpdateTaskStatus(ctx, dbTask.ID, db.RunStatusFinished, cancelAppParams.Comment, userLogin)
	if err != nil {
		return fmt.Errorf("failed UpdateTaskStatus, err: %w", err)
	}

	_, err = ae.DB.UpdateTaskHumanStatus(ctx, dbTask.ID, string(pipeline.StatusRevoke), "")
	if err != nil {
		return fmt.Errorf("failed UpdateTaskHumanStatus, err: %w", err)
	}

	err = ae.DB.SetTaskPaused(ctx, dbTask.ID.String(), false)
	if err != nil {
		return fmt.Errorf("failed UnpauseTask, err: %w", err)
	}

	return nil
}

func (ae *Env) RateApplication(w http.ResponseWriter, r *http.Request, workNumber string) {
	ctx, s := trace.StartSpan(r.Context(), "rate_application")
	defer s.End()

	log := logger.GetLogger(ctx).
		WithField(script.MainFuncName, "RateApplication").
		WithField(script.Method, script.MethodGet).
		WithField(script.Transport, script.TransportREST).
		WithField(script.TraceID, s.SpanContext().TraceID.String()).
		WithField(script.LogVersion, "v1")
	errorHandler := newHTTPErrorHandler(log, w)

	b, err := io.ReadAll(r.Body)
	defer r.Body.Close()

	if err != nil {
		log.Error(err)
		errorHandler.handleError(RequestReadError, err)

		return
	}

	log = log.WithField("body", string(b))

	req := &RateApplicationRequest{}
	if err = json.Unmarshal(b, req); err != nil {
		log.Error(err)
		errorHandler.handleError(UpdateTaskParsingError, err)

		return
	}

	ui, err := user.GetUserInfoFromCtx(ctx)
	if err != nil {
		log.Error(err)
		errorHandler.handleError(NoUserInContextError, err)

		return
	}

	err = ae.DB.UpdateTaskRate(ctx, &db.UpdateTaskRate{
		ByLogin:    ui.Username,
		WorkNumber: workNumber,
		Comment:    req.Comment,
		Rate:       req.Rate,
	})
	if err != nil {
		log.Error(err)
		errorHandler.handleError(UpdateTaskRateError, err)

		return
	}

	if err = sendResponse(w, http.StatusOK, nil); err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

type stoppedTask struct {
	FinishedAt time.Time `json:"finished_at"`
	Status     string    `json:"status"`
	WorkNumber string    `json:"work_number"`
	ID         uuid.UUID `json:"-"`
}

type stoppedTasks struct {
	Tasks []stoppedTask `json:"tasks"`
}

const stopTasksPath = "/tasks/stop"

func (ae *Env) StopTasks(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ctx, s := trace.StartSpan(r.Context(), "stop_tasks")

	requestInfo := metrics.NewPostRequestInfo(stopTasksPath)

	defer func() {
		s.End()

		requestInfo.Duration = time.Since(start)

		ae.Metrics.RequestsIncrease(requestInfo)
	}()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)
	errorHandler.setMetricsRequestInfo(requestInfo)

	b, err := io.ReadAll(r.Body)
	if err != nil {
		errorHandler.handleError(RequestReadError, err)

		return
	}

	defer r.Body.Close()

	req := &TasksStop{}
	if err = json.Unmarshal(b, req); err != nil {
		errorHandler.handleError(StopTaskParsingError, err)

		return
	}

	if len(req.Tasks) == 0 {
		errorHandler.handleError(ValidateTasksError, ValidateTasksError)
		requestInfo.Status = ValidateTasksError.Status()

		return
	}

	ui, err := user.GetUserInfoFromCtx(ctx)
	if err != nil {
		errorHandler.handleError(NoUserInContextError, err)

		return
	}

	resp := stoppedTasks{
		Tasks: make([]stoppedTask, 0, len(req.Tasks)),
	}

	txStorage, transactionErr := ae.DB.StartTransaction(ctx)
	if transactionErr != nil {
		log.WithError(transactionErr).Error("couldn't start transaction")
		errorHandler.sendError(UnknownError)

		return
	}

	defer func() {
		if r := recover(); r != nil {
			log = log.WithField("funcName", "StopTasks").
				WithField("panic handle", true)
			log.Error(r)

			if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
				log.WithError(errors.New("couldn't rollback tx")).
					Error(txErr)
			}
		}
	}()

	var stoppedTasksCnt int

	for _, workNumber := range req.Tasks {
		requestInfo.WorkNumber = workNumber

		updateErr := ae.updateTaskByWorkNumber(ctx, txStorage, ui, workNumber, &resp)
		if updateErr != nil {
			if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
				log.WithField("funcName", "updateTaskByWorkNumber").
					WithError(errors.New("couldn't rollback tx")).
					Error(txErr)
			}

			log.WithError(updateErr).Error("couldn't update human status")

			continue
		}

		stoppedTasksCnt++

		workID, errGetID := ae.DB.GetWorkIDByWorkNumber(ctx, workNumber)
		if errGetID != nil {
			log.WithError(errGetID).Error("couldn't update get work id by number: " + workNumber)

			continue
		}

		params := struct {
			Steps []string `json:"steps"`
		}{Steps: []string{}}

		jsonParams, mrshErr := json.Marshal(params)
		if mrshErr != nil {
			log.Error(mrshErr)
		}

		_, err = txStorage.CreateTaskEvent(ctx, &entity.CreateTaskEvent{
			WorkID:    workID.String(),
			EventType: "pause",
			Author:    ui.Username,
			Params:    jsonParams,
		})
		if err != nil {
			log.WithError(err).Error("couldn't create task event pause: " + workNumber)
		}
	}

	if stoppedTasksCnt == 0 {
		errorHandler.handleError(UpdateTaskError, UpdateTaskError)
		requestInfo.Status = UpdateTaskError.Status()

		return
	}

	if err = txStorage.CommitTransaction(ctx); err != nil {
		log.WithError(err).Error("couldn't commit transaction")
		errorHandler.handleError(UnknownError, err)

		return
	}

	for _, task := range resp.Tasks {
		err = ae.Scheduler.DeleteAllTasksByWorkID(ctx, task.ID)
		if err != nil {
			log.WithError(err).Error("failed delete all tasks by work id in scheduler")
		}
	}

	err = ae.processTasks(ctx, resp.Tasks)
	if err != nil {
		log.WithError(err).Error("failed process stopped tasks")
		errorHandler.sendError(UnknownError)

		return
	}

	err = sendResponse(w, http.StatusOK, resp)
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

func (ae *Env) updateTaskByWorkNumber(
	ctx context.Context,
	txStorage db.Database,
	ui *sso.UserInfo,
	workNumber string,
	tasks *stoppedTasks,
) error {
	log := logger.GetLogger(ctx)

	dbTask, getTaskErr := txStorage.GetTask(ctx, []string{ui.Username}, []string{ui.Username}, ui.Username, workNumber)
	if getTaskErr != nil {
		log.WithError(getTaskErr).Error("couldn't get task")

		return getTaskErr
	}

	if dbTask.FinishedAt != nil {
		tasks.Tasks = append(
			tasks.Tasks,
			stoppedTask{
				FinishedAt: *dbTask.FinishedAt,
				Status:     dbTask.HumanStatus,
				WorkNumber: dbTask.WorkNumber,
				ID:         dbTask.ID,
			},
		)

		return nil
	}

	err := txStorage.StopTaskBlocks(ctx, dbTask.ID)
	if err != nil {
		log.WithError(err).Error("couldn't stop task blocks")

		return err
	}

	err = txStorage.UpdateTaskStatus(ctx, dbTask.ID, db.RunStatusCanceled, db.CommentCanceled, ui.Username)
	if err != nil {
		log.WithError(err).Error("couldn't update task status")

		return err
	}

	updatedTask, updateTaskErr := txStorage.UpdateTaskHumanStatus(ctx, dbTask.ID, string(pipeline.StatusCancel), "")
	if updateTaskErr != nil {
		log.WithError(updateTaskErr).Error("couldn't update human status")

		return updateTaskErr
	}

	logins, loginsErr := ae.getAuthorAndMembersToNotify(ctx, workNumber, ui.Username)
	if loginsErr != nil {
		log.WithError(loginsErr).Error("couldn't get logins")
	}

	emails := make([]string, 0, len(logins))

	for _, login := range logins {
		userEmail, getUserEmailErr := ae.People.GetUserEmail(ctx, login)
		if getUserEmailErr != nil {
			continue
		}

		emails = append(emails, userEmail)
	}

	em := mail.NewRejectPipelineGroupTemplate(dbTask.WorkNumber, dbTask.Name, ae.Mail.SdAddress)

	file, ok := ae.Mail.Images[em.Image]
	if !ok {
		log.Error("couldn't find images: ", em.Image)

		return fmt.Errorf("couldn't find images: %s", em.Image)
	}

	files := []email.Attachment{
		{
			Name:    headImg,
			Content: file,
			Type:    email.EmbeddedAttachment,
		},
	}

	sendNotifErr := ae.Mail.SendNotification(ctx, emails, files, em)
	if sendNotifErr != nil {
		log.WithError(sendNotifErr).Error("couldn't send notification")
	}

	err = ae.Scheduler.DeleteAllTasksByWorkID(ctx, dbTask.ID)
	if err != nil {
		log.WithError(err).Error("failed delete all tasks by work id in scheduler")
	}

	tasks.Tasks = append(
		tasks.Tasks,
		stoppedTask{
			FinishedAt: *updatedTask.FinishedAt,
			Status:     updatedTask.HumanStatus,
			WorkNumber: updatedTask.WorkNumber,
			ID:         dbTask.ID,
		},
	)

	return nil
}

func (ae *Env) processTasks(ctx context.Context, stoppedTasks []stoppedTask) error {
	const maxSyncTasksCount = 1
	if len(stoppedTasks) > maxSyncTasksCount {
		return ae.processTasksAsync(ctx, stoppedTasks)
	}

	return ae.processTasksSync(ctx, stoppedTasks)
}

func (ae *Env) processTasksSync(ctx context.Context, stoppedTasks []stoppedTask) error {
	for _, task := range stoppedTasks {
		task := task

		err := ae.processSingleTask(ctx, &task)
		if err != nil {
			return err
		}
	}

	return nil
}

func (ae *Env) processTasksAsync(ctx context.Context, stoppedTasks []stoppedTask) error {
	var errgr errgroup.Group

	for _, task := range stoppedTasks {
		task := task

		errgr.Go(
			func() error {
				return ae.processSingleTask(ctx, &task)
			},
		)
	}

	return errgr.Wait()
}

func (ae *Env) processSingleTask(ctx context.Context, task *stoppedTask) error {
	runCtx := pipeline.BlockRunContext{
		WorkNumber: task.WorkNumber,
		TaskID:     task.ID,
		Services: pipeline.RunContextServices{
			HTTPClient:   ae.HTTPClient,
			Integrations: ae.Integrations,
			Storage:      ae.DB,
		},
		BlockRunResults: &pipeline.BlockRunResults{},
	}

	runCtx.SetTaskEvents(ctx)

	nodeEvents, nodeKafkaEvents, err := runCtx.GetCancelledStepsEvents(ctx)
	if err != nil {
		return err
	}

	runCtx.BlockRunResults.NodeEvents = nodeEvents
	runCtx.BlockRunResults.NodeKafkaEvents = nodeKafkaEvents
	runCtx.NotifyEvents(ctx)

	return nil
}
