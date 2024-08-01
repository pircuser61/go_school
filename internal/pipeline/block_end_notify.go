package pipeline

import (
	"context"
	"gitlab.services.mts.ru/abp/mail/pkg/email"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
)

//nolint:dupl // maybe later
func (gb *GoEndBlock) handleNotifications(ctx context.Context) error {
	if gb.RunContext.skipNotifications {
		return nil
	}

	_, files, err := gb.RunContext.makeNotificationDescription(ctx, gb.Name, false)
	if err != nil {
		return err
	}

	emails := make(map[string]mail.Template, 0)

	task, getVersionErr := gb.RunContext.Services.Storage.GetVersionByWorkNumber(ctx, gb.RunContext.WorkNumber)
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

	filesNames := make([]string, 0)

	fnames, err := gb.setMailTemplates(ctx, login, emails)
	if err != nil {
		return err
	}

	filesNames = append(filesNames, fnames...)

	err = gb.sendNotifications(ctx, emails, filesNames, files)
	if err != nil {
		return err
	}

	return nil
}

func (gb *GoEndBlock) sendNotifications(
	ctx context.Context,
	emails map[string]mail.Template,
	filesNames []string,
	files []email.Attachment,
) error {
	for i, item := range emails {
		iconsName := []string{item.Image}
		iconsName = append(iconsName, filesNames...)

		iconFiles, errFiles := gb.RunContext.GetIcons(iconsName)
		if errFiles != nil {
			return errFiles
		}

		iconFiles = append(iconFiles, files...)

		sendErr := gb.RunContext.Services.Sender.SendNotification(
			ctx,
			[]string{i},
			iconFiles,
			emails[i],
		)
		if sendErr != nil {
			return sendErr
		}
	}

	return nil
}

func (gb *GoEndBlock) setMailTemplates(
	ctx context.Context,
	login string,
	mailTemplates map[string]mail.Template,
) ([]string, error) {
	log := logger.GetLogger(ctx)
	filesNames := make([]string, 0)

	userEmail, getUserEmailErr := gb.RunContext.Services.People.GetUserEmail(ctx, login)
	if getUserEmailErr != nil {
		log.WithField("login", login).WithError(getUserEmailErr).Warning("couldn't get email")

		return nil, getUserEmailErr
	}

	mailTemplates[userEmail] = mail.NewAppCompletedTemplate(
		&mail.AppCompletedTemplate{
			WorkNumber: gb.RunContext.WorkNumber,
			Name:       gb.RunContext.NotifName,
			SdURL:      gb.RunContext.Services.Sender.SdAddress,
			Mailto:     gb.RunContext.Services.Sender.FetchEmail,
			Login:      login,
		},
	)

	return filesNames, nil
}
