package pipeline

import (
	"context"

	"github.com/iancoleman/orderedmap"
	"gitlab.services.mts.ru/abp/mail/pkg/email"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	file_registry "gitlab.services.mts.ru/jocasta/pipeliner/internal/fileregistry"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
)

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
