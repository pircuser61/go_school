package pipeline

import (
	"context"
	"fmt"

	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
)

func (gb *GoExecutionBlock) checkBreachedSLA(ctx context.Context) error {
	const fn = "pipeline.execution.checkBreachedSLA"

	log := logger.GetLogger(ctx)

	emails := make([]string, 0, len(gb.State.Executors))
	logins := getSliceFromMapOfStrings(gb.State.Executors)

	delegations, err := gb.RunContext.Services.HumanTasks.GetDelegationsByLogins(ctx, logins)
	if err != nil {
		log.WithError(err).Info(fn, fmt.Sprintf("executors %v have no delegates", logins))
	}

	delegations = delegations.FilterByType("execution")
	logins = delegations.GetUserInArrayWithDelegations(logins)

	var executorEmail string

	for i := range logins {
		executorEmail, err = gb.RunContext.Services.People.GetUserEmail(ctx, logins[i])
		if err != nil {
			log.WithError(err).Warning(fn, fmt.Sprintf("executor login %s not found", logins[i]))

			continue
		}

		emails = append(emails, executorEmail)
	}

	tpl := mail.NewExecutionSLATpl(
		gb.RunContext.WorkNumber,
		gb.RunContext.NotifName,
		gb.RunContext.Services.Sender.SdAddress,
	)

	filesList := []string{tpl.Image}

	icons, iconEerr := gb.RunContext.GetIcons(filesList)
	if iconEerr != nil {
		return iconEerr
	}

	if len(emails) == 0 {
		return nil
	}

	err = gb.RunContext.Services.Sender.SendNotification(ctx, emails, icons, tpl)
	if err != nil {
		return err
	}

	return nil
}
