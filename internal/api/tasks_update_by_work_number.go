package api

import (
	"context"
	"fmt"

	"gitlab.services.mts.ru/abp/mail/pkg/email"
	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
)

func (ae *Env) updateTaskByWorkNumber(
	ctx context.Context,
	txStorage db.Database,
	ui *sso.UserInfo,
	workNumber string,
	tasks *stoppedTasks,
) error {
	log := logger.GetLogger(ctx)

	dbTask, getTaskErr := txStorage.GetTask(ctx, []string{ui.Username}, []string{ui.Username}, ui.Username, workNumber)
	if getTaskErr != nil {
		log.WithError(getTaskErr).Error("couldn't get task")

		return getTaskErr
	}

	if dbTask.FinishedAt != nil {
		tasks.Tasks = append(
			tasks.Tasks,
			stoppedTask{
				FinishedAt: *dbTask.FinishedAt,
				Status:     dbTask.HumanStatus,
				WorkNumber: dbTask.WorkNumber,
				ID:         dbTask.ID,
			},
		)

		return nil
	}

	err := txStorage.StopTaskBlocks(ctx, dbTask.ID)
	if err != nil {
		log.WithError(err).Error("couldn't stop task blocks")

		return err
	}

	err = txStorage.UpdateTaskStatus(ctx, dbTask.ID, db.RunStatusCanceled, db.CommentCanceled, ui.Username)
	if err != nil {
		log.WithError(err).Error("couldn't update task status")

		return err
	}

	updatedTask, updateTaskErr := txStorage.UpdateTaskHumanStatus(ctx, dbTask.ID, string(pipeline.StatusCancel), "")
	if updateTaskErr != nil {
		log.WithError(updateTaskErr).Error("couldn't update human status")

		return updateTaskErr
	}

	logins, loginsErr := ae.getAuthorAndMembersToNotify(ctx, workNumber, ui.Username)
	if loginsErr != nil {
		log.WithError(loginsErr).Error("couldn't get logins")
	}

	emails := make([]string, 0, len(logins))

	for _, login := range logins {
		userEmail, getUserEmailErr := ae.People.GetUserEmail(ctx, login)
		if getUserEmailErr != nil {
			continue
		}

		emails = append(emails, userEmail)
	}

	em := mail.NewRejectPipelineGroupTemplate(dbTask.WorkNumber, dbTask.Name, ae.Mail.SdAddress)

	file, ok := ae.Mail.Images[em.Image]
	if !ok {
		log.Error("couldn't find images: ", em.Image)

		return fmt.Errorf("couldn't find images: %s", em.Image)
	}

	files := []email.Attachment{
		{
			Name:    headImg,
			Content: file,
			Type:    email.EmbeddedAttachment,
		},
	}

	sendNotifErr := ae.Mail.SendNotification(ctx, emails, files, em)
	if sendNotifErr != nil {
		log.WithError(sendNotifErr).Error("couldn't send notification")
	}

	tasks.Tasks = append(
		tasks.Tasks,
		stoppedTask{
			FinishedAt: *updatedTask.FinishedAt,
			Status:     updatedTask.HumanStatus,
			WorkNumber: updatedTask.WorkNumber,
			ID:         dbTask.ID,
		},
	)

	return nil
}
