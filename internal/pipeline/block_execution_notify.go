package pipeline

import (
	c "context"
	"time"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	file_registry "gitlab.services.mts.ru/jocasta/pipeliner/internal/file-registry"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sla"
)

const (
	downloadImg = "iconDownload.svg"
	documentImg = "iconDocument.svg"
)

//nolint:dupl,gocyclo // maybe later
func (gb *GoExecutionBlock) handleNotifications(ctx c.Context) error {
	if gb.RunContext.skipNotifications {
		return nil
	}

	l := logger.GetLogger(ctx)

	executors := getSliceFromMapOfStrings(gb.State.Executors)
	delegates, getDelegationsErr := gb.RunContext.Services.HumanTasks.GetDelegationsByLogins(ctx, executors)
	if getDelegationsErr != nil {
		return getDelegationsErr
	}
	delegates = delegates.FilterByType("execution")

	loginsToNotify := delegates.GetUserInArrayWithDelegations(executors)

	description, files, err := gb.RunContext.makeNotificationDescription(gb.Name)
	if err != nil {
		return err
	}

	emails := make(map[string]mail.Template, 0)

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
		lastWorksForUser, getWorksErr = gb.RunContext.Services.Storage.GetWorksForUserWithGivenTimeRange(
			ctx,
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

	initiator := false

	if getSlaInfoErr != nil {
		return getSlaInfoErr
	}
	for _, login = range loginsToNotify {
		email, getUserEmailErr := gb.RunContext.Services.People.GetUserEmail(ctx, login)
		if getUserEmailErr != nil {
			l.WithField("login", login).WithError(getUserEmailErr).Warning("couldn't get email")
			continue
		}
		if !gb.State.IsTakenInWork {
			emails[email] = mail.NewExecutionNeedTakeInWorkTpl(
				&mail.ExecutorNotifTemplate{
					WorkNumber:  gb.RunContext.WorkNumber,
					Name:        gb.RunContext.NotifName,
					SdUrl:       gb.RunContext.Services.Sender.SdAddress,
					BlockID:     BlockGoExecutionID,
					Description: description,
					Mailto:      gb.RunContext.Services.Sender.FetchEmail,
					Login:       login,
					LastWorks:   lastWorksForUser,
					IsGroup:     len(gb.State.Executors) > 1,
					Deadline:    gb.RunContext.Services.SLAService.ComputeMaxDateFormatted(time.Now(), gb.State.SLA, slaInfoPtr),
				},
			)
		} else {
			author, errAuthor := gb.RunContext.Services.People.GetUser(ctx, gb.RunContext.Initiator)
			if errAuthor != nil {
				return err
			}

			initiatorInfo, errInitiator := author.ToUserinfo()
			if errInitiator != nil {
				return err
			}

			initiator = true

			emails[email] = mail.NewAppPersonStatusNotificationTpl(
				&mail.NewAppPersonStatusTpl{
					WorkNumber:  gb.RunContext.WorkNumber,
					Name:        gb.RunContext.NotifName,
					Status:      string(StatusExecution),
					Action:      statusToTaskAction[StatusExecution],
					DeadLine:    gb.RunContext.Services.SLAService.ComputeMaxDateFormatted(time.Now(), gb.State.SLA, slaInfoPtr),
					Description: description,
					SdUrl:       gb.RunContext.Services.Sender.SdAddress,
					Mailto:      gb.RunContext.Services.Sender.FetchEmail,
					Login:       login,
					IsEditable:  gb.State.GetIsEditable(),
					Initiator:   initiatorInfo,

					BlockID:                   BlockGoExecutionID,
					ExecutionDecisionExecuted: string(ExecutionDecisionExecuted),
					ExecutionDecisionRejected: string(ExecutionDecisionRejected),
					LastWorks:                 lastWorksForUser,
				})
		}
	}

	for i := range emails {
		item := emails[i]

		iconsName := []string{item.Image}

		if initiator {
			iconsName = append(iconsName, userImg)
		}

		if len(lastWorksForUser) != 0 {
			iconsName = append(iconsName, warningImg)
		}

		for _, v := range description {
			links, link := v.Get("attachLinks")
			if link {
				attachFiles, ok := links.([]file_registry.AttachInfo)
				if ok && len(attachFiles) != 0 {
					descIcons := []string{documentImg, downloadImg}
					iconsName = append(iconsName, descIcons...)
					break
				}
			}
		}

		iconFiles, errFiles := gb.RunContext.GetIcons(iconsName)
		if err != nil {
			return errFiles
		}

		files = append(files, iconFiles...)

		if sendErr := gb.RunContext.Services.Sender.SendNotification(ctx, []string{i}, files,
			emails[i]); sendErr != nil {
			return sendErr
		}
	}

	return nil
}

func (gb *GoExecutionBlock) notifyNeedRework(ctx c.Context) error {
	l := logger.GetLogger(ctx)

	delegates, err := gb.RunContext.Services.HumanTasks.GetDelegationsFromLogin(ctx, gb.RunContext.Initiator)
	if err != nil {
		return err
	}

	loginsToNotify := delegates.GetUserInArrayWithDelegations([]string{gb.RunContext.Initiator})

	var em string
	emails := make([]string, 0, len(loginsToNotify))
	for _, login := range loginsToNotify {
		em, err = gb.RunContext.Services.People.GetUserEmail(ctx, login)
		if err != nil {
			l.WithField("login", login).WithError(err).Warning("couldn't get email")
			continue
		}

		emails = append(emails, em)
	}
	tpl := mail.NewSendToInitiatorEditTpl(gb.RunContext.WorkNumber, gb.RunContext.NotifName,
		gb.RunContext.Services.Sender.SdAddress)

	filesList := []string{tpl.Image}
	files, iconEerr := gb.RunContext.GetIcons(filesList)
	if iconEerr != nil {
		return iconEerr
	}

	if sendErr := gb.RunContext.Services.Sender.SendNotification(ctx, emails, files, tpl); sendErr != nil {
		return sendErr
	}

	return nil
}

// 22 (Soglasovanie analogichno)
func (gb *GoExecutionBlock) notifyNeedMoreInfo(ctx c.Context) error {
	loginsToNotify := []string{gb.RunContext.Initiator}

	emails := make([]string, 0, len(loginsToNotify))
	for _, login := range loginsToNotify {
		email, err := gb.RunContext.Services.People.GetUserEmail(ctx, login)
		if err != nil {
			return err
		}

		emails = append(emails, email)
	}
	tpl := mail.NewRequestExecutionInfoTpl(gb.RunContext.WorkNumber,
		gb.RunContext.NotifName, gb.RunContext.Services.Sender.SdAddress)

	filesList := []string{tpl.Image}
	files, iconEerr := gb.RunContext.GetIcons(filesList)
	if iconEerr != nil {
		return iconEerr
	}

	if err := gb.RunContext.Services.Sender.SendNotification(ctx, emails, files, tpl); err != nil {
		return err
	}

	return nil
}

func (gb *GoExecutionBlock) notifyNewInfoReceived(ctx c.Context) error {
	delegates, err := gb.RunContext.Services.HumanTasks.GetDelegationsByLogins(ctx,
		getSliceFromMapOfStrings(gb.State.Executors))
	if err != nil {
		return err
	}

	loginsToNotify := delegates.GetUserInArrayWithDelegations(getSliceFromMapOfStrings(gb.State.Executors))

	var email string
	emails := make([]string, 0, len(loginsToNotify))
	for _, login := range loginsToNotify {
		email, err = gb.RunContext.Services.People.GetUserEmail(ctx, login)
		if err != nil {
			continue
		}

		emails = append(emails, email)
	}
	tpl := mail.NewAnswerExecutionInfoTpl(gb.RunContext.WorkNumber,
		gb.RunContext.NotifName, gb.RunContext.Services.Sender.SdAddress)

	files := []string{tpl.Image}
	iconFiles, iconEerr := gb.RunContext.GetIcons(files)
	if iconEerr != nil {
		return iconEerr
	}

	if sendErr := gb.RunContext.Services.Sender.SendNotification(ctx, emails, iconFiles, tpl); sendErr != nil {
		return sendErr
	}

	return nil
}
