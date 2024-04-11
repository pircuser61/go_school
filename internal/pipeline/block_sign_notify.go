package pipeline

import (
	c "context"
	"time"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sla"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

func (gb *GoSignBlock) notifyAdditionalApprovers(ctx c.Context, logins []string, _ []entity.Attachment) error {
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

	emails = utils.UniqueStrings(emails)

	slaDeadline := ""

	if gb.State.SLA != nil && gb.State.WorkType != nil {
		slaInfoPtr, getSLAInfoErr := gb.RunContext.Services.SLAService.GetSLAInfoPtr(ctx, sla.InfoDTO{
			TaskCompletionIntervals: []entity.TaskCompletionInterval{{
				StartedAt:  gb.RunContext.CurrBlockStartTime,
				FinishedAt: gb.RunContext.CurrBlockStartTime.Add(time.Hour * 24 * 100),
			}},
			WorkType: sla.WorkHourType(*gb.State.WorkType),
		})
		if getSLAInfoErr != nil {
			return getSLAInfoErr
		}

		slaDeadline = gb.RunContext.Services.SLAService.ComputeMaxDateFormatted(time.Now(), *gb.State.SLA, slaInfoPtr)
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

	author, authorErr := gb.RunContext.Services.People.GettingUser(ctx, gb.RunContext.Initiator)
	if authorErr != nil {
		return authorErr
	}

	initiatorInfo, initialErr := author.ToUserinfo()
	if initialErr != nil {
		return initialErr
	}

	for i := range emails {
		tpl, _ := mail.NewAddApproversTpl(
			&mail.NewAppPersonStatusTpl{
				WorkNumber:      gb.RunContext.WorkNumber,
				Name:            gb.RunContext.NotifName,
				SdURL:           gb.RunContext.Services.Sender.SdAddress,
				Action:          "",
				DeadLine:        slaDeadline,
				LastWorks:       lastWorksForUser,
				Description:     description,
				Mailto:          gb.RunContext.Services.Sender.FetchEmail,
				Login:           login,
				IsEditable:      false,
				ApproverActions: nil,
				BlockID:         BlockGoSignID,
				Initiator:       initiatorInfo,
			}, emails[i],
		)

		filesList := []string{tpl.Image, userImg, rejectBtn, approveBtn}

		if len(lastWorksForUser) != 0 {
			filesList = append(filesList, warningImg)
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

	user, err := gb.RunContext.Services.People.GettingUser(ctx, gb.RunContext.UpdateData.ByLogin)
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
		gb.RunContext.WorkNumber,
		gb.RunContext.ClientID,
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
