package pipeline

import (
	"github.com/iancoleman/orderedmap"
	"gitlab.services.mts.ru/abp/mail/pkg/email"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
)

func (gb *GoExecutionBlock) attachFiles(
	tpl *mail.Template,
	buttons []mail.Button,
	lastWorksForUser []*entity.EriusTask,
	description []orderedmap.OrderedMap,
) ([]email.Attachment, error) {
	iconsName := []string{tpl.Image, userImg}

	for _, v := range buttons {
		iconsName = append(iconsName, v.Img)
	}

	if len(lastWorksForUser) != 0 {
		iconsName = append(iconsName, warningImg)
	}

	if gb.downloadImgFromDescription(description) {
		iconsName = append(iconsName, downloadImg)
	}

	attachFiles, err := gb.RunContext.GetIcons(iconsName)
	if err != nil {
		return nil, err
	}

	return attachFiles, nil
}
