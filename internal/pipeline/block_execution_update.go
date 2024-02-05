package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/iancoleman/orderedmap"
	"github.com/pkg/errors"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/abp/mail/pkg/email"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	file_registry "gitlab.services.mts.ru/jocasta/pipeliner/internal/fileregistry"
	hs "gitlab.services.mts.ru/jocasta/pipeliner/internal/humantasks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sla"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

func (gb *GoExecutionBlock) Update(ctx context.Context) (interface{}, error) {
	err := gb.handleTaskUpdateAction(ctx)
	if err != nil {
		return nil, err
	}

	deadline, deadlineErr := gb.getDeadline(ctx, gb.State.WorkType)
	if deadlineErr != nil {
		return nil, deadlineErr
	}

	gb.State.Deadline = deadline

	var stateBytes []byte

	stateBytes, err = json.Marshal(gb.State)
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

func (gb *GoExecutionBlock) handleTaskUpdateAction(ctx context.Context) error {
	data := gb.RunContext.UpdateData
	if data == nil {
		return errors.New("empty data")
	}

	gb.RunContext.Delegations = gb.RunContext.Delegations.FilterByType("execution")

	err := gb.handleAction(ctx, entity.TaskUpdateAction(data.Action))
	if err != nil {
		return err
	}

	return nil
}

//nolint:gocognit,gocyclo // вся сложность функции состоит в switch case, под каждым вызывается одна-две функции
func (gb *GoExecutionBlock) handleAction(ctx context.Context, action entity.TaskUpdateAction) error {
	//nolint:exhaustive //нам не нужно обрабатывать остальные случаи
	switch action {
	case entity.TaskUpdateActionSLABreach:
		errUpdate := gb.handleBreachedSLA(ctx)
		if errUpdate != nil {
			return errUpdate
		}
	case entity.TaskUpdateActionHalfSLABreach:
		gb.handleHalfSLABreached(ctx)
	case entity.TaskUpdateActionReworkSLABreach:
		errUpdate := gb.handleReworkSLABreached(ctx)
		if errUpdate != nil {
			return errUpdate
		}
	case entity.TaskUpdateActionExecution:
		if !gb.State.IsTakenInWork {
			return errors.New("is not taken in work")
		}

		errUpdate := gb.updateDecision(ctx)
		if errUpdate != nil {
			return errUpdate
		}
	case entity.TaskUpdateActionChangeExecutor:
		if !gb.State.IsTakenInWork {
			return errors.New("is not taken in work")
		}

		errUpdate := gb.changeExecutor(ctx)
		if errUpdate != nil {
			return errUpdate
		}
	case entity.TaskUpdateActionRequestExecutionInfo:
		if !gb.State.IsTakenInWork {
			return errors.New("is not taken in work")
		}

		errUpdate := gb.updateRequestInfo(ctx)
		if errUpdate != nil {
			return errUpdate
		}
	case entity.TaskUpdateActionReplyExecutionInfo:
		if !gb.State.IsTakenInWork {
			return errors.New("is not taken in work")
		}

		errUpdate := gb.updateReplyInfo(ctx)
		if errUpdate != nil {
			return errUpdate
		}
	case entity.TaskUpdateActionExecutorStartWork:
		if gb.State.IsTakenInWork {
			return errors.New("is already taken in work")
		}

		errUpdate := gb.executorStartWork(ctx)
		if errUpdate != nil {
			return errUpdate
		}
	case entity.TaskUpdateActionExecutorSendEditApp:
		if !gb.State.IsTakenInWork {
			return errors.New("is not taken in work")
		}

		errUpdate := gb.toEditApplication(ctx)
		if errUpdate != nil {
			return errUpdate
		}
	case entity.TaskUpdateActionDayBeforeSLARequestAddInfo:
		errUpdate := gb.handleBreachedDayBeforeSLARequestAddInfo(ctx)
		if errUpdate != nil {
			return errUpdate
		}
	case entity.TaskUpdateActionSLABreachRequestAddInfo:
		errUpdate := gb.HandleBreachedSLARequestAddInfo(ctx)
		if errUpdate != nil {
			return errUpdate
		}
	}

	return nil
}

type ExecutorChangeParams struct {
	NewExecutorLogin string              `json:"new_executor_login"`
	Comment          string              `json:"comment"`
	Attachments      []entity.Attachment `json:"attachments,omitempty"`
}

func (gb *GoExecutionBlock) changeExecutor(ctx context.Context) (err error) {
	currentLogin := gb.RunContext.UpdateData.ByLogin
	_, executorFound := gb.State.Executors[currentLogin]

	delegateFor, isDelegate := gb.RunContext.Delegations.FindDelegatorFor(currentLogin, getSliceFromMapOfStrings(gb.State.Executors))
	if !(executorFound || isDelegate) {
		return NewUserIsNotPartOfProcessErr()
	}

	var updateParams ExecutorChangeParams
	if err = json.Unmarshal(gb.RunContext.UpdateData.Parameters, &updateParams); err != nil {
		return errors.New("can't assert provided update data")
	}

	if err = gb.State.SetChangeExecutor(gb.RunContext.UpdateData.ByLogin, delegateFor, &updateParams); err != nil {
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

func (a *ExecutionData) SetChangeExecutor(oldLogin, delegateFor string, in *ExecutorChangeParams) error {
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
		DelegateFor: delegateFor,
	})

	return nil
}

type ExecutionUpdateParams struct {
	Decision    ExecutionDecision   `json:"decision"`
	Comment     string              `json:"comment"`
	Attachments []entity.Attachment `json:"attachments"`
}

//nolint:dupl //its not duplicate
func (gb *GoExecutionBlock) handleBreachedSLA(ctx context.Context) error {
	if !gb.State.CheckSLA {
		gb.State.SLAChecked = true
		gb.State.HalfSLAChecked = true

		return nil
	}

	if gb.State.SLA >= 8 {
		err := gb.checkBreachedSLA(ctx)
		if err != nil {
			return err
		}
	}

	gb.State.SLAChecked = true
	gb.State.HalfSLAChecked = true

	return nil
}

func (gb *GoExecutionBlock) checkBreachedSLA(ctx context.Context) error {
	const fn = "pipeline.execution.checkBreachedSLA"

	log := logger.GetLogger(ctx)

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

	icons, iconEerr := gb.RunContext.GetIcons(filesList)
	if iconEerr != nil {
		return iconEerr
	}

	if len(emails) == 0 {
		return nil
	}

	err = gb.RunContext.Services.Sender.SendNotification(ctx, emails, icons, tpl)
	if err != nil {
		return err
	}

	return nil
}

//nolint:dupl //its not duplicate
func (gb *GoExecutionBlock) handleHalfSLABreached(ctx context.Context) {
	const fn = "pipeline.execution.handleHalfSLABreached"

	if !gb.State.CheckSLA {
		gb.State.SLAChecked = true
		gb.State.HalfSLAChecked = true

		return
	}

	log := logger.GetLogger(ctx)

	if gb.State.SLA >= 8 {
		_ = gb.sendNotification(ctx, log, fn)
	}

	gb.State.HalfSLAChecked = true
}

func (gb *GoExecutionBlock) sendNotification(ctx context.Context, log logger.Logger, fn string) error {
	emails := make([]string, 0, len(gb.State.Executors))
	logins := getSliceFromMapOfStrings(gb.State.Executors)

	delegations, err := gb.RunContext.Services.HumanTasks.GetDelegationsByLogins(ctx, logins)
	if err != nil {
		log.WithError(err).Info(fn, fmt.Sprintf("executors %v have no delegates", logins))
	}

	delegations = delegations.FilterByType("execution")
	logins = delegations.GetUserInArrayWithDelegations(logins)

	for i := range logins {
		executorEmail, getEmailErr := gb.RunContext.Services.People.GetUserEmail(ctx, logins[i])
		if getEmailErr != nil {
			log.WithError(getEmailErr).Warning(fn, fmt.Sprintf("executor login %s not found", logins[i]))

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
		_, getWorksErr := gb.RunContext.Services.Storage.GetWorksForUserWithGivenTimeRange(ctx,
			processSettings.ResubmissionPeriod,
			login,
			task.VersionID.String(),
			gb.RunContext.WorkNumber,
		)
		if getWorksErr != nil {
			return getWorksErr
		}
	}

	slaInfoPtr, getSLAInfoErr := gb.RunContext.Services.SLAService.GetSLAInfoPtr(
		ctx,
		sla.InfoDTO{
			TaskCompletionIntervals: []entity.TaskCompletionInterval{
				{
					StartedAt:  gb.RunContext.CurrBlockStartTime,
					FinishedAt: gb.RunContext.CurrBlockStartTime.Add(time.Hour * 24 * 100),
				},
			},
			WorkType: sla.WorkHourType(gb.State.WorkType),
		},
	)
	if getSLAInfoErr != nil {
		return getSLAInfoErr
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

	return nil
}

// nolint:dupl // another action
func (gb *GoExecutionBlock) handleReworkSLABreached(ctx context.Context) error {
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

	var (
		em  string
		err error
	)

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

	//nolint:gocritic //в этом проекте не принято использовать поинтеры в коллекциях
	for _, event := range nodeEvents {
		// event for this node will spawn later
		if event.NodeName == gb.Name {
			continue
		}

		gb.happenedEvents = append(gb.happenedEvents, event)
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
		userEmail, err := gb.RunContext.Services.People.GetUserEmail(ctx, login)
		if err != nil {
			log.WithError(err).Warning(fn, fmt.Sprintf("login %s not found", login))

			continue
		}

		emails = append(emails, userEmail)
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
func (gb *GoExecutionBlock) HandleBreachedSLARequestAddInfo(ctx context.Context) error {
	const fn = "pipeline.execution.HandleBreachedSLARequestAddInfo"

	comment := "заявка автоматически перенесена в архив по истечении 3 дней"

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

	var (
		em  string
		err error
	)

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

	//nolint:gocritic //в этом проекте не принято использовать поинтеры в коллекциях
	for _, event := range nodeEvents {
		// event for this node will spawn later
		if event.NodeName == gb.Name {
			continue
		}

		gb.happenedEvents = append(gb.happenedEvents, event)
	}

	return nil
}

func (gb *GoExecutionBlock) updateDecision(ctx context.Context) error {
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

type requestInfoUpdateParams struct {
	Comment       string              `json:"comment"`
	ReqType       RequestInfoType     `json:"req_type"`
	Attachments   []entity.Attachment `json:"attachments"`
	ExecutorLogin string              `json:"executor_login"`
}

type replyInfoUpdateParams struct {
	Comment       string              `json:"comment"`
	Attachments   []entity.Attachment `json:"attachments"`
	ExecutorLogin string              `json:"executor_login"`
}

func (gb *GoExecutionBlock) updateRequestInfo(ctx context.Context) (err error) {
	var updateParams requestInfoUpdateParams

	delegations := gb.RunContext.Delegations.FilterByType("execution")

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

func (gb *GoExecutionBlock) updateReplyInfo(ctx context.Context) (err error) {
	if gb.RunContext.UpdateData.ByLogin != gb.RunContext.Initiator {
		return NewUserIsNotPartOfProcessErr()
	}

	var updateParams replyInfoUpdateParams

	err = json.Unmarshal(gb.RunContext.UpdateData.Parameters, &updateParams)
	if err != nil {
		return errors.New("can't assert provided update replyInfoUpdateParams data")
	}

	errSet := gb.State.setReplyExecutionInfo(gb.RunContext.UpdateData.ByLogin, &updateParams)
	if errSet != nil {
		return errSet
	}

	err = gb.notifyNewInfoReceived(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (a *ExecutionData) setReplyExecutionInfo(login string, in *replyInfoUpdateParams) error {
	a.RequestExecutionInfoLogs = append(a.RequestExecutionInfoLogs, RequestExecutionInfoLog{
		Login:       login,
		Comment:     in.Comment,
		CreatedAt:   time.Now(),
		ReqType:     RequestInfoAnswer,
		Attachments: in.Attachments,
		DelegateFor: "",
	})

	return nil
}

func (a *ExecutionData) SetRequestExecutionInfo(
	login string,
	delegations hs.Delegations,
	in *requestInfoUpdateParams,
) error {
	_, executorFound := a.Executors[login]
	delegateFor, isDelegate := delegations.FindDelegatorFor(
		login,
		getSliceFromMapOfStrings(a.Executors),
	)

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

func (gb *GoExecutionBlock) executorStartWork(ctx context.Context) (err error) {
	currentLogin := gb.RunContext.UpdateData.ByLogin
	_, executorFound := gb.State.Executors[currentLogin]

	delegateFor, isDelegate := gb.RunContext.Delegations.FindDelegatorFor(
		currentLogin,
		getSliceFromMapOfStrings(gb.State.Executors),
	)
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
	gb.State.TakenInWorkLog = append(gb.State.TakenInWorkLog, StartWorkLog{
		Executor:    gb.RunContext.UpdateData.ByLogin,
		CreatedAt:   time.Now(),
		DelegateFor: delegateFor,
	})

	slaInfoPtr, getSLAInfoErr := gb.RunContext.Services.SLAService.GetSLAInfoPtr(ctx, sla.InfoDTO{
		TaskCompletionIntervals: []entity.TaskCompletionInterval{{
			StartedAt:  gb.RunContext.CurrBlockStartTime,
			FinishedAt: gb.RunContext.CurrBlockStartTime.Add(time.Hour * 24 * 100),
		}},
		WorkType: sla.WorkHourType(gb.State.WorkType),
	})
	if getSLAInfoErr != nil {
		return getSLAInfoErr
	}

	workHours := gb.RunContext.Services.SLAService.GetWorkHoursBetweenDates(
		gb.RunContext.CurrBlockStartTime,
		time.Now(),
		slaInfoPtr,
	)
	gb.State.IncreaseSLA(workHours)

	if gb.RunContext.skipNotifications {
		return nil
	}

	err = gb.emailGroupExecutors(ctx, gb.RunContext.UpdateData.ByLogin, executorLogins)
	if err != nil {
		return err
	}

	return nil
}

func (gb *GoExecutionBlock) emailGroupExecutors(ctx context.Context, loginTakenInWork string, logins map[string]struct{}) (err error) {
	log := logger.GetLogger(ctx)

	executors := getSliceFromMapOfStrings(logins)
	log.WithField("func", "emailGroupExecutors").WithField("logins", logins)

	delegates, err := gb.RunContext.Services.HumanTasks.GetDelegationsByLogins(ctx, executors)
	if err != nil {
		return err
	}

	delegates = delegates.FilterByType("execution")

	loginsToNotify := delegates.GetUserInArrayWithDelegations(executors)

	emails := gb.mapLoginsToEmails(ctx, loginsToNotify, loginTakenInWork)

	log.WithField("func", "emailGroupExecutors").WithField("emails", emails)

	description, files, err := gb.RunContext.makeNotificationDescription(gb.Name)
	if err != nil {
		return err
	}

	typedAuthor, err := gb.typedAuthor(ctx)
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

	lastWorksForUser, err := gb.lastWorksForUser(ctx, &processSettings, login, task)
	if err != nil {
		return err
	}

	initiatorInfo, err := gb.initiatorInfo(ctx)
	if err != nil {
		return err
	}

	tpl := mail.NewExecutionTakenInWorkTpl(
		&mail.ExecutorNotifTemplate{
			WorkNumber:  gb.RunContext.WorkNumber,
			Name:        gb.RunContext.NotifName,
			SdURL:       gb.RunContext.Services.Sender.SdAddress,
			Description: description,
			Executor:    typedAuthor,
			Initiator:   initiatorInfo,
			LastWorks:   lastWorksForUser,
			Mailto:      gb.RunContext.Services.Sender.FetchEmail,
		},
	)

	iconsName := []string{tpl.Image, userImg}

	if len(lastWorksForUser) != 0 {
		iconsName = append(iconsName, warningImg)
	}

	if gb.downloadImgFromDescription(description) {
		iconsName = append(iconsName, downloadImg)
	}

	iconFiles, err := gb.RunContext.GetIcons(iconsName)
	if err != nil {
		return err
	}

	iconFiles = append(iconFiles, files...)

	if errSend := gb.RunContext.Services.Sender.SendNotification(ctx, emails, iconFiles, tpl); errSend != nil {
		return errSend
	}

	emailTakenInWork, emailErr := gb.RunContext.Services.People.GetUserEmail(ctx, loginTakenInWork)
	if emailErr != nil {
		return emailErr
	}

	slaInfoPtr, getSLAInfoErr := gb.RunContext.Services.SLAService.GetSLAInfoPtr(
		ctx,
		sla.InfoDTO{
			TaskCompletionIntervals: []entity.TaskCompletionInterval{
				{
					StartedAt:  gb.RunContext.CurrBlockStartTime,
					FinishedAt: gb.RunContext.CurrBlockStartTime.Add(time.Hour * 24 * 100),
				},
			},
			WorkType: sla.WorkHourType(gb.State.WorkType),
		},
	)
	if getSLAInfoErr != nil {
		return getSLAInfoErr
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
			SdURL:       gb.RunContext.Services.Sender.SdAddress,
			Mailto:      gb.RunContext.Services.Sender.FetchEmail,
			Login:       loginTakenInWork,
			IsEditable:  gb.State.GetIsEditable(),
			Initiator:   initiatorInfo,

			BlockID:                   BlockGoExecutionID,
			ExecutionDecisionExecuted: string(ExecutionDecisionExecuted),
			ExecutionDecisionRejected: string(ExecutionDecisionRejected),
			LastWorks:                 lastWorksForUser,
		},
	)

	attachFiles, err := gb.attachFiles(&tpl, buttons, lastWorksForUser, description)
	if err != nil {
		return err
	}

	sendErr := gb.RunContext.Services.Sender.SendNotification(
		ctx,
		[]string{emailTakenInWork},
		attachFiles,
		tpl,
	)
	if sendErr != nil {
		return sendErr
	}

	return nil
}

func (gb *GoExecutionBlock) attachFiles(
	tpl *mail.Template,
	buttons []mail.Button,
	lastWorksForUser []*entity.EriusTask,
	description []orderedmap.OrderedMap,
) ([]email.Attachment, error) {
	iconsName := []string{tpl.Image, userImg}

	for _, v := range buttons {
		iconsName = append(iconsName, v.Img)
	}

	if len(lastWorksForUser) != 0 {
		iconsName = append(iconsName, warningImg)
	}

	if isNeedAddDownloadImage(description) {
		iconsName = append(iconsName, downloadImg)
	}

	attachFiles, err := gb.RunContext.GetIcons(iconsName)
	if err != nil {
		return nil, err
	}

	return attachFiles, nil
}

func (gb *GoExecutionBlock) mapLoginsToEmails(ctx context.Context, loginsToNotify []string, loginTakenInWork string) []string {
	log := logger.GetLogger(ctx)
	emails := make([]string, 0)

	for _, login := range loginsToNotify {
		if login != loginTakenInWork {
			userEmail, emailErr := gb.RunContext.Services.People.GetUserEmail(ctx, login)
			if emailErr != nil {
				log.WithField("login", login).WithError(emailErr).Warning("couldn't get email")

				continue
			}

			emails = append(emails, userEmail)
		}
	}

	return emails
}

func (gb *GoExecutionBlock) typedAuthor(ctx context.Context) (*sso.UserInfo, error) {
	author, err := gb.RunContext.Services.People.GetUser(ctx, gb.RunContext.UpdateData.ByLogin)
	if err != nil {
		return nil, err
	}

	typedAuthor, err := author.ToUserinfo()
	if err != nil {
		return nil, err
	}

	return typedAuthor, nil
}

func (gb *GoExecutionBlock) initiatorInfo(ctx context.Context) (*sso.UserInfo, error) {
	initiator, err := gb.RunContext.Services.People.GetUser(ctx, gb.RunContext.Initiator)
	if err != nil {
		return nil, err
	}

	initiatorInfo, err := initiator.ToUserinfo()
	if err != nil {
		return nil, err
	}

	return initiatorInfo, nil
}

func (gb *GoExecutionBlock) downloadImgFromDescription(description []orderedmap.OrderedMap) bool {
	for _, v := range description {
		links, link := v.Get("attachLinks")
		if link {
			attachFiles, ok := links.([]file_registry.AttachInfo)
			if ok && len(attachFiles) != 0 {
				return true
			}
		}
	}

	return false
}

func (gb *GoExecutionBlock) lastWorksForUser(
	ctx context.Context,
	processSettings *entity.ProcessSettings,
	login string,
	task *entity.EriusScenario,
) ([]*entity.EriusTask, error) {
	if processSettings.ResubmissionPeriod > 0 {
		lastWorksForUser, getWorksErr := gb.RunContext.Services.Storage.GetWorksForUserWithGivenTimeRange(
			ctx,
			processSettings.ResubmissionPeriod,
			login,
			task.VersionID.String(),
			gb.RunContext.WorkNumber,
		)
		if getWorksErr != nil {
			return make([]*entity.EriusTask, 0), getWorksErr
		}

		return lastWorksForUser, nil
	}

	return make([]*entity.EriusTask, 0), nil
}

type executorUpdateEditParams struct {
	Comment     string              `json:"comment"`
	Attachments []entity.Attachment `json:"attachments"`
}

func (gb *GoExecutionBlock) toEditApplication(ctx context.Context) (err error) {
	if gb.State.Decision != nil {
		return errors.New("decision already set")
	}

	var updateParams executorUpdateEditParams
	if err = json.Unmarshal(gb.RunContext.UpdateData.Parameters, &updateParams); err != nil {
		return errors.New("can't assert provided update data")
	}

	byLogin := gb.RunContext.UpdateData.ByLogin
	_, executorFound := gb.State.Executors[byLogin]

	delegateFor, isDelegate := gb.RunContext.Delegations.FindDelegatorFor(
		byLogin,
		getSliceFromMapOfStrings(gb.State.Executors),
	)
	if !(executorFound || isDelegate) {
		return NewUserIsNotPartOfProcessErr()
	}

	// возврат на доработку всей заявки инициатору
	if gb.isNextBlockServiceDesk() {
		err := gb.returnToAdminForRevision(ctx, delegateFor, updateParams)
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
		if gb.Sockets[i].ID == executionEditAppSocketID &&
			utils.IsContainsInSlice("servicedesk_application_0", gb.Sockets[i].NextBlockIds) {
			return true
		}
	}

	return false
}

func (gb *GoExecutionBlock) returnToAdminForRevision(
	ctx context.Context,
	delegateFor string,
	updateParams executorUpdateEditParams,
) (err error) {
	err = gb.State.setEditAppToInitiator(
		gb.RunContext.UpdateData.ByLogin,
		delegateFor,
		updateParams,
	)
	if err != nil {
		return err
	}

	err = gb.notifyNeedRework(ctx)
	if err != nil {
		return err
	}

	err = gb.RunContext.Services.Storage.FinishTaskBlocks(ctx, gb.RunContext.TaskID, []string{gb.Name}, false)
	if err != nil {
		return err
	}

	return nil
}
