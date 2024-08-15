package pipeline

import (
	"context"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
)

//nolint:dupl // maybe later
func (gb *GoEndBlock) handleNotifications(ctx context.Context) error {
	if gb.RunContext.skipNotifications {
		return nil
	}

	log := logger.GetLogger(ctx)

	login := gb.RunContext.Initiator

	templates := make(map[string]mail.Template, len(login))

	var btns []mail.Button

	buttonImg := make([]string, 0, 11)

	buttons := []string{
		"qualityControl-0.png", "qualityControl-1.png", "qualityControl-2.png",
		"qualityControl-3.png", "qualityControl-4.png", "qualityControl-5.png",
		"qualityControl-6.png", "qualityControl-7.png", "qualityControl-8.png",
		"qualityControl-9.png", "qualityControl-10.png",
	}

	initiatorEmail, getUserEmailErr := gb.RunContext.Services.People.GetUserEmail(ctx, login)
	if getUserEmailErr != nil {
		log.WithField("login", login).WithError(getUserEmailErr).Warning("couldn't get email")

		return getUserEmailErr
	}

	tpl := &mail.ProcessFinishedTemplate{
		WorkNumber: gb.RunContext.WorkNumber,
		Name:       gb.RunContext.NotifName,
		SdURL:      gb.RunContext.Services.Sender.SdAddress,
		Mailto:     gb.RunContext.Services.Sender.FetchEmail,
		Login:      login,
		Action:     "rate",
	}

	templates[initiatorEmail], btns = mail.NewNotifyProcessFinished(tpl, buttons)

	for _, v := range btns {
		buttonImg = append(buttonImg, v.Img)
	}

	err := gb.sendNotifications(ctx, templates, buttonImg)
	if err != nil {
		return err
	}

	return nil
}

func (gb *GoEndBlock) sendNotifications(
	ctx context.Context,
	templates map[string]mail.Template,
	buttonImg []string,
) error {
	for login, mailTemplate := range templates {
		iconsName := []string{mailTemplate.Image}

		iconsName = append(iconsName, buttonImg...)

		iconsFiles, iconsErr := gb.RunContext.GetIcons(iconsName)
		if iconsErr != nil {
			return iconsErr
		}

		sendErr := gb.RunContext.Services.Sender.SendNotification(
			ctx, []string{login}, iconsFiles, mailTemplate)

		if sendErr != nil {
			return sendErr
		}
	}

	return nil
}
