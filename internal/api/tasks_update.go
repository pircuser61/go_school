package api

import (
	"bytes"
	c "context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/user"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

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

		for fileName := range emails[i].Action.Attachments {
			r := bytes.NewReader(emails[i].Action.Attachments[fileName].Raw)
			ext := emails[i].Action.Attachments[fileName].Ext
			id, errSave := ae.Minio.SaveFile(ctx, ext, fileName, r, r.Size())
			if errSave != nil {
				log.WithField("workNumber", emails[i].Action.WorkNumber).
					WithField("fileName", fileName).
					Error(errSave)
				continue
			}

			emails[i].Action.AttachmentsIds = append(emails[i].Action.AttachmentsIds, id)
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

		errUpdate := ae.updateTaskInternal(ctx, emails[i].Action.WorkNumber, emails[i].Action.Login, &updateData)
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

	if updateData.IsApplicationAction() {
		err = ae.updateApplicationInternal(ctx, workNumber, ui.Username, &updateData)
	} else {
		err = ae.updateTaskInternal(ctx, workNumber, ui.Username, &updateData)
	}

	if err != nil {
		e := UpdateTaskParsingError
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

//nolint:gocyclo // ok here
func (ae *APIEnv) updateTaskInternal(ctx c.Context, workNumber, userLogin string, in *entity.TaskUpdate) (err error) {
	ctxLocal, span := trace.StartSpan(ctx, "update_task_internal")
	defer span.End()

	log := logger.GetLogger(ctx).WithField("mainFuncName", "updateTaskInternal")

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
		return errors.New(e.errorMessage(nil))
	}

	var steps entity.TaskSteps
	for _, blockType := range blockTypes {
		stepsByBlock, stepErr := ae.DB.GetUnfinishedTaskStepsByWorkIdAndStepType(ctxLocal, dbTask.ID, blockType, in.Action)
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

	if in.Action == entity.TaskUpdateActionCancelApp {
		steps = steps[:1]
	}

	couldUpdateOne := false
	spCtx := span.SpanContext()
	for _, item := range steps {
		// nolint:staticcheck // fix later
		routineCtx := c.WithValue(c.Background(), XRequestIDHeader, ctx.Value(XRequestIDHeader))
		routineCtx = logger.WithLogger(routineCtx, log)
		processCtx, fakeSpan := trace.StartSpanWithRemoteParent(routineCtx, "start_task_step_update", spCtx)
		fakeSpan.End()

		txStorage, transactionErr := ae.DB.StartTransaction(processCtx)
		if transactionErr != nil {
			continue
		}

		storage, getErr := txStorage.GetVariableStorageForStep(processCtx, dbTask.ID, item.Name)
		if getErr != nil {
			if txErr := txStorage.RollbackTransaction(processCtx); txErr != nil {
				log.WithField("funcName", "GetVariableStorageForStep").
					WithError(errors.New("couldn't rollback tx")).
					Error(txErr)
			}
			log.WithError(getErr).Error("couldn't get block to update")
			continue
		}
		runCtx := &pipeline.BlockRunContext{
			TaskID:     dbTask.ID,
			WorkNumber: workNumber,
			WorkTitle:  dbTask.Name,
			Initiator:  dbTask.Author,
			Storage:    txStorage,
			VarStore:   storage,

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

			UpdateData: &script.BlockUpdateData{
				ByLogin:    userLogin,
				Action:     string(in.Action),
				Parameters: in.Parameters,
			},
			Delegations: delegations,
			IsTest:      dbTask.IsTest,
			NotifName:   dbTask.Name,
		}

		blockFunc, ok := scenario.Pipeline.Blocks[item.Name]
		if !ok {
			if txErr := txStorage.RollbackTransaction(processCtx); txErr != nil {
				log.WithField("funcName", "get block by name").
					WithError(errors.New("couldn't rollback tx")).
					Error(txErr)
			}
			log.WithError(errors.New("couldn't get block from pipeline")).
				Error("couldn't get block to update")
			continue
		}

		blockErr := pipeline.ProcessBlockWithEndMapping(processCtx, item.Name, &blockFunc, runCtx, true)
		if blockErr != nil {
			if txErr := txStorage.RollbackTransaction(processCtx); txErr != nil {
				log.WithField("funcName", "ProcessBlock").
					WithError(errors.New("couldn't rollback tx")).
					Error(txErr)
			}
			log.WithError(blockErr).Error("couldn't update block")
			continue
		}

		if err = txStorage.CommitTransaction(processCtx); err != nil {
			log.WithError(err).Error("couldn't update block")
			continue
		}

		couldUpdateOne = true
	}

	if !couldUpdateOne {
		e := UpdateBlockError
		return errors.New(e.errorMessage(errors.New("couldn't update work")))
	}

	return
}

func (ae *APIEnv) updateApplicationInternal(ctx c.Context, workNumber, userLogin string, in *entity.TaskUpdate) (err error) {
	ctxLocal, span := trace.StartSpan(ctx, "update_application_internal")
	defer span.End()

	blockTypes := getTaskStepNameByAction(in.Action)
	if len(blockTypes) == 0 {
		return errors.New("blockTypes is empty")
	}

	dbTask, err := ae.DB.GetTask(ctxLocal, []string{userLogin}, []string{userLogin}, userLogin, workNumber)

	delegations, getDelegationsErr := ae.HumanTasks.GetDelegationsToLogin(ctxLocal, userLogin)
	if getDelegationsErr != nil {
		return getDelegationsErr
	}

	delegationsByApprovement := delegations.FilterByType("approvement")
	delegationsByExecution := delegations.FilterByType("execution")

	taskMembersLogins, err := ae.DB.GetTaskMembersLogins(ctx, workNumber)
	if err != nil {
		return err
	}

	loginsToNotify := make([]string, 0)
	loginsToNotify = append(loginsToNotify, delegationsByApprovement.GetUserInArrayWithDelegators([]string{})...)
	loginsToNotify = append(loginsToNotify, delegationsByExecution.GetUserInArrayWithDelegators([]string{})...)
	loginsToNotify = append(loginsToNotify, taskMembersLogins...)

	emails := make([]string, 0, len(loginsToNotify))
	for _, login := range loginsToNotify {
		email, getUserEmailErr := ae.People.GetUserEmail(ctx, login)
		if getUserEmailErr != nil {
			continue
		}
		emails = append(emails, email)
	}

	em := mail.NewRejectPipelineGroupTemplate(dbTask.WorkNumber, dbTask.Name, ae.Mail.SdAddress)
	err = ae.Mail.SendNotification(ctxLocal, emails, nil, em)
	if err != nil {
		return err
	}

	err = ae.DB.StopTaskBlocks(ctxLocal, dbTask.ID)
	if err != nil {
		return err
	}

	err = ae.DB.UpdateTaskStatus(ctxLocal, dbTask.ID, db.RunStatusFinished)
	if err != nil {
		return err
	}

	err = ae.DB.UpdateTaskHumanStatus(ctxLocal, dbTask.ID, string(pipeline.StatusRevoke))
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
