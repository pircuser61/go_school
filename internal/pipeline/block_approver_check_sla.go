package pipeline

import (
	"context"
	"fmt"
	"time"

	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sla"
)

func (gb *GoApproverBlock) checkBreachedSLA(ctx context.Context) error {
	const fn = "pipeline.approver.checkSLA"

	log := logger.GetLogger(ctx)

	seenAdditionalApprovers := map[string]bool{}
	emails := make([]string, 0, len(gb.State.Approvers)+len(gb.State.AdditionalApprovers))
	logins := getSliceFromMapOfStrings(gb.State.Approvers)

	for _, additionalApprover := range gb.State.AdditionalApprovers {
		// check if approver has not decisioned, and we did not see approver before
		if additionalApprover.Decision != nil || seenAdditionalApprovers[additionalApprover.ApproverLogin] {
			continue
		}

		seenAdditionalApprovers[additionalApprover.ApproverLogin] = true

		logins = append(logins, additionalApprover.ApproverLogin)
	}

	delegations, err := gb.RunContext.Services.HumanTasks.GetDelegationsByLogins(ctx, logins)
	if err != nil {
		log.WithError(err).Info(fn, fmt.Sprintf("approvers %v have no delegates", logins))
	}

	delegations = delegations.FilterByType("approvement")
	logins = delegations.GetUserInArrayWithDelegations(logins)

	var approverEmail string

	for i := range logins {
		approverEmail, err = gb.RunContext.Services.People.GetUserEmail(ctx, logins[i])
		if err != nil {
			log.WithError(err).Warning(fn, fmt.Sprintf("approver login %s not found", logins[i]))

			continue
		}

		emails = append(emails, approverEmail)
	}

	if len(emails) == 0 {
		return nil
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

	if processSettings.ResubmissionPeriod > 0 {
		var getWorksErr error

		_, getWorksErr = gb.RunContext.Services.Storage.GetWorksForUserWithGivenTimeRange(
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

	slaInfoPtr, getSLAInfoErr := gb.RunContext.Services.SLAService.GetSLAInfoPtr(ctx, sla.InfoDTO{
		TaskCompletionIntervals: []entity.TaskCompletionInterval{{
			StartedAt:  gb.RunContext.CurrBlockStartTime,
			FinishedAt: gb.RunContext.CurrBlockStartTime.Add(time.Hour * 24 * 100),
		}},
		WorkType: sla.WorkHourType(gb.State.WorkType),
	})
	if getSLAInfoErr != nil {
		return getSLAInfoErr
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

	tpl := mail.NewApprovementHalfSLATpl(
		gb.RunContext.WorkNumber,
		gb.RunContext.NotifName,
		gb.RunContext.Services.Sender.SdAddress,
		gb.State.ApproveStatusName,
		gb.RunContext.Services.SLAService.ComputeMaxDateFormatted(
			gb.RunContext.CurrBlockStartTime,
			gb.State.SLA,
			slaInfoPtr,
		),
		lastWorksForUser,
	)

	files := []string{tpl.Image}

	if len(lastWorksForUser) != 0 {
		files = append(files, warningImg)
	}

	iconFiles, iconErr := gb.RunContext.GetIcons(files)
	if iconErr != nil {
		return err
	}

	errSend := gb.RunContext.Services.Sender.SendNotification(ctx, emails, iconFiles, tpl)
	if errSend != nil {
		return errSend
	}

	return nil
}
