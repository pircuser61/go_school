package pipeline

import (
	c "context"
	"time"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	file_registry "gitlab.services.mts.ru/jocasta/pipeliner/internal/file-registry"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sla"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

const (
	headImg    = "header.png"
	userImg    = "iconUser.png"
	warningImg = "warning.png"
	vRabotuBtn = "v_rabotu.png"
)

//nolint:dupl // maybe later
func (gb *GoApproverBlock) handleNotifications(ctx c.Context) error {
	if gb.RunContext.skipNotifications {
		return nil
	}

	l := logger.GetLogger(ctx)

	delegates, getDelegationsErr := gb.RunContext.Services.HumanTasks.GetDelegationsByLogins(
		ctx, getSliceFromMapOfStrings(gb.State.Approvers))
	if getDelegationsErr != nil {
		return getDelegationsErr
	}
	delegates = delegates.FilterByType("approvement")

	approvers := getSliceFromMapOfStrings(gb.State.Approvers)
	loginsToNotify := delegates.GetUserInArrayWithDelegations(approvers)

	description, files, err := gb.RunContext.makeNotificationDescription(gb.Name)

	if err != nil {
		return err
	}

	actionsList := make([]mail.Action, 0, len(gb.State.ActionList))
	for i := range gb.State.ActionList {
		actionsList = append(actionsList, mail.Action{
			InternalActionName: gb.State.ActionList[i].Id,
			Title:              gb.State.ActionList[i].Title,
		})
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

	templates := make(map[string]mail.Template, 0)
	slaInfoPtr, getSlaInfoErr := gb.RunContext.Services.SLAService.GetSLAInfoPtr(ctx, sla.InfoDto{
		TaskCompletionIntervals: []entity.TaskCompletionInterval{{StartedAt: gb.RunContext.CurrBlockStartTime,
			FinishedAt: gb.RunContext.CurrBlockStartTime.Add(time.Hour * 24 * 100)}},
		WorkType: sla.WorkHourType(gb.State.WorkType),
	})

	if getSlaInfoErr != nil {
		return getSlaInfoErr
	}

	var buttons []mail.Button
	buttonImg := make([]string, 0, 7)
	for _, login = range loginsToNotify {
		email, getEmailErr := gb.RunContext.Services.People.GetUserEmail(ctx, login)
		if getEmailErr != nil {
			l.WithField("login", login).WithError(getEmailErr).Warning("couldn't get email")
			continue
		}

		author, autorErr := gb.RunContext.Services.People.GetUser(ctx, gb.RunContext.Initiator)
		if autorErr != nil {
			return autorErr
		}

		initiatorInfo, initialErr := author.ToUserinfo()
		if initialErr != nil {
			return initialErr
		}

		tpl := &mail.NewAppPersonStatusTpl{
			WorkNumber: gb.RunContext.WorkNumber,
			Name:       gb.RunContext.NotifName,
			Status:     gb.State.ApproveStatusName,
			Action:     statusToTaskAction[StatusApprovement],
			DeadLine: gb.RunContext.Services.SLAService.ComputeMaxDateFormatted(
				time.Now(), gb.State.SLA, slaInfoPtr,
			),
			SdUrl:                     gb.RunContext.Services.Sender.SdAddress,
			Mailto:                    gb.RunContext.Services.Sender.FetchEmail,
			Login:                     login,
			IsEditable:                gb.State.GetIsEditable(),
			ApproverActions:           actionsList,
			Description:               description,
			BlockID:                   BlockGoApproverID,
			ExecutionDecisionExecuted: string(ExecutionDecisionExecuted),
			ExecutionDecisionRejected: string(ExecutionDecisionRejected),
			LastWorks:                 lastWorksForUser,
			Initiator:                 initiatorInfo,
		}

		templates[email], buttons = mail.NewAppPersonStatusNotificationTpl(tpl)

	}

	for _, v := range buttons {
		buttonImg = append(buttonImg, v.Img)
	}

	for i := range templates {
		item := templates[i]

		iconsName := []string{item.Image, userImg}
		iconsName = append(iconsName, buttonImg...)

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

		iconsFiles, iconsErr := gb.RunContext.GetIcons(iconsName)
		if iconsErr != nil {
			return iconsErr
		}
		files = append(files, iconsFiles...)

		if sendErr := gb.RunContext.Services.Sender.SendNotification(
			ctx, []string{i}, files, item,
		); sendErr != nil {
			return sendErr
		}
	}

	return nil
}

func (gb *GoApproverBlock) notifyAdditionalApprovers(ctx c.Context, logins []string, attachsId []entity.Attachment) error {
	delegates, err := gb.RunContext.Services.HumanTasks.GetDelegationsByLogins(ctx, logins)
	if err != nil {
		return err
	}
	delegates = delegates.FilterByType("approvement")

	loginsToNotify := delegates.GetUserInArrayWithDelegations(logins)

	emails := make([]string, 0, len(loginsToNotify))
	for _, login := range loginsToNotify {
		approverEmail, emailErr := gb.RunContext.Services.People.GetUserEmail(ctx, login)
		if emailErr != nil {
			return emailErr
		}

		emails = append(emails, approverEmail)
	}

	files, err := gb.RunContext.Services.FileRegistry.GetAttachments(ctx, attachsId)
	if err != nil {
		return err
	}

	emails = utils.UniqueStrings(emails)

	slaInfoPtr, getSlaInfoErr := gb.RunContext.Services.SLAService.GetSLAInfoPtr(ctx, sla.InfoDto{
		TaskCompletionIntervals: []entity.TaskCompletionInterval{{StartedAt: gb.RunContext.CurrBlockStartTime,
			FinishedAt: gb.RunContext.CurrBlockStartTime.Add(time.Hour * 24 * 100)}},
		WorkType: sla.WorkHourType(gb.State.WorkType),
	})

	if getSlaInfoErr != nil {
		return getSlaInfoErr
	}

	lastWorksForUser := make([]*entity.EriusTask, 0)

	task, getVersionErr := gb.RunContext.Services.Storage.GetVersionByWorkNumber(ctx, gb.RunContext.WorkNumber)
	if getVersionErr != nil {
		return getVersionErr
	}

	processSettings, getVersionErr := gb.RunContext.Services.Storage.GetVersionSettings(ctx, task.VersionID.String())
	if getVersionErr != nil {
		return getVersionErr
	}

	login := task.Author

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

	for i := range emails {
		tpl := mail.NewAddApproversTpl(
			gb.RunContext.WorkNumber,
			gb.RunContext.NotifName,
			gb.RunContext.Services.Sender.SdAddress,
			gb.RunContext.Services.SLAService.ComputeMaxDateFormatted(
				time.Now(), gb.State.SLA, slaInfoPtr),
			lastWorksForUser,
		)

		filesList := []string{tpl.Image}

		if len(lastWorksForUser) != 0 {
			filesList = append(filesList, warningImg)
		}

		iconFiles, iconErr := gb.RunContext.GetIcons(filesList)
		if iconErr != nil {
			return iconErr
		}

		files = append(files, iconFiles...)

		err = gb.RunContext.Services.Sender.SendNotification(ctx, []string{emails[i]}, files, tpl)
		if err != nil {
			return err
		}
	}

	return nil
}

// notifyDecisionMadeByAdditionalApprover notifies requesting approvers
// and the task initiator that an additional approver has left a review
func (gb *GoApproverBlock) notifyDecisionMadeByAdditionalApprover(ctx c.Context, logins []string) error {
	delegates, err := gb.RunContext.Services.HumanTasks.GetDelegationsByLogins(ctx, logins)
	if err != nil {
		return err
	}
	delegates = delegates.FilterByType("approvement")

	loginsWithDelegates := delegates.GetUserInArrayWithDelegations(logins)

	emailsToNotify := make([]string, 0, len(loginsWithDelegates))
	for _, login := range loginsWithDelegates {
		emailToNotify, emailErr := gb.RunContext.Services.People.GetUserEmail(ctx, login)
		if emailErr != nil {
			return emailErr
		}

		emailsToNotify = append(emailsToNotify, emailToNotify)
	}

	user, err := gb.RunContext.Services.People.GetUser(ctx, gb.RunContext.UpdateData.ByLogin)
	if err != nil {
		return err
	}

	userInfo, err := user.ToUserinfo()
	if err != nil {
		return err
	}

	latestDecisonLog := gb.State.ApproverLog[len(gb.State.ApproverLog)-1]

	tpl := mail.NewDecisionMadeByAdditionalApprover(
		gb.RunContext.WorkNumber,
		gb.RunContext.NotifName,
		latestDecisonLog.Decision.ToRuString(),
		latestDecisonLog.Comment,
		gb.RunContext.Services.Sender.SdAddress,
		userInfo,
	)

	files, err := gb.RunContext.Services.FileRegistry.GetAttachments(
		ctx,
		latestDecisonLog.Attachments,
	)

	if err != nil {
		return err
	}

	filesList := []string{tpl.Image, userImg}
	iconFiles, iconEerr := gb.RunContext.GetIcons(filesList)
	if iconEerr != nil {
		return iconEerr
	}
	files = append(files, iconFiles...)

	err = gb.RunContext.Services.Sender.SendNotification(ctx, emailsToNotify, files, tpl)
	if err != nil {
		return err
	}

	return nil
}

func (gb *GoApproverBlock) notifyNeedRework(ctx c.Context) error {
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

	err = gb.RunContext.Services.Sender.SendNotification(ctx, emails, files, tpl)
	if err != nil {
		return err
	}

	return nil
}

func (gb *GoApproverBlock) notifyNewInfoReceived(ctx c.Context, approverLogin string) error {
	l := logger.GetLogger(ctx)

	logins := []string{approverLogin}
	for i := range gb.State.AdditionalApprovers {
		logins = append(logins, gb.State.AdditionalApprovers[i].ApproverLogin)
	}

	delegates, err := gb.RunContext.Services.HumanTasks.GetDelegationsByLogins(ctx, logins)
	if err != nil {
		return err
	}

	delegates = delegates.FilterByType("approvement")
	loginsToNotify := delegates.GetUserInArrayWithDelegations(logins)

	var em string
	emails := make([]string, 0, len(loginsToNotify))
	for _, login := range loginsToNotify {
		em, err = gb.RunContext.Services.People.GetUserEmail(ctx, login)
		if err != nil {
			l.WithField("login", login).WithError(err).Warning("couldn't get email")
			return err
		}

		emails = append(emails, em)
	}

	tpl := mail.NewAnswerApproverInfoTpl(gb.RunContext.WorkNumber, gb.RunContext.NotifName,
		gb.RunContext.Services.Sender.SdAddress)

	files := []string{tpl.Image}
	iconFiles, err := gb.RunContext.GetIcons(files)
	if err != nil {
		return err
	}

	if notifErr := gb.RunContext.Services.Sender.SendNotification(ctx, emails, iconFiles, tpl); notifErr != nil {
		return notifErr
	}

	return nil
}

func (gb *GoApproverBlock) notifyNeedMoreInfo(ctx c.Context) error {
	l := logger.GetLogger(ctx)

	loginsToNotify := []string{gb.RunContext.Initiator}
	for login := range gb.State.Approvers {
		if login != gb.RunContext.UpdateData.ByLogin {
			loginsToNotify = append(loginsToNotify, login)
		}
	}

	emails := make([]string, 0, len(loginsToNotify))
	for _, login := range loginsToNotify {
		em, err := gb.RunContext.Services.People.GetUserEmail(ctx, login)
		if err != nil {
			l.WithField("login", login).WithError(err).Warning("couldn't get email")
			return err
		}

		emails = append(emails, em)
	}

	tpl := mail.NewRequestApproverInfoTpl(gb.RunContext.WorkNumber, gb.RunContext.NotifName,
		gb.RunContext.Services.Sender.SdAddress)

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
