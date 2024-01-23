package pipeline

import (
	c "context"
	"encoding/json"
	"strconv"

	om "github.com/iancoleman/orderedmap"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	e "gitlab.services.mts.ru/abp/mail/pkg/email"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	fileregistry "gitlab.services.mts.ru/jocasta/pipeliner/internal/file-registry"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
)

type handleInitiatorNotifyParams struct {
	step     string
	stepType string
	action   string
	status   TaskHumanStatus
}

func (runCtx *BlockRunContext) handleInitiatorNotify(ctx c.Context, params handleInitiatorNotifyParams) error {
	const (
		FormStepType     = "form"
		TimerStepType    = "timer"
		FunctionStepType = "executable_function"
	)

	if runCtx.skipNotifications {
		return nil
	}

	switch params.status {
	case StatusNew,
		StatusApproved,
		StatusApproveViewed,
		StatusApproveInformed,
		StatusApproveConfirmed,
		StatusApprovementRejected,
		StatusExecution,
		StatusExecutionRejected,
		StatusSigned,
		StatusRejected,
		StatusProcessingError,
		StatusDone:
	default:
		return nil
	}

	st := params.stepType

	if params.status == StatusDone &&
		(st == FormStepType || st == FunctionStepType || st == TimerStepType) {
		return nil
	}

	description, files, err := runCtx.makeNotificationDescription(params.step)
	if err != nil {
		return err
	}

	loginsToNotify := []string{runCtx.Initiator}

	log := logger.GetLogger(ctx)

	var email string
	emails := make([]string, 0, len(loginsToNotify))
	for _, login := range loginsToNotify {
		email, err = runCtx.Services.People.GetUserEmail(ctx, login)
		if err != nil {
			log.WithField("login", login).WithError(err).Warning("couldn't get email")
			return nil
		}

		emails = append(emails, email)
	}

	if params.action == "" {
		params.action = statusToTaskState[params.status]
	}

	tmpl := mail.NewAppInitiatorStatusNotificationTpl(
		&mail.SignerNotifTemplate{
			WorkNumber:  runCtx.WorkNumber,
			Name:        runCtx.NotifName,
			SdUrl:       runCtx.Services.Sender.SdAddress,
			Description: description,
			Action:      params.action,
		})

	iconsName := []string{tmpl.Image}

	for _, v := range description {
		links, link := v.Get("attachLinks")
		if link {
			attachFiles, ok := links.([]fileregistry.AttachInfo)
			if ok && len(attachFiles) != 0 {
				descIcons := []string{downloadImg}
				iconsName = append(iconsName, descIcons...)
				break
			}
		}
	}

	iconFiles, iconErr := runCtx.GetIcons(iconsName)
	if iconErr != nil {
		return err
	}

	files = append(files, iconFiles...)

	if sendErr := runCtx.Services.Sender.SendNotification(ctx, emails, files, tmpl); sendErr != nil {
		return sendErr
	}

	return nil
}

func (runCtx *BlockRunContext) getFileField() ([]string, error) {
	task, err := runCtx.Services.Storage.GetTaskRunContext(c.Background(), runCtx.WorkNumber)
	if err != nil {
		return nil, err
	}

	return task.InitialApplication.AttachmentFields, nil
}

func (runCtx *BlockRunContext) makeNotificationFormAttachment(files []string) ([]fileregistry.FileInfo, error) {
	attachments := make([]entity.Attachment, 0)
	mapFiles := make(map[string][]entity.Attachment)
	for _, v := range files {
		attachments = append(attachments, entity.Attachment{FileID: v})
	}

	mapFiles["files"] = attachments

	file, err := runCtx.Services.FileRegistry.GetAttachmentsInfo(c.Background(), mapFiles)
	if err != nil {
		return nil, err
	}

	ta := make([]fileregistry.FileInfo, 0)
	for _, v := range file["files"] {
		ta = append(ta, fileregistry.FileInfo{FileId: v.FileId, Size: v.Size, Name: v.Name})
	}

	return ta, nil
}

func (runCtx *BlockRunContext) makeNotificationAttachment() ([]fileregistry.FileInfo, error) {
	task, err := runCtx.Services.Storage.GetTaskRunContext(c.Background(), runCtx.WorkNumber)
	if err != nil {
		return nil, err
	}

	attachments := make([]entity.Attachment, 0)
	mapFiles := make(map[string][]entity.Attachment)
	for _, v := range task.InitialApplication.AttachmentFields {
		filesAttach, ok := task.InitialApplication.ApplicationBody.Get(v)
		if ok {
			switch data := filesAttach.(type) {
			case om.OrderedMap:
				fileId, get := data.Get("file_id")
				if !get {
					continue
				}

				attachments = append(attachments, entity.Attachment{FileID: fileId.(string)})
			case []interface{}:
				for _, vv := range data {
					fileMap := vv.(om.OrderedMap)
					fileId, oks := fileMap.Get("file_id")
					if !oks {
						continue
					}

					attachments = append(attachments, entity.Attachment{FileID: fileId.(string)})
				}
			}
		}
	}

	mapFiles["files"] = attachments

	file, err := runCtx.Services.FileRegistry.GetAttachmentsInfo(c.Background(), mapFiles)
	if err != nil {
		return nil, err
	}

	ta := make([]fileregistry.FileInfo, 0)
	for _, v := range file["files"] {
		ta = append(ta, fileregistry.FileInfo{FileId: v.FileId, Size: v.Size, Name: v.Name})
	}

	return ta, nil
}

func (runCtx *BlockRunContext) makeNotificationDescription(nodeName string) ([]om.OrderedMap, []e.Attachment, error) {
	descr, err := runCtx.Services.Storage.GetTaskRunContext(c.Background(), runCtx.WorkNumber)
	if err != nil {
		return nil, nil, err
	}

	apBody := flatArray(descr.InitialApplication.ApplicationBody)

	descriptions := make([]om.OrderedMap, 0)

	filesAttach, err := runCtx.makeNotificationAttachment()
	if err != nil {
		return nil, nil, err
	}

	attachments, err := runCtx.GetAttach(filesAttach)
	if err != nil {
		return nil, nil, err
	}

	files := make([]e.Attachment, 0, len(attachments.AttachmentsList))

	if len(apBody.Values()) != 0 {
		apBody.Set("attachLinks", attachments.AttachLinks)
		apBody.Set("attachExist", attachments.AttachExists)
		apBody.Set("attachList", attachments.AttachmentsList)
	}

	apBody, err = runCtx.excludeHiddenApplicationFields(apBody)
	if err != nil {
		return nil, nil, err
	}

	descriptions = append(descriptions, apBody)

	additionalForms, err := runCtx.Services.Storage.GetAdditionalDescriptionForms(runCtx.WorkNumber, nodeName)
	if err != nil {
		return nil, nil, err
	}

	for _, v := range additionalForms {
		attachmentFiles := make([]string, 0)

		for _, val := range v.Values() {
			file, ok := val.(om.OrderedMap)
			if !ok {
				continue
			}

			if fileId, fileOk := file.Get("file_id"); fileOk {
				attachmentFiles = append(attachmentFiles, fileId.(string))
			}
		}

		fileInfo, fileErr := runCtx.makeNotificationFormAttachment(attachmentFiles)
		if fileErr != nil {
			return nil, nil, err
		}

		attach, attachErr := runCtx.GetAttach(fileInfo)
		if attachErr != nil {
			return nil, nil, err
		}

		v.Set("attachLinks", attach.AttachLinks)
		v.Set("attachExist", attach.AttachExists)
		v.Set("attachList", attach.AttachmentsList)

		files = append(files, attach.AttachmentsList...)
		formDesc, errExclude := runCtx.excludeHiddenFormFields(nodeName, flatArray(v))
		if errExclude != nil {
			return nil, nil, errExclude
		}
		descriptions = append(descriptions, formDesc)
	}

	files = append(files, attachments.AttachmentsList...)
	return descriptions, files, nil
}

func (runCtx *BlockRunContext) excludeHiddenApplicationFields(desc om.OrderedMap) (om.OrderedMap, error) {
	taskRunContext, getDataErr := runCtx.Services.Storage.GetTaskRunContext(c.Background(), runCtx.WorkNumber)
	if getDataErr != nil {
		return desc, getDataErr
	}

	for i := range desc.Keys() {
		for j := range taskRunContext.InitialApplication.HiddenFields {
			if desc.Keys()[i] == taskRunContext.InitialApplication.HiddenFields[j] {
				desc.Delete(desc.Keys()[i])
			}
		}
	}

	return desc, nil
}

func (runCtx *BlockRunContext) excludeHiddenFormFields(formName string, desc om.OrderedMap) (om.OrderedMap, error) {
	var state FormData
	err := json.Unmarshal(runCtx.VarStore.State[formName], &state)
	if err != nil {
		return desc, err
	}

	for i := range desc.Keys() {
		for j := range state.HiddenFields {
			if desc.Keys()[i] == state.HiddenFields[j] {
				desc.Delete(desc.Keys()[i])
			}
		}
	}

	return desc, nil
}

func flatArray(v om.OrderedMap) om.OrderedMap {
	res := om.New()
	keys := v.Keys()
	values := v.Values()

	for _, k := range keys {
		vv, ok := values[k].([]interface{})
		if ok {
			for i, v := range vv {
				res.Set(k+"("+strconv.Itoa(i)+")", v)
			}
		} else {
			res.Set(k, values[k])
		}
	}

	return *res
}
