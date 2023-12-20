package pipeline

import (
	c "context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	file_registry "gitlab.services.mts.ru/jocasta/pipeliner/internal/file-registry"
	human_tasks "gitlab.services.mts.ru/jocasta/pipeliner/internal/human-tasks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sla"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

//nolint:gocyclo //its ok here
func (gb *GoExecutionBlock) Update(ctx c.Context) (interface{}, error) {
	data := gb.RunContext.UpdateData
	if data == nil {
		return nil, errors.New("empty data")
	}

	gb.RunContext.Delegations = gb.RunContext.Delegations.FilterByType("execution")
	switch data.Action {
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
		if !gb.State.IsTakenInWork {
			return nil, errors.New("is not taken in work")
		}
		if errUpdate := gb.updateDecision(ctx); errUpdate != nil {
			return nil, errUpdate
		}
	case string(entity.TaskUpdateActionChangeExecutor):
		if !gb.State.IsTakenInWork {
			return nil, errors.New("is not taken in work")
		}
		if errUpdate := gb.changeExecutor(ctx); errUpdate != nil {
			return nil, errUpdate
		}
	case string(entity.TaskUpdateActionRequestExecutionInfo):
		if !gb.State.IsTakenInWork {
			return nil, errors.New("is not taken in work")
		}
		if errUpdate := gb.updateRequestInfo(ctx); errUpdate != nil {
			return nil, errUpdate
		}
	case string(entity.TaskUpdateActionExecutorStartWork):
		if gb.State.IsTakenInWork {
			return nil, errors.New("is already taken in work")
		}
		if errUpdate := gb.executorStartWork(ctx); errUpdate != nil {
			return nil, errUpdate
		}
	case string(entity.TaskUpdateActionExecutorSendEditApp):
		if !gb.State.IsTakenInWork {
			return nil, errors.New("is not taken in work")
		}
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

	deadline, deadlineErr := gb.getDeadline(ctx, gb.State.WorkType)
	if deadlineErr != nil {
		return nil, deadlineErr
	}

	gb.State.Deadline = deadline

	var stateBytes []byte
	stateBytes, err := json.Marshal(gb.State)
	if err != nil {
		return nil, err
	}

	gb.RunContext.VarStore.ReplaceState(gb.Name, stateBytes)

	if _, ok := gb.expectedEvents[eventEnd]; ok {
		status, _, _ := gb.GetTaskHumanStatus()
		event, eventErr := gb.RunContext.MakeNodeEndEvent(ctx, MakeNodeEndEventArgs{
			NodeName:      gb.Name,
			NodeShortName: gb.ShortName,
			HumanStatus:   status,
			NodeStatus:    gb.GetStatus(),
		})
		if eventErr != nil {
			return nil, eventErr
		}
		gb.happenedEvents = append(gb.happenedEvents, event)
	}

	return nil, nil
}

type ExecutorChangeParams struct {
	NewExecutorLogin string              `json:"new_executor_login"`
	Comment          string              `json:"comment"`
	Attachments      []entity.Attachment `json:"attachments,omitempty"`
}

func (gb *GoExecutionBlock) changeExecutor(ctx c.Context) (err error) {
	var currentLogin = gb.RunContext.UpdateData.ByLogin
	_, executorFound := gb.State.Executors[currentLogin]

	_, isDelegate := gb.RunContext.Delegations.FindDelegatorFor(currentLogin, getSliceFromMapOfStrings(gb.State.Executors))
	if !(executorFound || isDelegate) {
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

	gb.State.IsTakenInWork = false

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
	Decision    ExecutionDecision   `json:"decision"`
	Comment     string              `json:"comment"`
	Attachments []entity.Attachment `json:"attachments"`
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

		delegations, err := gb.RunContext.Services.HumanTasks.GetDelegationsByLogins(ctx, logins)
		if err != nil {
			log.WithError(err).Info(fn, fmt.Sprintf("executors %v have no delegates", logins))
		}
		delegations = delegations.FilterByType("execution")
		logins = delegations.GetUserInArrayWithDelegations(logins)

		var executorEmail string
		for i := range logins {
			executorEmail, err = gb.RunContext.Services.People.GetUserEmail(ctx, logins[i])
			if err != nil {
				log.WithError(err).Warning(fn, fmt.Sprintf("executor login %s not found", logins[i]))
				continue
			}
			emails = append(emails, executorEmail)
		}

		tpl := mail.NewExecutionSLATpl(
			gb.RunContext.WorkNumber,
			gb.RunContext.NotifName,
			gb.RunContext.Services.Sender.SdAddress,
		)

		filesList := []string{tpl.Image}
		files, iconEerr := gb.RunContext.GetIcons(filesList)
		if iconEerr != nil {
			return iconEerr
		}

		if len(emails) == 0 {
			return nil
		}
		err = gb.RunContext.Services.Sender.SendNotification(ctx, emails, files, tpl)

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

		delegations, err := gb.RunContext.Services.HumanTasks.GetDelegationsByLogins(ctx, logins)
		if err != nil {
			log.WithError(err).Info(fn, fmt.Sprintf("executors %v have no delegates", logins))
		}
		delegations = delegations.FilterByType("execution")
		logins = delegations.GetUserInArrayWithDelegations(logins)

		var executorEmail string
		for i := range logins {
			executorEmail, err = gb.RunContext.Services.People.GetUserEmail(ctx, logins[i])
			if err != nil {
				log.WithError(err).Warning(fn, fmt.Sprintf("executor login %s not found", logins[i]))
				continue
			}
			emails = append(emails, executorEmail)
		}

		if len(emails) == 0 {
			return nil
		}

		task, getVersionErr := gb.RunContext.Services.Storage.GetVersionByWorkNumber(ctx, gb.RunContext.WorkNumber)
		if getVersionErr != nil {
			return getVersionErr
		}

		processSettings, getVersionErr := gb.RunContext.Services.Storage.GetVersionSettings(ctx, task.VersionID.String())
		if getVersionErr != nil {
			return getVersionErr
		}

		taskRunContext, getDataErr := gb.RunContext.Services.Storage.GetTaskRunContext(ctx, gb.RunContext.WorkNumber)
		if getDataErr != nil {
			return getDataErr
		}

		login := task.Author

		recipient := getRecipientFromState(&taskRunContext.InitialApplication.ApplicationBody)

		if recipient != "" {
			login = recipient
		}

		if processSettings.ResubmissionPeriod > 0 {
			var getWorksErr error
			_, getWorksErr = gb.RunContext.Services.Storage.GetWorksForUserWithGivenTimeRange(ctx,
				processSettings.ResubmissionPeriod,
				login,
				task.VersionID.String(),
				gb.RunContext.WorkNumber,
			)
			if getWorksErr != nil {
				return getWorksErr
			}
		}

		slaInfoPtr, getSlaInfoErr := gb.RunContext.Services.SLAService.GetSLAInfoPtr(ctx, sla.InfoDto{
			TaskCompletionIntervals: []entity.TaskCompletionInterval{{StartedAt: gb.RunContext.CurrBlockStartTime,
				FinishedAt: gb.RunContext.CurrBlockStartTime.Add(time.Hour * 24 * 100)}},
			WorkType: sla.WorkHourType(gb.State.WorkType),
		})
		if getSlaInfoErr != nil {
			return getSlaInfoErr
		}

		lastWorksForUser := make([]*entity.EriusTask, 0)

		if processSettings.ResubmissionPeriod > 0 {
			var getWorksErr error
			lastWorksForUser, getWorksErr = gb.RunContext.Services.Storage.GetWorksForUserWithGivenTimeRange(ctx,
				processSettings.ResubmissionPeriod,
				login,
				task.VersionID.String(),
				gb.RunContext.WorkNumber,
			)
			if getWorksErr != nil {
				return getWorksErr
			}
		}

		tpl := mail.NewExecutiontHalfSLATpl(
			gb.RunContext.WorkNumber,
			gb.RunContext.NotifName,
			gb.RunContext.Services.Sender.SdAddress,
			gb.RunContext.Services.SLAService.ComputeMaxDateFormatted(gb.RunContext.CurrBlockStartTime, gb.State.SLA,
				slaInfoPtr),
			lastWorksForUser,
		)

		files := []string{tpl.Image}

		if len(lastWorksForUser) != 0 {
			files = append(files, warningImg)
		}

		iconFiles, fileErr := gb.RunContext.GetIcons(files)
		if fileErr != nil {
			return fileErr
		}

		err = gb.RunContext.Services.Sender.SendNotification(ctx, emails, iconFiles, tpl)
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

	if stopErr := gb.RunContext.Services.Storage.StopTaskBlocks(ctx, gb.RunContext.TaskID); stopErr != nil {
		return stopErr
	}

	if stopErr := gb.RunContext.updateTaskStatus(ctx, db.RunStatusFinished, "", db.SystemLogin); stopErr != nil {
		return stopErr
	}

	if stopErr := gb.RunContext.Services.Storage.SendTaskToArchive(ctx, gb.RunContext.TaskID); stopErr != nil {
		return stopErr
	}

	loginsToNotify := []string{gb.RunContext.Initiator}

	var em string
	var err error
	emails := make([]string, 0, len(loginsToNotify))
	for _, login := range loginsToNotify {
		em, err = gb.RunContext.Services.People.GetUserEmail(ctx, login)
		if err != nil {
			log.WithError(err).Warning(fn, fmt.Sprintf("login %s not found", login))
			continue
		}

		emails = append(emails, em)
	}
	tpl := mail.NewReworkSLATpl(gb.RunContext.WorkNumber, gb.RunContext.NotifName,
		gb.RunContext.Services.Sender.SdAddress, gb.State.ReworkSLA, gb.State.CheckSLA)

	filesList := []string{tpl.Image}
	files, iconEerr := gb.RunContext.GetIcons(filesList)
	if iconEerr != nil {
		return iconEerr
	}

	err = gb.RunContext.Services.Sender.SendNotification(ctx, emails, files, tpl)
	if err != nil {
		return err
	}

	nodeEvents, err := gb.RunContext.GetCancelledStepsEvents(ctx)
	if err != nil {
		return err
	}
	for _, event := range nodeEvents {
		// event for this node will spawn later
		if event.NodeName == gb.Name {
			continue
		}
		gb.happenedEvents = append(gb.happenedEvents, event)
	}

	return nil
}

func (gb *GoExecutionBlock) handleBreachedDayBeforeSLARequestAddInfo(ctx c.Context) error {
	const fn = "pipeline.execution.handleBreachedDayBeforeSLARequestAddInfo"

	if !gb.State.CheckDayBeforeSLARequestInfo {
		return nil
	}

	log := logger.GetLogger(ctx)

	loginsToNotify := []string{gb.RunContext.Initiator}

	emails := make([]string, 0, len(loginsToNotify))
	for _, login := range loginsToNotify {
		email, err := gb.RunContext.Services.People.GetUserEmail(ctx, login)
		if err != nil {
			log.WithError(err).Warning(fn, fmt.Sprintf("login %s not found", login))
			continue
		}

		emails = append(emails, email)
	}
	tpl := mail.NewDayBeforeRequestAddInfoSLABreached(gb.RunContext.WorkNumber, gb.RunContext.NotifName,
		gb.RunContext.Services.Sender.SdAddress)

	filesList := []string{tpl.Image}
	files, iconErr := gb.RunContext.GetIcons(filesList)
	if iconErr != nil {
		return iconErr
	}

	err := gb.RunContext.Services.Sender.SendNotification(ctx, emails, files, tpl)
	if err != nil {
		return err
	}

	gb.State.CheckDayBeforeSLARequestInfo = false

	return nil
}

//nolint:dupl // dont duplicate
func (gb *GoExecutionBlock) HandleBreachedSLARequestAddInfo(ctx c.Context) error {
	const fn = "pipeline.execution.HandleBreachedSLARequestAddInfo"
	var comment = "заявка автоматически перенесена в архив по истечении 3 дней"

	log := logger.GetLogger(ctx)

	decision := ExecutionDecisionRejected
	gb.State.Decision = &decision
	gb.State.DecisionComment = &comment

	if stopErr := gb.RunContext.Services.Storage.StopTaskBlocks(ctx, gb.RunContext.TaskID); stopErr != nil {
		return stopErr
	}

	if stopErr := gb.RunContext.updateTaskStatus(ctx, db.RunStatusFinished, "", db.SystemLogin); stopErr != nil {
		return stopErr
	}

	if stopErr := gb.RunContext.Services.Storage.SendTaskToArchive(ctx, gb.RunContext.TaskID); stopErr != nil {
		return stopErr
	}

	executors := getSliceFromMapOfStrings(gb.State.Executors)
	delegates, getDelegationsErr := gb.RunContext.Services.HumanTasks.GetDelegationsByLogins(ctx, executors)
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
		em, err = gb.RunContext.Services.People.GetUserEmail(ctx, login)
		if err != nil {
			log.WithError(err).Warning(fn, fmt.Sprintf("login %s not found", login))
			continue
		}

		emails = append(emails, em)
	}
	tpl := mail.NewRequestAddInfoSLABreached(gb.RunContext.WorkNumber, gb.RunContext.NotifName,
		gb.RunContext.Services.Sender.SdAddress, gb.State.ReworkSLA)

	filesList := []string{tpl.Image}
	files, iconEerr := gb.RunContext.GetIcons(filesList)
	if iconEerr != nil {
		return iconEerr
	}

	err = gb.RunContext.Services.Sender.SendNotification(ctx, emails, files, tpl)
	if err != nil {
		return err
	}

	nodeEvents, err := gb.RunContext.GetCancelledStepsEvents(ctx)
	if err != nil {
		return err
	}
	for _, event := range nodeEvents {
		// event for this node will spawn later
		if event.NodeName == gb.Name {
			continue
		}
		gb.happenedEvents = append(gb.happenedEvents, event)
	}

	return nil
}

func (gb *GoExecutionBlock) updateDecision(ctx c.Context) error {
	var updateParams ExecutionUpdateParams

	err := json.Unmarshal(gb.RunContext.UpdateData.Parameters, &updateParams)
	if err != nil {
		return errors.New("can't assert provided update data")
	}

	if errSet := gb.State.SetDecision(gb.RunContext.UpdateData.ByLogin, &updateParams,
		gb.RunContext.Delegations); errSet != nil {
		return errSet
	}

	if gb.State.Decision != nil {
		person, personErr := gb.RunContext.Services.ServiceDesc.GetSsoPerson(ctx, *gb.State.ActualExecutor)
		if personErr != nil {
			return personErr
		}

		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputExecutionLogin], person)
		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputDecision], &gb.State.Decision)
		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputComment], &gb.State.DecisionComment)
	}

	return nil
}

type RequestInfoUpdateParams struct {
	Comment       string              `json:"comment"`
	ReqType       RequestInfoType     `json:"req_type"`
	Attachments   []entity.Attachment `json:"attachments"`
	ExecutorLogin string              `json:"executor_login"`
}

//nolint:gocyclo //its ok here
func (gb *GoExecutionBlock) updateRequestInfo(ctx c.Context) (err error) {
	var updateParams RequestInfoUpdateParams
	var delegations = gb.RunContext.Delegations.FilterByType("execution")

	err = json.Unmarshal(gb.RunContext.UpdateData.Parameters, &updateParams)
	if err != nil {
		return errors.New("can't assert provided update requestExecutionInfo data")
	}

	if errSet := gb.State.SetRequestExecutionInfo(gb.RunContext.UpdateData.ByLogin, delegations,
		&updateParams); errSet != nil {
		return errSet
	}

	if updateParams.ReqType == RequestInfoQuestion {
		err = gb.notifyNeedMoreInfo(ctx)
		if err != nil {
			return err
		}

		gb.State.CheckDayBeforeSLARequestInfo = true
	}

	if updateParams.ReqType == RequestInfoAnswer {
		if gb.RunContext.UpdateData.ByLogin != gb.RunContext.Initiator {
			return NewUserIsNotPartOfProcessErr()
		}

		err = gb.notifyNewInfoReceived(ctx)
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
	var currentLogin = gb.RunContext.UpdateData.ByLogin
	_, executorFound := gb.State.Executors[currentLogin]

	_, isDelegate := gb.RunContext.Delegations.FindDelegatorFor(currentLogin,
		getSliceFromMapOfStrings(gb.State.Executors))
	if !(executorFound || isDelegate) {
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

	slaInfoPtr, getSlaInfoErr := gb.RunContext.Services.SLAService.GetSLAInfoPtr(ctx, sla.InfoDto{
		TaskCompletionIntervals: []entity.TaskCompletionInterval{{StartedAt: gb.RunContext.CurrBlockStartTime,
			FinishedAt: gb.RunContext.CurrBlockStartTime.Add(time.Hour * 24 * 100)}},
		WorkType: sla.WorkHourType(gb.State.WorkType),
	})

	if getSlaInfoErr != nil {
		return getSlaInfoErr
	}
	workHours := gb.RunContext.Services.SLAService.GetWorkHoursBetweenDates(gb.RunContext.CurrBlockStartTime,
		time.Now(), slaInfoPtr)
	gb.State.IncreaseSLA(workHours)

	if !gb.RunContext.skipNotifications {
		if err = gb.emailGroupExecutors(ctx, gb.RunContext.UpdateData.ByLogin, executorLogins); err != nil {
			return err
		}
	}

	return nil
}

// nolint:gocyclo // mb later
func (gb *GoExecutionBlock) emailGroupExecutors(ctx c.Context, loginTakenInWork string, logins map[string]struct{}) (err error) {
	log := logger.GetLogger(ctx)

	executors := getSliceFromMapOfStrings(logins)
	log.WithField("func", "emailGroupExecutors").WithField("logins", logins)

	delegates, err := gb.RunContext.Services.HumanTasks.GetDelegationsByLogins(ctx, executors)
	if err != nil {
		return err
	}

	delegates = delegates.FilterByType("execution")

	loginsToNotify := delegates.GetUserInArrayWithDelegations(executors)

	emails := make([]string, 0, len(executors))
	for _, login := range loginsToNotify {
		if login != loginTakenInWork {
			email, emailErr := gb.RunContext.Services.People.GetUserEmail(ctx, login)
			if emailErr != nil {
				log.WithField("login", login).WithError(emailErr).Warning("couldn't get email")
				continue
			}

			emails = append(emails, email)
		}
	}

	log.WithField("func", "emailGroupExecutors").WithField("emails", emails)

	description, files, err := gb.RunContext.makeNotificationDescription(gb.Name)
	if err != nil {
		return err
	}

	author, err := gb.RunContext.Services.People.GetUser(ctx, gb.RunContext.UpdateData.ByLogin)
	if err != nil {
		return err
	}

	typedAuthor, err := author.ToUserinfo()
	if err != nil {
		return err
	}

	initiator, err := gb.RunContext.Services.People.GetUser(ctx, gb.RunContext.Initiator)
	if err != nil {
		return err
	}

	initiatorInfo, err := initiator.ToUserinfo()
	if err != nil {
		return err
	}

	task, getVersionErr := gb.RunContext.Services.Storage.GetVersionByWorkNumber(ctx, gb.RunContext.WorkNumber)
	if getVersionErr != nil {
		return getVersionErr
	}

	processSettings, getVersionErr := gb.RunContext.Services.Storage.GetVersionSettings(ctx, task.VersionID.String())
	if getVersionErr != nil {
		return getVersionErr
	}

	taskRunContext, getDataErr := gb.RunContext.Services.Storage.GetTaskRunContext(ctx, gb.RunContext.WorkNumber)
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
		lastWorksForUser, getWorksErr = gb.RunContext.Services.Storage.GetWorksForUserWithGivenTimeRange(ctx,
			processSettings.ResubmissionPeriod,
			login,
			task.VersionID.String(),
			gb.RunContext.WorkNumber,
		)
		if getWorksErr != nil {
			return getWorksErr
		}
	}

	tpl := mail.NewExecutionTakenInWorkTpl(&mail.ExecutorNotifTemplate{
		WorkNumber:  gb.RunContext.WorkNumber,
		Name:        gb.RunContext.NotifName,
		SdUrl:       gb.RunContext.Services.Sender.SdAddress,
		Description: description,
		Executor:    typedAuthor,
		Initiator:   initiatorInfo,
		LastWorks:   lastWorksForUser,
		Mailto:      gb.RunContext.Services.Sender.FetchEmail,
	})

	iconsName := []string{tpl.Image, userImg}

	if len(lastWorksForUser) != 0 {
		iconsName = append(iconsName, warningImg)
	}

	for _, v := range description {
		links, link := v.Get("attachLinks")
		if link {
			attachFiles, ok := links.([]file_registry.AttachInfo)
			if ok && len(attachFiles) != 0 {
				descIcons := []string{downloadImg}
				iconsName = append(iconsName, descIcons...)
				break
			}
		}
	}

	iconFiles, err := gb.RunContext.GetIcons(iconsName)
	if err != nil {
		return err
	}
	files = append(files, iconFiles...)

	if errSend := gb.RunContext.Services.Sender.SendNotification(ctx, emails, files, tpl); errSend != nil {
		return errSend
	}

	emailTakenInWork, emailErr := gb.RunContext.Services.People.GetUserEmail(ctx, loginTakenInWork)
	if emailErr != nil {
		return emailErr
	}

	slaInfoPtr, getSlaInfoErr := gb.RunContext.Services.SLAService.GetSLAInfoPtr(ctx, sla.InfoDto{
		TaskCompletionIntervals: []entity.TaskCompletionInterval{{StartedAt: gb.RunContext.CurrBlockStartTime,
			FinishedAt: gb.RunContext.CurrBlockStartTime.Add(time.Hour * 24 * 100)}},
		WorkType: sla.WorkHourType(gb.State.WorkType),
	})

	if getSlaInfoErr != nil {
		return getSlaInfoErr
	}

	author1, getUserErr := gb.RunContext.Services.People.GetUser(ctx, gb.RunContext.Initiator)
	if getUserErr != nil {
		return err
	}

	initiatorInfo, toUserErr := author1.ToUserinfo()
	if toUserErr != nil {
		return err
	}

	var buttons []mail.Button
	tpl, buttons = mail.NewAppPersonStatusNotificationTpl(
		&mail.NewAppPersonStatusTpl{
			WorkNumber:  gb.RunContext.WorkNumber,
			Name:        gb.RunContext.NotifName,
			Status:      string(StatusExecution),
			Action:      statusToTaskAction[StatusExecution],
			DeadLine:    gb.RunContext.Services.SLAService.ComputeMaxDateFormatted(time.Now(), gb.State.SLA, slaInfoPtr),
			Description: description,
			SdUrl:       gb.RunContext.Services.Sender.SdAddress,
			Mailto:      gb.RunContext.Services.Sender.FetchEmail,
			Login:       loginTakenInWork,
			IsEditable:  gb.State.GetIsEditable(),
			Initiator:   initiatorInfo,

			BlockID:                   BlockGoExecutionID,
			ExecutionDecisionExecuted: string(ExecutionDecisionExecuted),
			ExecutionDecisionRejected: string(ExecutionDecisionRejected),
			LastWorks:                 lastWorksForUser,
		})

	iconsName = []string{tpl.Image, userImg}

	for _, v := range buttons {
		iconsName = append(iconsName, v.Img)
	}

	if len(lastWorksForUser) != 0 {
		iconsName = append(iconsName, warningImg)
	}

	for _, v := range description {
		links, link := v.Get("attachLinks")
		if link {
			attachFiles, ok := links.([]file_registry.AttachInfo)
			if ok && len(attachFiles) != 0 {
				iconsName = append(iconsName, downloadImg)
				break
			}
		}
	}

	attachFiles, err := gb.RunContext.GetIcons(iconsName)
	if err != nil {
		return err
	}

	if sendErr := gb.RunContext.Services.Sender.SendNotification(ctx,
		[]string{emailTakenInWork}, attachFiles, tpl); sendErr != nil {
		return sendErr
	}

	return nil
}

type executorUpdateEditParams struct {
	Comment     string              `json:"comment"`
	Attachments []entity.Attachment `json:"attachments"`
}

//nolint:gocyclo //its ok here
func (gb *GoExecutionBlock) toEditApplication(ctx c.Context) (err error) {
	if gb.State.Decision != nil {
		return errors.New("decision already set")
	}

	var updateParams executorUpdateEditParams
	if err = json.Unmarshal(gb.RunContext.UpdateData.Parameters, &updateParams); err != nil {
		return errors.New("can't assert provided update data")
	}

	byLogin := gb.RunContext.UpdateData.ByLogin
	_, executorFound := gb.State.Executors[byLogin]

	delegateFor, isDelegate := gb.RunContext.Delegations.FindDelegatorFor(byLogin,
		getSliceFromMapOfStrings(gb.State.Executors))
	if !(executorFound || isDelegate) {
		return NewUserIsNotPartOfProcessErr()
	}

	// возврат на доработку всей заявки инициатору
	if gb.isNextBlockServiceDesk() {
		if editErr := gb.State.setEditAppToInitiator(gb.RunContext.UpdateData.ByLogin, delegateFor,
			updateParams); editErr != nil {
			return editErr
		}

		if err = gb.notifyNeedRework(ctx); err != nil {
			return err
		}

		err = gb.RunContext.Services.Storage.FinishTaskBlocks(ctx, gb.RunContext.TaskID, []string{gb.Name}, false)
		if err != nil {
			return err
		}
	} else {
		if editErr := gb.State.setEditToNextBlock(gb.RunContext.UpdateData.ByLogin, delegateFor,
			updateParams); editErr != nil {
			return editErr
		}

		person, personErr := gb.RunContext.Services.ServiceDesc.GetSsoPerson(ctx, gb.RunContext.UpdateData.ByLogin)
		if personErr != nil {
			return personErr
		}

		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputExecutionLogin], person)
		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputDecision], ExecutionDecisionSentEdit)
		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputComment], updateParams.Comment)
	}

	return nil
}

func (gb *GoExecutionBlock) isNextBlockServiceDesk() bool {
	for i := range gb.Sockets {
		if gb.Sockets[i].Id == executionEditAppSocketID &&
			utils.IsContainsInSlice("servicedesk_application_0", gb.Sockets[i].NextBlockIds) {
			return true
		}
	}

	return false
}
