package pipeline

import (
	c "context"
	"errors"
	"time"

	e "gitlab.services.mts.ru/abp/mail/pkg/email"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sla"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

func (gb *GoSignBlock) notifyAdditionalApprovers(ctx c.Context, logins []string, attachsId []entity.Attachment) error {
	emails := make([]string, 0, len(logins))
	for _, login := range logins {
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
		WorkType: sla.WorkHourType(*gb.State.WorkType),
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
				time.Now(), *gb.State.SLA, slaInfoPtr),
			lastWorksForUser,
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

// notifyDecisionMadeByAdditionalApprover notifies requesting signers
// and the task initiator that an additional approver has left a review
func (gb *GoSignBlock) notifyDecisionMadeByAdditionalApprover(ctx c.Context, logins []string) error {
	emailsToNotify := make([]string, 0, len(logins))
	for _, login := range logins {
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

	latestDecisionLog := gb.State.SignLog[len(gb.State.SignLog)-1]
	tpl := mail.NewDecisionMadeByAdditionalApprover(
		gb.RunContext.WorkNumber,
		gb.RunContext.NotifName,
		latestDecisionLog.Decision.ToRuString(),
		latestDecisionLog.Comment,
		gb.RunContext.Services.Sender.SdAddress,
		userInfo,
	)

	files, err := gb.RunContext.Services.FileRegistry.GetAttachments(
		ctx,
		latestDecisionLog.Attachments,
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

	err = gb.RunContext.Services.Sender.SendNotification(ctx, emailsToNotify, files, tpl)
	if err != nil {
		return err
	}

	return nil
}
