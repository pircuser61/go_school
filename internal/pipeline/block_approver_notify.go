package pipeline

import (
	c "context"
	"time"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	e "gitlab.services.mts.ru/abp/mail/pkg/email"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sla"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

const (
	headImg = "header.png"
	userImg = "iconUser.svg"
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

	description, err := gb.RunContext.makeNotificationDescription()
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

	emails := make(map[string]mail.Template, 0)
	slaInfoPtr, getSlaInfoErr := gb.RunContext.Services.SLAService.GetSLAInfoPtr(ctx, sla.InfoDto{
		TaskCompletionIntervals: []entity.TaskCompletionInterval{{StartedAt: gb.RunContext.CurrBlockStartTime,
			FinishedAt: gb.RunContext.CurrBlockStartTime.Add(time.Hour * 24 * 100)}},
		WorkType: sla.WorkHourType(gb.State.WorkType),
	})

	if getSlaInfoErr != nil {
		return getSlaInfoErr
	}
	for _, login = range loginsToNotify {
		email, getEmailErr := gb.RunContext.Services.People.GetUserEmail(ctx, login)
		if getEmailErr != nil {
			l.WithField("login", login).WithError(getEmailErr).Warning("couldn't get email")
			continue
		}

		author, err := gb.RunContext.Services.People.GetUser(ctx, gb.RunContext.Initiator)
		if err != nil {
			return err
		}

		initiatorInfo, err := author.ToUserinfo()
		if err != nil {
			return err
		}

		emails[email] = mail.NewAppPersonStatusNotificationTpl(
			&mail.NewAppPersonStatusTpl{
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
			})
	}

	for i := range emails {

		file, ok := gb.RunContext.Services.Sender.Images[emails[i].Image]
		if !ok {
			return errors.New("file not found: " + emails[i].Image)
		}

		iconUser, iOk := gb.RunContext.Services.Sender.Images[userImg]
		if !iOk {
			return errors.New("file not found: " + emails[i].Image)
		}

		files := []e.Attachment{
			{
				Name:    headImg,
				Content: file,
				Type:    e.EmbeddedAttachment,
			},
			{
				Name:    userImg,
				Content: iconUser,
				Type:    e.EmbeddedAttachment,
			},
		}

		if sendErr := gb.RunContext.Services.Sender.SendNotification(
			ctx, []string{i}, files, emails[i],
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

	for i := range emails {
		tpl := mail.NewAddApproversTpl(
			gb.RunContext.WorkNumber,
			gb.RunContext.NotifName,
			gb.RunContext.Services.Sender.SdAddress,
			gb.RunContext.Services.SLAService.ComputeMaxDateFormatted(
				time.Now(), gb.State.SLA, slaInfoPtr),
		)

		file, ok := gb.RunContext.Services.Sender.Images[tpl.Image]
		if !ok {
			return errors.New("file not found: " + tpl.Image)
		}

		files = append(files, e.Attachment{
			Name:    headImg,
			Content: file,
			Type:    e.EmbeddedAttachment,
		})

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

	file, ok := gb.RunContext.Services.Sender.Images[tpl.Image]
	if !ok {
		return errors.New("file not found: " + tpl.Image)
	}

	files = append(files, e.Attachment{
		Name:    headImg,
		Content: file,
		Type:    e.EmbeddedAttachment,
	})

	iconUser, okU := gb.RunContext.Services.Sender.Images[userImg]
	if !okU {
		return errors.New("file not found: " + tpl.Image)
	}

	files = append(files, e.Attachment{
		Name:    headImg,
		Content: iconUser,
		Type:    e.EmbeddedAttachment,
	})

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

	file, ok := gb.RunContext.Services.Sender.Images[tpl.Image]
	if !ok {
		return errors.New("file not found: " + tpl.Image)
	}

	files := []e.Attachment{
		{
			Name:    headImg,
			Content: file,
			Type:    e.EmbeddedAttachment,
		},
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
	if err = gb.RunContext.Services.Sender.SendNotification(ctx, emails, nil, tpl); err != nil {
		return err
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

	file, ok := gb.RunContext.Services.Sender.Images[tpl.Image]
	if !ok {
		return errors.New("file not found: " + tpl.Image)
	}

	files := []e.Attachment{
		{
			Name:    headImg,
			Content: file,
			Type:    e.EmbeddedAttachment,
		},
	}
	if err := gb.RunContext.Services.Sender.SendNotification(ctx, emails, files, tpl); err != nil {
		return err
	}

	return nil
}
