package api

import (
	c "context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	ht "gitlab.services.mts.ru/jocasta/pipeliner/internal/human-tasks"
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
		TaskID:     data.task.ID,
		WorkNumber: data.workNumber,
		WorkTitle:  data.task.Name,
		Initiator:  data.task.Author,
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
			ByLogin:    data.login,
			Action:     string(data.updData.Action),
			Parameters: data.updData.Parameters,
		},
		Delegations: data.delegations,
		IsTest:      data.task.IsTest,
		NotifName:   data.task.Name,
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
	return true
}

//nolint:gocyclo // ok here
func (ae *APIEnv) updateTaskInternal(ctx c.Context, workNumber, userLogin string, in *entity.TaskUpdate) (err error) {
	ctxLocal, span := trace.StartSpan(ctx, "update_task_internal")
	defer span.End()

	log := logger.GetLogger(ctx)

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

	log.Info(fmt.Printf("update_task_internal steps: %+v", steps))

	if len(steps) == 0 {
		e := GetTaskError
		return errors.New(e.errorMessage(nil))
	}

	if in.Action == entity.TaskUpdateActionCancelApp {
		steps = steps[:1]
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
	taskMembers, err := ae.DB.GetTaskMembers(ctx, workNumber)
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

func (ae *APIEnv) updateApplicationInternal(ctx c.Context, workNumber, userLogin string, in *entity.TaskUpdate) (err error) {
	ctxLocal, span := trace.StartSpan(ctx, "update_application_internal")
	defer span.End()

	log := ae.Log.WithField("mainFuncName", "updateApplicationInternal")

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

	txStorage, transactionErr := ae.DB.StartTransaction(ctx)
	if transactionErr != nil {
		return transactionErr
	}
	defer func() {
		if r := recover(); r != nil {
			log = log.WithField("funcName", "updateApplicationInternal").
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

	err = ae.DB.UpdateTaskStatus(ctxLocal, dbTask.ID, db.RunStatusFinished)
	if err != nil {
		if txErr := txStorage.RollbackTransaction(ctxLocal); txErr != nil {
			log.WithField("funcName", "UpdateTaskStatus").
				WithError(errors.New("couldn't rollback tx")).
				Error(txErr)
		}
		return err
	}

	err = ae.DB.UpdateTaskHumanStatus(ctxLocal, dbTask.ID, string(pipeline.StatusRevoke))
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

	em := mail.NewRejectPipelineGroupTemplate(dbTask.WorkNumber, dbTask.Name, ae.Mail.SdAddress)
	err = ae.Mail.SendNotification(ctxLocal, emails, nil, em)
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

func (ae *APIEnv) StopTasks(w http.ResponseWriter, r *http.Request) {
}
