package pipeline

import (
	"context"

	"github.com/iancoleman/orderedmap"
	"gitlab.services.mts.ru/abp/mail/pkg/email"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	file_registry "gitlab.services.mts.ru/jocasta/pipeliner/internal/fileregistry"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
)

func (gb *GoApproverBlock) sendNotifications(
	ctx context.Context,
	templates map[string]mail.Template,
	buttonImg []string,
	lastWorksForUser []*entity.EriusTask,
	description []orderedmap.OrderedMap,
	files []email.Attachment,
) error {
	for login, mailTemplate := range templates {
		iconsName := []string{mailTemplate.Image, userImg}
		iconsName = append(iconsName, buttonImg...)

		if len(lastWorksForUser) != 0 {
			iconsName = append(iconsName, warningImg)
		}

		for _, v := range description {
			links, link := v.Get("attachLinks")
			if link {
				attachFiles, ok := links.([]file_registry.AttachInfo)
				if ok && len(attachFiles) != 0 {
					iconsName = append(iconsName, downloadImg)

					break
				}
			}
		}

		iconsFiles, iconsErr := gb.RunContext.GetIcons(iconsName)
		if iconsErr != nil {
			return iconsErr
		}

		iconsFiles = append(iconsFiles, files...)

		sendErr := gb.RunContext.Services.Sender.SendNotification(ctx, []string{login}, iconsFiles, mailTemplate)
		if sendErr != nil {
			return sendErr
		}
	}

	return nil
}
