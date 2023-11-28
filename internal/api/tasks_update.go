package api

import (
	c "context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"go.opencensus.io/trace"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/abp/mail/pkg/email"
	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	ht "gitlab.services.mts.ru/jocasta/pipeliner/internal/human-tasks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/user"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

const headImg = "header.png"

func (ae *APIEnv) UpdateTasksByMails(w http.ResponseWriter, req *http.Request) {
	const funcName = "update_tasks_by_mails"
	ctx, s := trace.StartSpan(req.Context(), funcName)
	defer s.End()

	log := logger.GetLogger(ctx)
	log.Info(funcName, ", started")

	emails, err := ae.MailFetcher.FetchEmails(ctx)
	if err != nil {
		e := ParseMailsError
		log.WithField(funcName, "parse parsedEmails failed").Error(err)
		_ = e.sendError(w)
		return
	}

	if emails == nil {
		return
	}

	token := req.Header.Get(AuthorizationHeader)

	for i := range emails {
		usr, errGetUser := ae.People.GetUser(ctx, emails[i].Action.Login)
		if errGetUser != nil {
			log.WithField("workNumber", emails[i].Action.WorkNumber).
				WithField("login", emails[i].Action.Login).Error(errGetUser)
			continue
		}

		useInfo, errToUserinfo := usr.ToUserinfo()
		if errToUserinfo != nil {
			log.Error(errToUserinfo)
			continue
		}

		if !strings.EqualFold(useInfo.Email, emails[i].From) && !utils.IsContainsInSlice(emails[i].From, useInfo.ProxyEmails) {
			log.WithField("userEmailByLogin", useInfo.Email).
				WithField("emailFromEmail", emails[i].From).
				WithField("proxyEmails", useInfo.ProxyEmails).
				Error(errors.New("login from email not eq or not in proxyAddresses"))
			continue
		}

		for fileName, fileData := range emails[i].Action.Attachments {
			id, errSave := ae.FileRegistry.SaveFile(ctx, token, fileName, fileData.Raw)
			if errSave != nil {
				log.WithField("workNumber", emails[i].Action.WorkNumber).
					WithField("fileName", fileName).
					Error(errSave)
				continue
			}

			emails[i].Action.AttachmentsIds = append(emails[i].Action.AttachmentsIds, entity.Attachment{FileID: id})
		}

		jsonBody, errParse := json.Marshal(emails[i].Action)
		if errParse != nil {
			log.WithField("workNumber", emails[i].Action.WorkNumber).Error(errParse)
			continue
		}

		updateData := entity.TaskUpdate{
			Action:     entity.TaskUpdateAction(emails[i].Action.ActionName),
			Parameters: jsonBody,
		}

		errUpdate := ae.updateTaskBlockInternal(ctx, emails[i].Action.WorkNumber, emails[i].Action.Login, &updateData)
		if errUpdate != nil {
			log.WithField("action", *emails[i].Action).
				WithField("workNumber", emails[i].Action.WorkNumber).
				Error(errUpdate)
			continue
		}
	}

	if err = sendResponse(w, http.StatusOK, nil); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

//nolint:gocyclo //its ok here
func (ae *APIEnv) UpdateTask(w http.ResponseWriter, req *http.Request, workNumber string) {
	ctx, s := trace.StartSpan(req.Context(), "update_task")
	defer s.End()

	log := logger.GetLogger(ctx)

	if workNumber == "" {
		e := WorkNumberParsingError
		log.Error(e.errorMessage(errors.New("workNumber is empty")))
		_ = e.sendError(w)

		return
	}

	b, err := io.ReadAll(req.Body)
	defer req.Body.Close()

	if err != nil {
		e := RequestReadError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	var updateData entity.TaskUpdate
	if err = json.Unmarshal(b, &updateData); err != nil {
		e := UpdateTaskParsingError
		log.WithField("updateData", string(b)).Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	ui, err := user.GetUserInfoFromCtx(ctx)
	if err != nil {
		e := NoUserInContextError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}

	log.WithField("workNumber", workNumber).WithField("login", ui.Username).
		WithField("body", string(b)).Info("updating block")

	if updateData.IsApplicationAction() {
		err = ae.updateTaskInternal(ctx, workNumber, ui.Username, &updateData)
	} else {
		err = ae.updateTaskBlockInternal(ctx, workNumber, ui.Username, &updateData)
	}

	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if err = sendResponse(w, http.StatusOK, nil); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
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

func (ae *APIEnv) updateStepInternal(ctx c.Context, data updateStepData) bool {
	log := logger.GetLogger(ctx)

	txStorage, transactionErr := ae.DB.StartTransaction(ctx)
	if transactionErr != nil {
		log.WithError(transactionErr).Error("couldn't set update step")
		return false
	}

	defer func() {
		if r := recover(); r != nil {
			log = log.WithField("funcName", "updateStepInternal").
				WithField("panic handle", true)
			log.Error(r)
			if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
				log.WithError(errors.New("couldn't rollback tx")).
					Error(txErr)
			}
		}
	}()

	storage, getErr := txStorage.GetVariableStorageForStep(ctx, data.task.ID, data.step.Name)
	if getErr != nil {
		if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
			log.WithField("funcName", "GetVariableStorageForStep").
				WithError(errors.New("couldn't rollback tx")).
				Error(txErr)
		}
		log.WithError(getErr).Error("couldn't get block to update")
		return false
	}
	runCtx := &pipeline.BlockRunContext{
		TaskID:      data.task.ID,
		WorkNumber:  data.workNumber,
		WorkTitle:   data.task.Name,
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
			Storage:       txStorage,
		},
		BlockRunResults: &pipeline.BlockRunResults{},

		UpdateData: &script.BlockUpdateData{
			ByLogin:    data.login,
			Action:     string(data.updData.Action),
			Parameters: data.updData.Parameters,
		},

		IsTest:    data.task.IsTest,
		NotifName: data.task.Name,
	}

	blockFunc, ok := data.scenario.Pipeline.Blocks[data.step.Name]
	if !ok {
		if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
			log.WithField("funcName", "get block by name").
				WithError(errors.New("couldn't rollback tx")).
				Error(txErr)
		}
		log.WithError(errors.New("couldn't get block from pipeline")).
			Error("couldn't get block to update")
		return false
	}

	runCtx.SetTaskEvents(ctx)

	blockErr := pipeline.ProcessBlockWithEndMapping(ctx, data.step.Name, &blockFunc, runCtx, true)
	if blockErr != nil {
		if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
			log.WithField("funcName", "ProcessBlockWithEndMapping").
				WithError(errors.New("couldn't rollback tx")).
				Error(txErr)
		}
		log.WithError(blockErr).Error("couldn't update block")
		return false
	}

	if err := txStorage.CommitTransaction(ctx); err != nil {
		log.WithError(err).Error("couldn't update block, CommitTransaction")
		return false
	}

	runCtx.NotifyEvents(ctx)
	return true
}

//nolint:gocyclo // ok here
func (ae *APIEnv) updateTaskBlockInternal(ctx c.Context, workNumber, userLogin string, in *entity.TaskUpdate) (err error) {
	ctxLocal, span := trace.StartSpan(ctx, "update_task_internal")
	defer span.End()

	delegations, getDelegationsErr := ae.HumanTasks.GetDelegationsToLogin(ctxLocal, userLogin)
	if getDelegationsErr != nil {
		return getDelegationsErr
	}

	if validateErr := in.Validate(); validateErr != nil {
		return validateErr
	}

	blockTypes := getTaskStepNameByAction(in.Action)
	if len(blockTypes) == 0 {
		return errors.New("blockTypes is empty")
	}

	delegationsByApprovement := delegations.FilterByType("approvement")
	delegationsByExecution := delegations.FilterByType("execution")

	dbTask, err := ae.DB.GetTask(ctxLocal,
		delegationsByApprovement.GetUserInArrayWithDelegators([]string{userLogin}),
		delegationsByExecution.GetUserInArrayWithDelegators([]string{userLogin}),
		userLogin,
		workNumber)

	if err != nil {
		e := GetTaskError
		return errors.New(e.errorMessage(nil))
	}

	if !dbTask.IsRun() {
		e := UpdateNotRunningTaskError
		return errors.New(e.errorMessage(nil))
	}

	scenario, err := ae.DB.GetPipelineVersion(ctxLocal, dbTask.VersionID, false)
	if err != nil {
		e := GetVersionError
		return errors.New(e.errorMessage(err))
	}

	var steps entity.TaskSteps
	for _, blockType := range blockTypes {
		stepsByBlock, stepErr := ae.DB.GetUnfinishedTaskStepsByWorkIdAndStepType(ctxLocal, dbTask.ID, blockType, in)
		if stepErr != nil {
			e := GetTaskError
			return errors.New(e.errorMessage(nil))
		}
		steps = append(steps, stepsByBlock...)
	}

	if len(steps) == 0 {
		e := GetTaskError
		return errors.New(e.errorMessage(nil))
	}

	couldUpdateOne := false
	for _, item := range steps {
		success := ae.updateStepInternal(ctxLocal, updateStepData{
			scenario:    scenario,
			task:        dbTask,
			step:        item,
			updData:     in,
			delegations: delegations,
			workNumber:  workNumber,
			login:       userLogin,
		})
		if success {
			couldUpdateOne = true
		}
	}

	if !couldUpdateOne {
		e := UpdateBlockError
		return errors.New(e.errorMessage(errors.New("couldn't update work")))
	}

	return
}

func (ae *APIEnv) getAuthorAndMembersToNotify(ctx c.Context, workNumber, userLogin string) ([]string, error) {
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
		executors, approvers, executorDelegates, approverDelegates, formexec,
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

func (ae *APIEnv) updateTaskInternal(ctx c.Context, workNumber, userLogin string, in *entity.TaskUpdate) (err error) {
	ctxLocal, span := trace.StartSpan(ctx, "update_application_internal")
	defer span.End()

	log := ae.Log.WithField("mainFuncName", "updateTaskInternal")

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
		email, getUserEmailErr := ae.People.GetUserEmail(ctxLocal, login)
		if getUserEmailErr != nil {
			continue
		}
		emails = append(emails, email)
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

	err = ae.DB.StopTaskBlocks(ctxLocal, dbTask.ID)
	if err != nil {
		if txErr := txStorage.RollbackTransaction(ctxLocal); txErr != nil {
			log.WithField("funcName", "StopTaskBlocks").
				WithError(errors.New("couldn't rollback tx")).
				Error(txErr)
		}
		return err
	}

	err = ae.DB.UpdateTaskStatus(ctxLocal, dbTask.ID, db.RunStatusFinished, cancelAppParams.Comment, userLogin)
	if err != nil {
		if txErr := txStorage.RollbackTransaction(ctxLocal); txErr != nil {
			log.WithField("funcName", "UpdateTaskStatus").
				WithError(errors.New("couldn't rollback tx")).
				Error(txErr)
		}
		return err
	}

	_, err = ae.DB.UpdateTaskHumanStatus(ctxLocal, dbTask.ID, string(pipeline.StatusRevoke), "")
	if err != nil {
		if txErr := txStorage.RollbackTransaction(ctxLocal); txErr != nil {
			log.WithField("funcName", "UpdateTaskHumanStatus").
				WithError(errors.New("couldn't rollback tx")).
				Error(txErr)
		}
		return err
	}

	if commitErr := txStorage.CommitTransaction(ctxLocal); commitErr != nil {
		log.WithError(commitErr).Error("couldn't commit transaction")
		return commitErr
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

	nodeEvents, err := runCtx.GetCancelledStepsEvents(ctxLocal)
	if err != nil {
		return err
	}

	runCtx.BlockRunResults.NodeEvents = nodeEvents
	runCtx.NotifyEvents(ctxLocal)

	em := mail.NewRejectPipelineGroupTemplate(dbTask.WorkNumber, dbTask.Name, ae.Mail.SdAddress)

	file, ok := ae.Mail.Images[em.Image]
	if !ok {
		return errors.New("file not found: " + em.Image)
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

//nolint:gocyclo //its ok here
func (ae *APIEnv) RateApplication(w http.ResponseWriter, r *http.Request, workNumber string) {
	ctx, s := trace.StartSpan(r.Context(), "rate_application")
	defer s.End()

	log := logger.GetLogger(ctx)

	b, err := io.ReadAll(r.Body)
	defer r.Body.Close()

	if err != nil {
		e := RequestReadError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	req := &RateApplicationRequest{}
	if err = json.Unmarshal(b, req); err != nil {
		e := UpdateTaskParsingError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	ui, err := user.GetUserInfoFromCtx(ctx)
	if err != nil {
		e := NoUserInContextError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}

	err = ae.DB.UpdateTaskRate(ctx, &db.UpdateTaskRate{
		ByLogin:    ui.Username,
		WorkNumber: workNumber,
		Comment:    req.Comment,
		Rate:       req.Rate,
	})
	if err != nil {
		e := UpdateTaskRateError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if err = sendResponse(w, http.StatusOK, nil); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

type stoppedTask struct {
	FinishedAt time.Time `json:"finished_at"`
	Status     string    `json:"status"`
	WorkNumber string    `json:"work_number"`
	ID         uuid.UUID `json:"-"`
}

func (ae *APIEnv) StopTasks(w http.ResponseWriter, r *http.Request) {
	ctx, s := trace.StartSpan(r.Context(), "stop_tasks")
	defer s.End()

	log := logger.GetLogger(ctx)

	b, err := io.ReadAll(r.Body)
	if err != nil {
		e := RequestReadError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}
	defer r.Body.Close()

	req := &TasksStop{}
	if err = json.Unmarshal(b, req); err != nil {
		e := StopTaskParsingError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	ui, err := user.GetUserInfoFromCtx(ctx)
	if err != nil {
		e := NoUserInContextError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}

	resp := struct {
		Tasks []stoppedTask `json:"tasks"`
	}{
		Tasks: make([]stoppedTask, 0, len(req.Tasks)),
	}

	txStorage, transactionErr := ae.DB.StartTransaction(ctx)
	if transactionErr != nil {
		log.WithError(transactionErr).Error("couldn't start transaction")
		e := UnknownError
		_ = e.sendError(w)
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

	for _, workNumber := range req.Tasks {
		dbTask, getTaskErr := txStorage.GetTask(ctx, []string{ui.Username}, []string{ui.Username}, ui.Username, workNumber)
		if getTaskErr != nil {
			if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
				log.WithField("funcName", "GetTask").
					WithError(errors.New("couldn't rollback tx")).
					Error(txErr)
			}
			log.WithError(getTaskErr).Error("couldn't get task")
			continue
		}

		if dbTask.FinishedAt != nil {
			resp.Tasks = append(resp.Tasks, stoppedTask{
				FinishedAt: *dbTask.FinishedAt,
				Status:     dbTask.HumanStatus,
				WorkNumber: dbTask.WorkNumber,
				ID:         dbTask.ID,
			})
			continue
		}

		err = txStorage.StopTaskBlocks(ctx, dbTask.ID)
		if err != nil {
			if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
				log.WithField("funcName", "StopTasksBlocks").
					WithError(errors.New("couldn't rollback tx")).
					Error(txErr)
			}
			log.WithError(err).Error("couldn't stop task blocks")
			continue
		}

		err = txStorage.UpdateTaskStatus(ctx, dbTask.ID, db.RunStatusCanceled, db.CommentCanceled, ui.Username)
		if err != nil {
			if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
				log.WithField("funcName", "UpdateTaskStatus").
					WithError(errors.New("couldn't rollback tx")).
					Error(txErr)
			}
			log.WithError(err).Error("couldn't update task status")
			continue
		}

		updatedTask, updateTaskErr := txStorage.UpdateTaskHumanStatus(ctx, dbTask.ID, string(pipeline.StatusCancel), "")
		if updateTaskErr != nil {
			if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
				log.WithField("funcName", "UpdateTaskHumanStatus").
					WithError(errors.New("couldn't rollback tx")).
					Error(txErr)
			}
			log.WithError(updateTaskErr).Error("couldn't update human status")
			continue
		}

		logins, loginsErr := ae.getAuthorAndMembersToNotify(ctx, workNumber, ui.Username)
		if loginsErr != nil {
			log.WithError(loginsErr).Error("couldn't get logins")
		}

		emails := make([]string, 0, len(logins))
		for _, login := range logins {
			email, getUserEmailErr := ae.People.GetUserEmail(ctx, login)
			if getUserEmailErr != nil {
				continue
			}
			emails = append(emails, email)
		}

		em := mail.NewRejectPipelineGroupTemplate(dbTask.WorkNumber, dbTask.Name, ae.Mail.SdAddress)

		file, ok := ae.Mail.Images[em.Image]
		if !ok {
			log.Error("couldn't find images: ", em.Image)
			return
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

		resp.Tasks = append(resp.Tasks, stoppedTask{
			FinishedAt: *updatedTask.FinishedAt,
			Status:     updatedTask.HumanStatus,
			WorkNumber: updatedTask.WorkNumber,
			ID:         dbTask.ID,
		})
	}

	if err = txStorage.CommitTransaction(ctx); err != nil {
		log.WithError(err).Error("couldn't commit transaction")
		e := UnknownError
		_ = e.sendError(w)
		return
	}

	for i := range resp.Tasks {
		task := resp.Tasks[i]
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

		nodeEvents, eventErr := runCtx.GetCancelledStepsEvents(ctx)
		if eventErr != nil {
			log.WithError(eventErr).Error("couldn't get cancelled steps events")
			e := UnknownError
			_ = e.sendError(w)
			return
		}

		runCtx.BlockRunResults.NodeEvents = nodeEvents
		runCtx.NotifyEvents(ctx)
	}

	if err = sendResponse(w, http.StatusOK, resp); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}
