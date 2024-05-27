package pipeline

import (
	"context"
	"time"

	"github.com/iancoleman/orderedmap"

	"gitlab.services.mts.ru/abp/mail/pkg/email"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	file_registry "gitlab.services.mts.ru/jocasta/pipeliner/internal/fileregistry"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sla"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

const (
	headImg    = "header.png"
	userImg    = "iconUser.png"
	warningImg = "warning.png"
	vRabotuBtn = "v_rabotu.png"

	approveBtn = "soglas.png"
	rejectBtn  = "otklon.png"
)

//nolint:dupl // maybe later
func (gb *GoApproverBlock) handleNotifications(ctx context.Context) error {
	if gb.RunContext.skipNotifications {
		return nil
	}

	l := logger.GetLogger(ctx)

	delegates, getDelegationsErr := gb.RunContext.Services.HumanTasks.GetDelegationsByLogins(
		ctx,
		getSliceFromMap(gb.State.Approvers),
	)
	if getDelegationsErr != nil {
		return getDelegationsErr
	}

	delegates = delegates.FilterByType("approvement")

	approvers := getSliceFromMap(gb.State.Approvers)
	loginsToNotify := delegates.GetUserInArrayWithDelegations(approvers)

	description, files, err := gb.RunContext.makeNotificationDescription(gb.Name)
	if err != nil {
		return err
	}

	actionsList := gb.makeActionList()

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

	templates := make(map[string]mail.Template, len(loginsToNotify))

	var buttons []mail.Button

	buttonImg := make([]string, 0, 7)

	usersNotToNotify := gb.getUsersNotToNotifySet()

	for _, login = range loginsToNotify {
		if _, ok := usersNotToNotify[login]; ok {
			continue
		}

		userEmail, getEmailErr := gb.RunContext.Services.People.GetUserEmail(ctx, login)
		if getEmailErr != nil {
			l.WithField("login", login).WithError(getEmailErr).Warning("couldn't get email")

			continue
		}

		author, autorErr := gb.RunContext.Services.People.GetUser(ctx, gb.RunContext.Initiator, false)
		if autorErr != nil {
			return autorErr
		}

		initiatorInfo, initialErr := author.ToUserinfo()
		if initialErr != nil {
			return initialErr
		}

		tpl := &mail.NewAppPersonStatusTpl{
			WorkNumber:                gb.RunContext.WorkNumber,
			Name:                      gb.RunContext.NotifName,
			Status:                    gb.State.ApproveStatusName,
			Action:                    statusToTaskAction[StatusApprovement],
			DeadLine:                  gb.RunContext.Services.SLAService.ComputeMaxDateFormatted(time.Now(), gb.State.SLA, slaInfoPtr),
			SdURL:                     gb.RunContext.Services.Sender.SdAddress,
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

		templates[userEmail], buttons = mail.NewAppPersonStatusNotificationTpl(tpl)
	}

	for _, v := range buttons {
		buttonImg = append(buttonImg, v.Img)
	}

	err = gb.sendNotifications(ctx, templates, buttonImg, lastWorksForUser, description, files)
	if err != nil {
		return err
	}

	return nil
}

func (gb *GoApproverBlock) makeActionList() []mail.Action {
	actionsList := make([]mail.Action, 0, len(gb.State.ActionList))

	for i := range gb.State.ActionList {
		actionsList = append(actionsList, mail.Action{
			InternalActionName: gb.State.ActionList[i].ID,
			Title:              gb.State.ActionList[i].Title,
		})
	}

	return actionsList
}

func (gb *GoApproverBlock) sendNotifications(
	ctx context.Context,
	templates map[string]mail.Template,
	buttonImg []string,
	lastWorksForUser []*entity.EriusTask,
	description []orderedmap.OrderedMap,
	files []email.Attachment,
) error {
	for login, mailTemplate := range templates {
		iconsName := []string{mailTemplate.Image, userImg}
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

		iconsFiles = append(iconsFiles, files...)

		sendErr := gb.RunContext.Services.Sender.SendNotification(ctx, []string{login}, iconsFiles, mailTemplate)
		if sendErr != nil {
			return sendErr
		}
	}

	return nil
}

func (gb *GoApproverBlock) notifyAdditionalApprovers(ctx context.Context, logins []string) error {
	log := logger.GetLogger(ctx)

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
			log.WithField("login", login).WithError(emailErr).Warning("couldn't get email")

			continue
		}

		emails = append(emails, approverEmail)
	}

	emails = utils.UniqueStrings(emails)

	slaInfoPtr, getSLAInfoErr := gb.RunContext.Services.SLAService.GetSLAInfoPtr(ctx, sla.InfoDTO{
		TaskCompletionIntervals: []entity.TaskCompletionInterval{
			{
				StartedAt:  gb.RunContext.CurrBlockStartTime,
				FinishedAt: gb.RunContext.CurrBlockStartTime.Add(time.Hour * 24 * 100),
			},
		},
		WorkType: sla.WorkHourType(gb.State.WorkType),
	})

	if getSLAInfoErr != nil {
		return getSLAInfoErr
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

	description, files, err := gb.RunContext.makeNotificationDescription(gb.Name)
	if err != nil {
		return err
	}

	author, authorErr := gb.RunContext.Services.People.GetUser(ctx, gb.RunContext.Initiator, true)
	if authorErr != nil {
		return authorErr
	}

	initiatorInfo, initialErr := author.ToUserinfo()
	if initialErr != nil {
		return initialErr
	}

	actionsList := make([]mail.Action, 0, len(gb.State.ActionList))
	for i := range gb.State.ActionList {
		actionsList = append(actionsList, mail.Action{
			InternalActionName: gb.State.ActionList[i].ID,
			Title:              gb.State.ActionList[i].Title,
		})
	}

	for i := range emails {
		tpl, _ := mail.NewAddApproversTpl(
			&mail.NewAppPersonStatusTpl{
				WorkNumber: gb.RunContext.WorkNumber,
				Name:       gb.RunContext.NotifName,
				SdURL:      gb.RunContext.Services.Sender.SdAddress,
				Action:     script.SettingStatusApprovement,
				DeadLine: gb.RunContext.Services.SLAService.ComputeMaxDateFormatted(
					time.Now(), gb.State.SLA, slaInfoPtr),
				LastWorks:       lastWorksForUser,
				Description:     description,
				Mailto:          gb.RunContext.Services.Sender.FetchEmail,
				Login:           login,
				IsEditable:      gb.State.GetIsEditable(),
				ApproverActions: actionsList,
				BlockID:         BlockGoApproverID,
				Initiator:       initiatorInfo,
			}, emails[i],
		)

		filesList := []string{tpl.Image, userImg, approveBtn, rejectBtn}

		if len(lastWorksForUser) != 0 {
			filesList = append(filesList, warningImg)
		}

		if isNeedAddDownloadImage(description) {
			filesList = append(filesList, downloadImg)
		}

		iconFiles, iconErr := gb.RunContext.GetIcons(filesList)
		if iconErr != nil {
			return iconErr
		}

		iconFiles = append(iconFiles, files...)

		err = gb.RunContext.Services.Sender.SendNotification(ctx, []string{emails[i]}, iconFiles, tpl)
		if err != nil {
			return err
		}
	}

	return nil
}

func isNeedAddDownloadImage(description []orderedmap.OrderedMap) bool {
	for _, v := range description {
		links, ok := v.Get("attachLinks")
		if ok {
			attachFiles, ok := links.([]file_registry.AttachInfo)
			if ok && len(attachFiles) != 0 {
				return true
			}
		}
	}

	return false
}

// notifyDecisionMadeByAdditionalApprover notifies requesting approvers
// and the task initiator that an additional approver has left a review
func (gb *GoApproverBlock) notifyDecisionMadeByAdditionalApprover(ctx context.Context, logins []string) error {
	l := logger.GetLogger(ctx)

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
			l.WithField("login", login).WithError(emailErr).Warning("couldn't get email")

			continue
		}

		emailsToNotify = append(emailsToNotify, emailToNotify)
	}

	user, err := gb.RunContext.Services.People.GetUser(ctx, gb.RunContext.UpdateData.ByLogin, false)
	if err != nil {
		return err
	}

	userInfo, err := user.ToUserinfo()
	if err != nil {
		return err
	}

	latestDecisonLog := gb.State.ApproverLog[len(gb.State.ApproverLog)-1]

	files := make([]email.Attachment, 0)

	filesAttach, _, err := gb.RunContext.makeNotificationAttachment()
	if err != nil {
		return err
	}

	attach, err := gb.RunContext.GetAttach(filesAttach)
	if err != nil {
		return err
	}

	files = append(files, attach.AttachmentsList...)

	cleanName(files)

	tpl := mail.NewDecisionMadeByAdditionalApprover(
		&mail.ReviewTemplate{
			ID:          gb.RunContext.WorkNumber,
			Name:        gb.RunContext.NotifName,
			Decision:    latestDecisonLog.Decision.ToRuString(),
			Comment:     latestDecisonLog.Comment,
			SdURL:       gb.RunContext.Services.Sender.SdAddress,
			Author:      userInfo,
			AttachLinks: attach.AttachLinks,
			AttachExist: attach.AttachExists,
		},
	)

	filesList := []string{tpl.Image, userImg}

	if len(attach.AttachLinks) != 0 {
		filesList = append(filesList, downloadImg)
	}

	iconFiles, iconErr := gb.RunContext.GetIcons(filesList)
	if iconErr != nil {
		return iconErr
	}

	files = append(files, iconFiles...)

	err = gb.RunContext.Services.Sender.SendNotification(ctx, emailsToNotify, files, tpl)
	if err != nil {
		return err
	}

	return nil
}

func (gb *GoApproverBlock) notifyNeedRework(ctx context.Context) error {
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

	tpl := mail.NewSendToInitiatorEditTpl(
		gb.RunContext.WorkNumber,
		gb.RunContext.NotifName,
		gb.RunContext.Services.Sender.SdAddress,
	)

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

func (gb *GoApproverBlock) notifyNewInfoReceived(ctx context.Context, approverLogin string) error {
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

			continue
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

	return gb.RunContext.Services.Sender.SendNotification(ctx, emails, iconFiles, tpl)
}

func (gb *GoApproverBlock) notifyNeedMoreInfo(ctx context.Context) error {
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

			continue
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

func (gb *GoApproverBlock) getUsersNotToNotifySet() map[string]struct{} {
	usersNotToNotify := make(map[string]struct{})

	for i := range gb.State.ApproverLog {
		if gb.State.ApproverLog[i].LogType == ApproverLogDecision {
			usersNotToNotify[gb.State.ApproverLog[i].Login] = struct{}{}
			usersNotToNotify[gb.State.ApproverLog[i].DelegateFor] = struct{}{}
		}
	}

	return usersNotToNotify
}
