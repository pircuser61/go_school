package pipeline

import (
	c "context"
	"encoding/json"
	"fmt"
	"time"

	"golang.org/x/net/context"

	"github.com/pkg/errors"

	e "gitlab.services.mts.ru/abp/mail/pkg/email"
	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	human_tasks "gitlab.services.mts.ru/jocasta/pipeliner/internal/human-tasks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
)

//nolint:gocyclo //its ok here
func (gb *GoExecutionBlock) Update(ctx c.Context) (interface{}, error) {
	gb.RunContext.Delegations = gb.RunContext.Delegations.FilterByType("execution")
	switch gb.RunContext.UpdateData.Action {
	case string(entity.TaskUpdateActionSLABreach):
		if errUpdate := gb.handleBreachedSLA(ctx); errUpdate != nil {
			return nil, errUpdate
		}
	case string(entity.TaskUpdateActionHalfSLABreach):
		if errUpdate := gb.handleHalfSLABreached(ctx); errUpdate != nil {
			return nil, errUpdate
		}
	case string(entity.TaskUpdateActionReworkSLABreach):
		if errUpdate := gb.handleReworkSLABreached(ctx); errUpdate != nil {
			return nil, errUpdate
		}
	case string(entity.TaskUpdateActionExecution):
		if errUpdate := gb.updateDecision(); errUpdate != nil {
			return nil, errUpdate
		}
	case string(entity.TaskUpdateActionChangeExecutor):
		if errUpdate := gb.changeExecutor(ctx); errUpdate != nil {
			return nil, errUpdate
		}
	case string(entity.TaskUpdateActionRequestExecutionInfo):
		if errUpdate := gb.updateRequestInfo(ctx); errUpdate != nil {
			return nil, errUpdate
		}
	case string(entity.TaskUpdateActionExecutorStartWork):
		if errUpdate := gb.executorStartWork(ctx); errUpdate != nil {
			return nil, errUpdate
		}
	case string(entity.TaskUpdateActionExecutorSendEditApp):
		if errUpdate := gb.toEditApplication(ctx); errUpdate != nil {
			return nil, errUpdate
		}
	case string(entity.TaskUpdateActionDayBeforeSLARequestAddInfo):
		if errUpdate := gb.handleBreachedDayBeforeSLARequestAddInfo(ctx); errUpdate != nil {
			return nil, errUpdate
		}
	case string(entity.TaskUpdateActionSLABreachRequestAddInfo):
		if errUpdate := gb.HandleBreachedSLARequestAddInfo(ctx); errUpdate != nil {
			return nil, errUpdate
		}
	}

	var stateBytes []byte
	stateBytes, err := json.Marshal(gb.State)
	if err != nil {
		return nil, err
	}

	gb.RunContext.VarStore.ReplaceState(gb.Name, stateBytes)

	return nil, nil
}

type ExecutorChangeParams struct {
	NewExecutorLogin string   `json:"new_executor_login"`
	Comment          string   `json:"comment"`
	Attachments      []string `json:"attachments,omitempty"`
}

func (gb *GoExecutionBlock) changeExecutor(ctx c.Context) (err error) {
	var currentLogin = gb.RunContext.UpdateData.ByLogin
	_, executorFound := gb.State.Executors[currentLogin]

	_, isDelegate := gb.RunContext.Delegations.FindDelegatorFor(currentLogin, getSliceFromMapOfStrings(gb.State.Executors))
	if !(executorFound || isDelegate) && currentLogin != AutoApprover {
		return NewUserIsNotPartOfProcessErr()
	}

	var updateParams ExecutorChangeParams
	if err = json.Unmarshal(gb.RunContext.UpdateData.Parameters, &updateParams); err != nil {
		return errors.New("can't assert provided update data")
	}

	if err = gb.State.SetChangeExecutor(gb.RunContext.UpdateData.ByLogin, &updateParams); err != nil {
		return errors.New("can't assert provided change executor data")
	}

	delete(gb.State.Executors, gb.RunContext.UpdateData.ByLogin)
	oldExecutors := gb.State.Executors

	// add new person to exec anyway
	defer func() {
		oldExecutors[updateParams.NewExecutorLogin] = struct{}{}
		gb.State.Executors = oldExecutors
	}()

	gb.State.Executors = map[string]struct{}{
		updateParams.NewExecutorLogin: {},
	}
	// do notif only for the new person
	if notifErr := gb.handleNotifications(ctx); notifErr != nil {
		return notifErr
	}

	return nil
}

func (a *ExecutionData) SetChangeExecutor(oldLogin string, in *ExecutorChangeParams) error {
	_, ok := a.Executors[oldLogin]
	if !ok {
		return fmt.Errorf("%s not found in executors", oldLogin)
	}

	a.ChangedExecutorsLogs = append(a.ChangedExecutorsLogs, ChangeExecutorLog{
		OldLogin:    oldLogin,
		NewLogin:    in.NewExecutorLogin,
		Comment:     in.Comment,
		Attachments: in.Attachments,
		CreatedAt:   time.Now(),
	})

	return nil
}

type ExecutionUpdateParams struct {
	Decision    ExecutionDecision `json:"decision"`
	Comment     string            `json:"comment"`
	Attachments []string          `json:"attachments"`
}

//nolint:dupl //its not duplicate
func (gb *GoExecutionBlock) handleBreachedSLA(ctx c.Context) error {
	const fn = "pipeline.execution.handleBreachedSLA"

	if !gb.State.CheckSLA {
		gb.State.SLAChecked = true
		gb.State.HalfSLAChecked = true
		return nil
	}

	log := logger.GetLogger(ctx)

	if gb.State.SLA >= 8 {
		emails := make([]string, 0, len(gb.State.Executors))
		logins := getSliceFromMapOfStrings(gb.State.Executors)

		delegations, err := gb.RunContext.HumanTasks.GetDelegationsByLogins(ctx, logins)
		if err != nil {
			log.WithError(err).Info(fn, fmt.Sprintf("executors %v have no delegates", logins))
		}
		delegations = delegations.FilterByType("execution")
		logins = delegations.GetUserInArrayWithDelegations(logins)

		var executorEmail string
		for i := range logins {
			executorEmail, err = gb.RunContext.People.GetUserEmail(ctx, logins[i])
			if err != nil {
				log.WithError(err).Warning(fn, fmt.Sprintf("executor login %s not found", logins[i]))
				continue
			}
			emails = append(emails, executorEmail)
		}

		if len(emails) == 0 {
			return nil
		}
		notifName, err := gb.RunContext.GetTestName(ctx)
		if err != nil {
			return err
		}
		err = gb.RunContext.Sender.SendNotification(
			ctx,
			emails,
			nil,
			mail.NewExecutionSLATpl(
				gb.RunContext.WorkNumber,
				notifName,
				gb.RunContext.Sender.SdAddress,
			))
		if err != nil {
			return err
		}
	}
	gb.State.SLAChecked = true
	gb.State.HalfSLAChecked = true
	return nil
}

//nolint:dupl //its not duplicate
func (gb *GoExecutionBlock) handleHalfSLABreached(ctx c.Context) error {
	const fn = "pipeline.execution.handleHalfSLABreached"

	if !gb.State.CheckSLA {
		gb.State.SLAChecked = true
		gb.State.HalfSLAChecked = true
		return nil
	}

	log := logger.GetLogger(ctx)

	if gb.State.SLA >= 8 {
		emails := make([]string, 0, len(gb.State.Executors))
		logins := getSliceFromMapOfStrings(gb.State.Executors)

		delegations, err := gb.RunContext.HumanTasks.GetDelegationsByLogins(ctx, logins)
		if err != nil {
			log.WithError(err).Info(fn, fmt.Sprintf("executors %v have no delegates", logins))
		}
		delegations = delegations.FilterByType("execution")
		logins = delegations.GetUserInArrayWithDelegations(logins)

		var executorEmail string
		for i := range logins {
			executorEmail, err = gb.RunContext.People.GetUserEmail(ctx, logins[i])
			if err != nil {
				log.WithError(err).Warning(fn, fmt.Sprintf("executor login %s not found", logins[i]))
				continue
			}
			emails = append(emails, executorEmail)
		}

		if len(emails) == 0 {
			return nil
		}

		task, getVersionErr := gb.RunContext.Storage.GetVersionByWorkNumber(ctx, gb.RunContext.WorkNumber)
		if getVersionErr != nil {
			return getVersionErr
		}

		processSettings, getVersionErr := gb.RunContext.Storage.GetVersionSettings(ctx, task.VersionID.String())
		if getVersionErr != nil {
			return getVersionErr
		}

		taskRunContext, getDataErr := gb.RunContext.Storage.GetTaskRunContext(ctx, gb.RunContext.WorkNumber)
		if getDataErr != nil {
			return getDataErr
		}

		login := task.Author

		recipient := getRecipientFromState(&taskRunContext.InitialApplication.ApplicationBody)

		if recipient != "" {
			login = recipient
		}

		lastWorksForUser := make([]*entity.EriusTask, 0)

		if processSettings.ResubmissionPeriod > 0 {
			var getWorksErr error
			lastWorksForUser, getWorksErr = gb.RunContext.Storage.GetWorksForUserWithGivenTimeRange(ctx,
				processSettings.ResubmissionPeriod,
				login,
				task.VersionID.String(),
				gb.RunContext.WorkNumber,
			)
			if getWorksErr != nil {
				return getWorksErr
			}
		}
		notifName, err := gb.RunContext.GetTestName(ctx)
		if err != nil {
			return err
		}
		err = gb.RunContext.Sender.SendNotification(
			ctx,
			emails,
			nil,
			mail.NewExecutiontHalfSLATpl(
				gb.RunContext.WorkNumber,
				notifName,
				gb.RunContext.Sender.SdAddress,
				lastWorksForUser,
			))
		if err != nil {
			return err
		}
	}
	gb.State.HalfSLAChecked = true
	return nil
}

// nolint:dupl // another action
func (gb *GoExecutionBlock) handleReworkSLABreached(ctx c.Context) error {
	const fn = "pipeline.execution.handleReworkSLABreached"

	if !gb.State.CheckReworkSLA {
		return nil
	}

	log := logger.GetLogger(ctx)

	decision := ExecutionDecisionRejected
	gb.State.Decision = &decision
	gb.State.EditingApp = nil

	comment := fmt.Sprintf("заявка автоматически перенесена в архив по истечении %d дней", gb.State.ReworkSLA/8)
	gb.State.DecisionComment = &comment

	if stopErr := gb.RunContext.Storage.StopTaskBlocks(ctx, gb.RunContext.TaskID); stopErr != nil {
		return stopErr
	}

	if stopErr := gb.RunContext.updateTaskStatus(ctx, db.RunStatusFinished); stopErr != nil {
		return stopErr
	}

	if stopErr := gb.RunContext.Storage.SendTaskToArchive(ctx, gb.RunContext.TaskID); stopErr != nil {
		return stopErr
	}

	loginsToNotify := []string{gb.RunContext.Initiator}

	var em string
	var err error
	emails := make([]string, 0, len(loginsToNotify))
	for _, login := range loginsToNotify {
		em, err = gb.RunContext.People.GetUserEmail(ctx, login)
		if err != nil {
			log.WithError(err).Warning(fn, fmt.Sprintf("login %s not found", login))
			continue
		}

		emails = append(emails, em)
	}
	notifName, err := gb.RunContext.GetTestName(ctx)
	if err != nil {
		return err
	}
	tpl := mail.NewReworkSLATpl(gb.RunContext.WorkNumber, notifName, gb.RunContext.Sender.SdAddress, gb.State.ReworkSLA)
	err = gb.RunContext.Sender.SendNotification(ctx, emails, nil, tpl)
	if err != nil {
		return err
	}

	return nil
}

func (gb *GoExecutionBlock) handleBreachedDayBeforeSLARequestAddInfo(ctx context.Context) error {
	const fn = "pipeline.execution.handleBreachedDayBeforeSLARequestAddInfo"

	if !gb.State.CheckDayBeforeSLARequestInfo {
		return nil
	}

	log := logger.GetLogger(ctx)

	loginsToNotify := []string{gb.RunContext.Initiator}

	emails := make([]string, 0, len(loginsToNotify))
	for _, login := range loginsToNotify {
		email, err := gb.RunContext.People.GetUserEmail(ctx, login)
		if err != nil {
			log.WithError(err).Warning(fn, fmt.Sprintf("login %s not found", login))
			continue
		}

		emails = append(emails, email)
	}
	notifName, err := gb.RunContext.GetTestName(ctx)
	if err != nil {
		return err
	}
	tpl := mail.NewDayBeforeRequestAddInfoSLABreached(gb.RunContext.WorkNumber, notifName, gb.RunContext.Sender.SdAddress)
	err = gb.RunContext.Sender.SendNotification(ctx, emails, nil, tpl)
	if err != nil {
		return err
	}

	gb.State.CheckDayBeforeSLARequestInfo = false

	return nil
}

//nolint:dupl // dont duplicate
func (gb *GoExecutionBlock) HandleBreachedSLARequestAddInfo(ctx context.Context) error {
	const fn = "pipeline.execution.HandleBreachedSLARequestAddInfo"
	var comment = "заявка автоматически перенесена в архив по истечении 3 дней"

	log := logger.GetLogger(ctx)

	decision := ExecutionDecisionRejected
	gb.State.Decision = &decision
	gb.State.DecisionComment = &comment

	if stopErr := gb.RunContext.Storage.StopTaskBlocks(ctx, gb.RunContext.TaskID); stopErr != nil {
		return stopErr
	}

	if stopErr := gb.RunContext.updateTaskStatus(ctx, db.RunStatusFinished); stopErr != nil {
		return stopErr
	}

	if stopErr := gb.RunContext.Storage.SendTaskToArchive(ctx, gb.RunContext.TaskID); stopErr != nil {
		return stopErr
	}

	executors := getSliceFromMapOfStrings(gb.State.Executors)
	delegates, getDelegationsErr := gb.RunContext.HumanTasks.GetDelegationsByLogins(ctx, executors)
	if getDelegationsErr != nil {
		return getDelegationsErr
	}
	delegates = delegates.FilterByType("execution")

	loginsToNotify := delegates.GetUserInArrayWithDelegations(executors)
	loginsToNotify = append(loginsToNotify, gb.RunContext.Initiator)

	var em string
	var err error
	emails := make([]string, 0, len(loginsToNotify))
	for _, login := range loginsToNotify {
		em, err = gb.RunContext.People.GetUserEmail(ctx, login)
		if err != nil {
			log.WithError(err).Warning(fn, fmt.Sprintf("login %s not found", login))
			continue
		}

		emails = append(emails, em)
	}
	notifName, err := gb.RunContext.GetTestName(ctx)
	if err != nil {
		return err
	}
	tpl := mail.NewRequestAddInfoSLABreached(gb.RunContext.WorkNumber, notifName, gb.RunContext.Sender.SdAddress)
	err = gb.RunContext.Sender.SendNotification(ctx, emails, nil, tpl)
	if err != nil {
		return err
	}

	return nil
}

func (gb *GoExecutionBlock) updateDecision() error {
	var updateParams ExecutionUpdateParams

	err := json.Unmarshal(gb.RunContext.UpdateData.Parameters, &updateParams)
	if err != nil {
		return errors.New("can't assert provided update data")
	}

	if errSet := gb.State.SetDecision(gb.RunContext.UpdateData.ByLogin, &updateParams, gb.RunContext.Delegations); errSet != nil {
		return errSet
	}

	if gb.State.Decision != nil {
		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputExecutionLogin], &gb.State.ActualExecutor)
		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputDecision], &gb.State.Decision)
		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputComment], &gb.State.DecisionComment)
	}

	return nil
}

func (a *ExecutionData) SetDecision(login string, in *ExecutionUpdateParams, delegations human_tasks.Delegations) error {
	_, executorFound := a.Executors[login]

	delegateFor, isDelegate := delegations.FindDelegatorFor(login, getSliceFromMapOfStrings(a.Executors))
	if !(executorFound || isDelegate) && login != AutoApprover {
		return NewUserIsNotPartOfProcessErr()
	}

	if a.Decision != nil {
		return errors.New("decision already set")
	}

	if in.Decision != ExecutionDecisionExecuted && in.Decision != ExecutionDecisionRejected {
		return fmt.Errorf("unknown decision %s", in.Decision)
	}

	a.Decision = &in.Decision
	a.DecisionComment = &in.Comment
	a.DecisionAttachments = in.Attachments
	a.ActualExecutor = &login
	a.DelegateFor = delegateFor

	return nil
}

type RequestInfoUpdateParams struct {
	Comment       string          `json:"comment"`
	ReqType       RequestInfoType `json:"req_type"`
	Attachments   []string        `json:"attachments"`
	ExecutorLogin string          `json:"executor_login"`
}

//nolint:gocyclo //its ok here
func (gb *GoExecutionBlock) updateRequestInfo(ctx c.Context) (err error) {
	var updateParams RequestInfoUpdateParams
	var delegations = gb.RunContext.Delegations.FilterByType("execution")

	err = json.Unmarshal(gb.RunContext.UpdateData.Parameters, &updateParams)
	if err != nil {
		return errors.New("can't assert provided update requestExecutionInfo data")
	}

	if errSet := gb.State.SetRequestExecutionInfo(gb.RunContext.UpdateData.ByLogin, delegations, &updateParams); errSet != nil {
		return errSet
	}

	if updateParams.ReqType == RequestInfoAnswer {
		if gb.RunContext.UpdateData.ByLogin != gb.RunContext.Initiator {
			return NewUserIsNotPartOfProcessErr()
		}

		if len(gb.State.RequestExecutionInfoLogs) > 0 {
			workHours := getWorkHoursBetweenDates(
				gb.State.RequestExecutionInfoLogs[len(gb.State.RequestExecutionInfoLogs)-1].CreatedAt,
				time.Now(),
				nil,
			)
			gb.State.IncreaseSLA(workHours)
		}
	}

	if updateParams.ReqType == RequestInfoQuestion {
		err = gb.notificateNeedMoreInfo(ctx)
		if err != nil {
			return err
		}

		gb.State.CheckDayBeforeSLARequestInfo = true
	}

	if updateParams.ReqType == RequestInfoAnswer {
		err = gb.notificateNewInfoRecieved(ctx)
		if err != nil {
			return err
		}
	}

	return err
}

func (a *ExecutionData) SetRequestExecutionInfo(login string, delegations human_tasks.Delegations,
	in *RequestInfoUpdateParams) error {
	_, executorFound := a.Executors[login]
	delegateFor, isDelegate := delegations.FindDelegatorFor(
		login, getSliceFromMapOfStrings(a.Executors))

	if !(executorFound || isDelegate) && in.ReqType == RequestInfoQuestion {
		return NewUserIsNotPartOfProcessErr()
	}

	if in.ReqType != RequestInfoAnswer && in.ReqType != RequestInfoQuestion {
		return fmt.Errorf("request info type is not valid")
	}

	a.RequestExecutionInfoLogs = append(a.RequestExecutionInfoLogs, RequestExecutionInfoLog{
		Login:       login,
		Comment:     in.Comment,
		CreatedAt:   time.Now(),
		ReqType:     in.ReqType,
		Attachments: in.Attachments,
		DelegateFor: delegateFor,
	})

	return nil
}

func (gb *GoExecutionBlock) executorStartWork(ctx c.Context) (err error) {
	log := logger.GetLogger(ctx)

	if gb.State.IsTakenInWork {
		return nil
	}
	var currentLogin = gb.RunContext.UpdateData.ByLogin
	_, executorFound := gb.State.Executors[currentLogin]

	_, isDelegate := gb.RunContext.Delegations.FindDelegatorFor(currentLogin, getSliceFromMapOfStrings(gb.State.Executors))
	if !(executorFound || isDelegate) && currentLogin != AutoApprover {
		return NewUserIsNotPartOfProcessErr()
	}

	executorLogins := make(map[string]struct{}, 0)
	for i := range gb.State.Executors {
		executorLogins[i] = gb.State.Executors[i]
	}

	gb.State.Executors = map[string]struct{}{
		gb.RunContext.UpdateData.ByLogin: {},
	}

	gb.State.IsTakenInWork = true
	workHours := getWorkHoursBetweenDates(gb.RunContext.currBlockStartTime, time.Now(), nil)
	gb.State.IncreaseSLA(workHours)

	if err = gb.emailGroupExecutors(ctx, gb.RunContext.UpdateData.ByLogin, executorLogins); err != nil {
		log.Error(err)
		return err
	}

	return nil
}

// nolint:gocyclo // mb later
func (gb *GoExecutionBlock) emailGroupExecutors(ctx c.Context, loginTakenInWork string, logins map[string]struct{}) (err error) {
	log := logger.GetLogger(ctx)

	executors := getSliceFromMapOfStrings(logins)
	log.WithField("func", "emailGroupExecutors").WithField("logins", logins)

	delegates, err := gb.RunContext.HumanTasks.GetDelegationsByLogins(ctx, executors)
	if err != nil {
		return err
	}

	delegates = delegates.FilterByType("execution")

	loginsToNotify := delegates.GetUserInArrayWithDelegations(executors)

	emails := make([]string, 0, len(executors))
	for _, login := range loginsToNotify {
		if login != loginTakenInWork {
			email, emailErr := gb.RunContext.People.GetUserEmail(ctx, login)
			if emailErr != nil {
				return emailErr
			}

			emails = append(emails, email)
		}
	}

	log.WithField("func", "emailGroupExecutors").WithField("emails", emails)

	var description string
	var emailAttachment []e.Attachment

	description, err = gb.RunContext.makeNotificationDescription(gb.Name)
	if err != nil {
		return err
	}

	author, err := gb.RunContext.People.GetUser(ctx, gb.RunContext.UpdateData.ByLogin)
	if err != nil {
		return err
	}

	typedAuthor, err := author.ToSSOUserTyped()
	if err != nil {
		return err
	}

	task, getVersionErr := gb.RunContext.Storage.GetVersionByWorkNumber(ctx, gb.RunContext.WorkNumber)
	if getVersionErr != nil {
		return getVersionErr
	}

	processSettings, getVersionErr := gb.RunContext.Storage.GetVersionSettings(ctx, task.VersionID.String())
	if getVersionErr != nil {
		return getVersionErr
	}

	taskRunContext, getDataErr := gb.RunContext.Storage.GetTaskRunContext(ctx, gb.RunContext.WorkNumber)
	if getDataErr != nil {
		return getDataErr
	}

	login := task.Author

	recipient := getRecipientFromState(&taskRunContext.InitialApplication.ApplicationBody)

	if recipient != "" {
		login = recipient
	}

	lastWorksForUser := make([]*entity.EriusTask, 0)

	if processSettings.ResubmissionPeriod > 0 {
		var getWorksErr error
		lastWorksForUser, getWorksErr = gb.RunContext.Storage.GetWorksForUserWithGivenTimeRange(ctx,
			processSettings.ResubmissionPeriod,
			login,
			task.VersionID.String(),
			gb.RunContext.WorkNumber,
		)
		if getWorksErr != nil {
			return getWorksErr
		}
	}
	notifName, err := gb.RunContext.GetTestName(ctx)
	if err != nil {
		return err
	}
	tpl := mail.NewExecutionTakenInWorkTpl(&mail.ExecutorNotifTemplate{
		WorkNumber:   gb.RunContext.WorkNumber,
		Name:         notifName,
		SdUrl:        gb.RunContext.Sender.SdAddress,
		Description:  description,
		ExecutorName: typedAuthor.GetFullName(),
		Initiator:    gb.RunContext.Initiator,
		LastWorks:    lastWorksForUser,
		Mailto:       gb.RunContext.Sender.FetchEmail,
	})

	if errSend := gb.RunContext.Sender.SendNotification(ctx, emails, emailAttachment, tpl); errSend != nil {
		return errSend
	}

	emailTakenInWork, emailErr := gb.RunContext.People.GetUserEmail(ctx, loginTakenInWork)
	if emailErr != nil {
		return emailErr
	}

	tpl = mail.NewAppPersonStatusNotificationTpl(
		&mail.NewAppPersonStatusTpl{
			WorkNumber:  gb.RunContext.WorkNumber,
			Name:        notifName,
			Status:      string(StatusExecution),
			Action:      statusToTaskAction[StatusExecution],
			DeadLine:    ComputeDeadline(time.Now(), gb.State.SLA),
			Description: description,
			SdUrl:       gb.RunContext.Sender.SdAddress,
			Mailto:      gb.RunContext.Sender.FetchEmail,
			Login:       loginTakenInWork,
			IsEditable:  gb.State.GetIsEditable(),

			BlockID:                   BlockGoExecutionID,
			ExecutionDecisionExecuted: string(ExecutionDecisionExecuted),
			ExecutionDecisionRejected: string(ExecutionDecisionRejected),
			LastWorks:                 lastWorksForUser,
		})

	if sendErr := gb.RunContext.Sender.SendNotification(ctx, []string{emailTakenInWork}, emailAttachment, tpl); sendErr != nil {
		return sendErr
	}

	return nil
}

type executorUpdateEditParams struct {
	Comment     string   `json:"comment"`
	Attachments []string `json:"attachments"`
}

//nolint:gocyclo //its ok here
func (gb *GoExecutionBlock) toEditApplication(ctx c.Context) (err error) {
	var updateParams executorUpdateEditParams
	if err = json.Unmarshal(gb.RunContext.UpdateData.Parameters, &updateParams); err != nil {
		return errors.New("can't assert provided update data")
	}

	if editErr := gb.State.setEditApp(gb.RunContext.UpdateData.ByLogin, updateParams, gb.RunContext.Delegations); editErr != nil {
		return editErr
	}

	loginsToNotify := []string{gb.RunContext.Initiator}

	var email string
	emails := make([]string, 0, len(loginsToNotify))
	for _, login := range loginsToNotify {
		email, err = gb.RunContext.People.GetUserEmail(ctx, login)
		if err != nil {
			return err
		}

		emails = append(emails, email)
	}
	notifName, err := gb.RunContext.GetTestName(ctx)
	if err != nil {
		return err
	}
	tpl := mail.NewAnswerSendToEditTpl(gb.RunContext.WorkNumber,
		notifName, gb.RunContext.Sender.SdAddress)
	err = gb.RunContext.Sender.SendNotification(ctx, emails, nil, tpl)
	if err != nil {
		return err
	}

	return nil
}

func (gb *GoExecutionBlock) notificateNeedMoreInfo(ctx c.Context) error {
	loginsToNotify := []string{gb.RunContext.Initiator}

	emails := make([]string, 0, len(loginsToNotify))
	for _, login := range loginsToNotify {
		email, err := gb.RunContext.People.GetUserEmail(ctx, login)
		if err != nil {
			return err
		}

		emails = append(emails, email)
	}
	notifName, err := gb.RunContext.GetTestName(ctx)
	if err != nil {
		return err
	}
	tpl := mail.NewRequestExecutionInfoTpl(gb.RunContext.WorkNumber,
		notifName, gb.RunContext.Sender.SdAddress)

	err = gb.RunContext.Sender.SendNotification(ctx, emails, nil, tpl)
	if err != nil {
		return err
	}

	return nil
}

func (gb *GoExecutionBlock) notificateNewInfoRecieved(ctx c.Context) error {
	delegates, err := gb.RunContext.HumanTasks.GetDelegationsByLogins(ctx, getSliceFromMapOfStrings(gb.State.Executors))
	if err != nil {
		return err
	}

	loginsToNotify := delegates.GetUserInArrayWithDelegations(getSliceFromMapOfStrings(gb.State.Executors))

	var email string
	emails := make([]string, 0, len(loginsToNotify))
	for _, login := range loginsToNotify {
		email, err = gb.RunContext.People.GetUserEmail(ctx, login)
		if err != nil {
			continue
		}

		emails = append(emails, email)
	}
	notifName, err := gb.RunContext.GetTestName(ctx)
	if err != nil {
		return err
	}
	tpl := mail.NewAnswerExecutionInfoTpl(gb.RunContext.WorkNumber,
		notifName, gb.RunContext.Sender.SdAddress)
	err = gb.RunContext.Sender.SendNotification(ctx, emails, nil, tpl)
	if err != nil {
		return err
	}

	return nil
}
