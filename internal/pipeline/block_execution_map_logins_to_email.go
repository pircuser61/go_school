package pipeline

import (
	"context"

	"gitlab.services.mts.ru/abp/myosotis/logger"
)

func (gb *GoExecutionBlock) mapLoginsToEmails(ctx context.Context, loginsToNotify []string, loginTakenInWork string) []string {
	log := logger.GetLogger(ctx)
	emails := make([]string, 0)

	for _, login := range loginsToNotify {
		if login != loginTakenInWork {
			email, emailErr := gb.RunContext.Services.People.GetUserEmail(ctx, login)
			if emailErr != nil {
				log.WithField("login", login).WithError(emailErr).Warning("couldn't get email")

				continue
			}

			emails = append(emails, email)
		}
	}

	return emails
}
