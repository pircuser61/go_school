package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/iancoleman/orderedmap"

	"gitlab.services.mts.ru/abp/mail/pkg/email"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	file_registry "gitlab.services.mts.ru/jocasta/pipeliner/internal/fileregistry"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sla"
)

const (
	downloadImg = "iconDownload.png"
)

//nolint:dupl // maybe later
func (gb *GoExecutionBlock) handleNotifications(ctx context.Context) error {
	if gb.RunContext.skipNotifications {
		return nil
	}

	executors := getSliceFromMap(gb.State.Executors)

	delegates, getDelegationsErr := gb.RunContext.Services.HumanTasks.GetDelegationsByLogins(ctx, executors)
	if getDelegationsErr != nil {
		return getDelegationsErr
	}

	delegates = delegates.FilterByType("execution")

	loginsToNotify := delegates.GetUserInArrayWithDelegations(executors)

	description, files, err := gb.RunContext.makeNotificationDescription(ctx, gb.Name, false)
	if err != nil {
		return err
	}

	emails := make(map[string]mail.Template, 0)

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

	filesNames := make([]string, 0)

	if getSLAInfoErr != nil {
		return getSLAInfoErr
	}

	if !gb.State.IsTakenInWork {
		filesNames = append(filesNames, vRabotuBtn)
	}

	fnames, err := gb.setMailTemplates(ctx, loginsToNotify, emails, description, lastWorksForUser, slaInfoPtr)
	if err != nil {
		return err
	}

	filesNames = append(filesNames, fnames...)

	err = gb.sendNotifications(ctx, emails, filesNames, lastWorksForUser, description, files)
	if err != nil {
		return err
	}

	return nil
}

func (gb *GoExecutionBlock) sendNotifications(
	ctx context.Context,
	emails map[string]mail.Template,
	filesNames []string,
	lastWorksForUser []*entity.EriusTask,
	description []orderedmap.OrderedMap,
	files []email.Attachment,
) error {
	for i, item := range emails {
		iconsName := []string{item.Image}
		iconsName = append(iconsName, filesNames...)

		if len(lastWorksForUser) != 0 {
			iconsName = append(iconsName, warningImg)
		}

		for _, v := range description {
			links, link := v.Get("attachLinks")
			if !link {
				continue
			}

			attachFiles, ok := links.([]file_registry.AttachInfo)

			if ok && len(attachFiles) != 0 {
				iconsName = append(iconsName, downloadImg)

				break
			}
		}

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

func (gb *GoExecutionBlock) notifyNeedRework(ctx context.Context) error {
	l := logger.GetLogger(ctx)

	delegates, err := gb.RunContext.Services.HumanTasks.GetDelegationsFromLogin(ctx, gb.RunContext.Initiator)
	if err != nil {
		return err
	}

	var updateParams ExecutionUpdateParams

	if err = json.Unmarshal(gb.RunContext.UpdateData.Parameters, &updateParams); err != nil {
		return errors.New("can't unmarshal update params")
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
		gb.RunContext.Services.Sender.SdAddress, updateParams.Comment)

	filesList := []string{tpl.Image}

	files, iconEerr := gb.RunContext.GetIcons(filesList)
	if iconEerr != nil {
		return iconEerr
	}

	if sendErr := gb.RunContext.Services.Sender.SendNotification(ctx, emails, files, tpl); sendErr != nil {
		return sendErr
	}

	return nil
}

func (gb *GoExecutionBlock) setMailTemplates(
	ctx context.Context,
	loginsToNotify []string,
	mailTemplates map[string]mail.Template,
	description []orderedmap.OrderedMap,
	lastWorksForUser []*entity.EriusTask,
	slaInfoPtr *sla.Info,
) ([]string, error) {
	log := logger.GetLogger(ctx)
	filesNames := make([]string, 0)

	for _, login := range loginsToNotify {
		userEmail, getUserEmailErr := gb.RunContext.Services.People.GetUserEmail(ctx, login)
		if getUserEmailErr != nil {
			log.WithField("login", login).WithError(getUserEmailErr).Warning("couldn't get email")

			continue
		}

		if !gb.State.IsTakenInWork {
			mailTemplates[userEmail] = mail.NewExecutionNeedTakeInWorkTpl(
				&mail.ExecutorNotifTemplate{
					WorkNumber:  gb.RunContext.WorkNumber,
					Name:        gb.RunContext.NotifName,
					SdURL:       gb.RunContext.Services.Sender.SdAddress,
					BlockID:     BlockGoExecutionID,
					Description: description,
					Mailto:      gb.RunContext.Services.Sender.FetchEmail,
					Login:       login,
					LastWorks:   lastWorksForUser,
					IsGroup:     len(gb.State.Executors) > 1,
					Deadline:    gb.RunContext.Services.SLAService.ComputeMaxDateFormatted(time.Now(), gb.State.SLA, slaInfoPtr),
				},
			)
		} else {
			author, errAuthor := gb.RunContext.Services.People.GetUser(ctx, gb.RunContext.Initiator, false)
			if errAuthor != nil {
				return nil, errAuthor
			}

			initiatorInfo, errInitiator := author.ToUserinfo()
			if errInitiator != nil {
				return nil, errInitiator
			}

			mailTemplates[userEmail], _ = mail.NewAppPersonStatusNotificationTpl(
				&mail.NewAppPersonStatusTpl{
					WorkNumber:  gb.RunContext.WorkNumber,
					Name:        gb.RunContext.NotifName,
					Status:      string(StatusExecution),
					Action:      statusToTaskAction[StatusExecution],
					DeadLine:    gb.RunContext.Services.SLAService.ComputeMaxDateFormatted(time.Now(), gb.State.SLA, slaInfoPtr),
					Description: description,
					SdURL:       gb.RunContext.Services.Sender.SdAddress,
					Mailto:      gb.RunContext.Services.Sender.FetchEmail,
					Login:       login,
					IsEditable:  gb.State.GetIsEditable(),
					Initiator:   initiatorInfo,

					BlockID:                   BlockGoExecutionID,
					ExecutionDecisionExecuted: string(ExecutionDecisionExecuted),
					ExecutionDecisionRejected: string(ExecutionDecisionRejected),
					LastWorks:                 lastWorksForUser,
				})

			if initiatorInfo != nil {
				filesNames = append(filesNames, userImg)
			}
		}
	}

	return filesNames, nil
}

// 22 (Soglasovanie analogichno)
func (gb *GoExecutionBlock) notifyNeedMoreInfo(ctx context.Context) error {
	l := logger.GetLogger(ctx)

	var updateParams ExecutionUpdateParams

	if err := json.Unmarshal(gb.RunContext.UpdateData.Parameters, &updateParams); err != nil {
		return errors.New("can't unmarshal update params")
	}

	loginsToNotify := []string{gb.RunContext.Initiator}

	emails := make([]string, 0, len(loginsToNotify))

	for _, login := range loginsToNotify {
		userEmail, err := gb.RunContext.Services.People.GetUserEmail(ctx, login)
		if err != nil {
			l.WithField("login", login).WithError(err).Warning("couldn't get email")

			return nil
		}

		emails = append(emails, userEmail)
	}

	tpl := mail.NewRequestExecutionInfoTpl(
		gb.RunContext.WorkNumber,
		gb.RunContext.NotifName,
		gb.RunContext.Services.Sender.SdAddress,
		updateParams.Comment,
	)

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

func (gb *GoExecutionBlock) notifyNewInfoReceived(ctx context.Context) error {
	l := logger.GetLogger(ctx)

	delegates, err := gb.RunContext.Services.HumanTasks.GetDelegationsByLogins(ctx,
		getSliceFromMap(gb.State.Executors))
	if err != nil {
		return err
	}

	loginsToNotify := delegates.GetUserInArrayWithDelegations(getSliceFromMap(gb.State.Executors))

	var userEmail string

	emails := make([]string, 0, len(loginsToNotify))

	for _, login := range loginsToNotify {
		userEmail, err = gb.RunContext.Services.People.GetUserEmail(ctx, login)
		if err != nil {
			l.WithField("login", login).WithError(err).Warning("couldn't get email")

			continue
		}

		emails = append(emails, userEmail)
	}

	tpl := mail.NewAnswerExecutionInfoTpl(
		gb.RunContext.WorkNumber,
		gb.RunContext.NotifName,
		gb.RunContext.Services.Sender.SdAddress,
	)

	files := []string{tpl.Image}

	iconFiles, iconEerr := gb.RunContext.GetIcons(files)
	if iconEerr != nil {
		return iconEerr
	}

	if sendErr := gb.RunContext.Services.Sender.SendNotification(ctx, emails, iconFiles, tpl); sendErr != nil {
		return sendErr
	}

	return nil
}
