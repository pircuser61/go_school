package pipeline

import (
	c "context"

	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

func (gb *GoSignBlock) notifyAdditionalApprovers(ctx c.Context, logins []string, attachsId []entity.Attachment) error {
	l := logger.GetLogger(ctx)

	emails := make([]string, 0, len(logins))
	for _, login := range logins {
		approverEmail, emailErr := gb.RunContext.Services.People.GetUserEmail(ctx, login)
		if emailErr != nil {
			l.WithField("login", login).WithError(emailErr).Warning("couldn't get email")
			continue
		}

		emails = append(emails, approverEmail)
	}

	files, err := gb.RunContext.Services.FileRegistry.GetAttachments(ctx, attachsId)
	if err != nil {
		return err
	}

	emails = utils.UniqueStrings(emails)

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

	for i := range emails {
		tpl := mail.NewAddApproversTpl(
			gb.RunContext.WorkNumber,
			gb.RunContext.NotifName,
			gb.RunContext.Services.Sender.SdAddress,
			"",
			lastWorksForUser,
		)

		err = gb.RunContext.Services.Sender.SendNotification(ctx, []string{emails[i]}, files, tpl)
		if err != nil {
			return err
		}
	}

	return nil
}

// notifyDecisionMadeByAdditionalApprover notifies requesting signers
// and the task initiator that an additional approver has left a review
func (gb *GoSignBlock) notifyDecisionMadeByAdditionalApprover(ctx c.Context, logins []string) error {
	l := logger.GetLogger(ctx)

	emailsToNotify := make([]string, 0, len(logins))
	for _, login := range logins {
		emailToNotify, emailErr := gb.RunContext.Services.People.GetUserEmail(ctx, login)
		if emailErr != nil {
			l.WithField("login", login).WithError(emailErr).Warning("couldn't get email")
			continue
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

	latestDecisionLog := gb.State.SignLog[len(gb.State.SignLog)-1]

	tpl := mail.NewDecisionMadeByAdditionalApprover(
		gb.RunContext.WorkNumber,
		gb.RunContext.NotifName,
		userInfo.FullName,
		latestDecisionLog.Decision.ToRuString(),
		latestDecisionLog.Comment,
		gb.RunContext.Services.Sender.SdAddress,
	)

	files, err := gb.RunContext.Services.FileRegistry.GetAttachments(
		ctx,
		latestDecisionLog.Attachments,
	)

	if err != nil {
		return err
	}

	err = gb.RunContext.Services.Sender.SendNotification(ctx, emailsToNotify, files, tpl)
	if err != nil {
		return err
	}

	return nil
}
