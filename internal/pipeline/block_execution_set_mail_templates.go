package pipeline

import (
	"context"
	"time"

	"github.com/iancoleman/orderedmap"
	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sla"
)

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
		email, getUserEmailErr := gb.RunContext.Services.People.GetUserEmail(ctx, login)
		if getUserEmailErr != nil {
			log.WithField("login", login).WithError(getUserEmailErr).Warning("couldn't get email")

			continue
		}

		if !gb.State.IsTakenInWork {
			mailTemplates[email] = mail.NewExecutionNeedTakeInWorkTpl(
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
			author, errAuthor := gb.RunContext.Services.People.GetUser(ctx, gb.RunContext.Initiator)
			if errAuthor != nil {
				return nil, errAuthor
			}

			initiatorInfo, errInitiator := author.ToUserinfo()
			if errInitiator != nil {
				return nil, errInitiator
			}

			mailTemplates[email], _ = mail.NewAppPersonStatusNotificationTpl(
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
