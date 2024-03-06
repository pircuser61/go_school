package pipeline

import (
	c "context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	om "github.com/iancoleman/orderedmap"

	e "gitlab.services.mts.ru/abp/mail/pkg/email"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	fileregistry "gitlab.services.mts.ru/jocasta/pipeliner/internal/fileregistry"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

type handleInitiatorNotifyParams struct {
	step     string
	stepType string
	action   string
	status   TaskHumanStatus
}

const (
	attachLinks = "attachLinks"
	attachExist = "attachExist"
	attachList  = "attachList"
)

func (runCtx *BlockRunContext) handleInitiatorNotify(ctx c.Context, params handleInitiatorNotifyParams) error {
	const (
		FormStepType     = "form"
		TimerStepType    = "timer"
		FunctionStepType = "executable_function"
	)

	if runCtx.skipNotifications {
		return nil
	}

	//nolint:exhaustive //нам не нужно обрабатывать остальные случаи
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
			SdURL:       runCtx.Services.Sender.SdAddress,
			Description: description,
			Action:      params.action,
		})

	iconsName := []string{tmpl.Image}

	for _, v := range description {
		links, link := v.Get(attachLinks)
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
		ta = append(ta, fileregistry.FileInfo{FileID: v.FileID, Size: v.Size, Name: v.Name})
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

	notHiddenAttachmentFields := filterHiddenAttachmentFields(task.InitialApplication.AttachmentFields, task.InitialApplication.HiddenFields)

	for _, v := range notHiddenAttachmentFields {
		filesAttach, ok := task.InitialApplication.ApplicationBody.Get(v)
		if ok {
			switch data := filesAttach.(type) {
			case om.OrderedMap:
				fileID, get := data.Get("file_id")
				if !get {
					continue
				}

				attachments = append(attachments, entity.Attachment{FileID: fileID.(string)})
			case []interface{}:
				for _, vv := range data {
					fileMap := vv.(om.OrderedMap)

					fileID, oks := fileMap.Get("file_id")
					if !oks {
						continue
					}

					attachments = append(attachments, entity.Attachment{FileID: fileID.(string)})
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
		ta = append(ta, fileregistry.FileInfo{FileID: v.FileID, Size: v.Size, Name: v.Name})
	}

	return ta, nil
}

func filterHiddenAttachmentFields(attachmentFields, hiddenFields []string) []string {
	hiddenFieldsMap := make(map[string]struct{}, len(hiddenFields))

	for _, field := range hiddenFields {
		hiddenFieldsMap[field] = struct{}{}
	}

	filteredAttachmentFields := make([]string, 0)

	for _, field := range attachmentFields {
		_, hidden := hiddenFieldsMap[field]
		if !hidden {
			filteredAttachmentFields = append(filteredAttachmentFields, field)
		}
	}

	return filteredAttachmentFields
}

func cleanKey(s string) string {
	replacements := map[string]string{
		"\\t": "",
		"\t":  "",
		"\\n": "",
		"\n":  "",
		"\r":  "",
		"\\r": "",
	}

	for old, news := range replacements {
		s = strings.ReplaceAll(s, old, news)
	}

	s = strings.ReplaceAll(s, "\\", "")

	return s
}

//nolint:gocognit,gocyclo // данный нейминг хорошо описывает механику метода
func (runCtx *BlockRunContext) makeNotificationDescription(nodeName string) ([]om.OrderedMap, []e.Attachment, error) {
	taskContext, err := runCtx.Services.Storage.GetTaskRunContext(c.Background(), runCtx.WorkNumber)
	if err != nil {
		return nil, nil, err
	}

	var (
		descriptions = make([]om.OrderedMap, 0)
		files        = make([]e.Attachment, 0)
	)

	apDesc := flatArray(taskContext.InitialApplication.ApplicationBody)

	filesAttach, getAttachErr := runCtx.GetAttachmentFiles(&apDesc, nil)
	if getAttachErr != nil {
		return nil, nil, getAttachErr
	}

	apDesc = GetConvertDesc(apDesc, taskContext.InitialApplication.Keys, taskContext.InitialApplication.HiddenFields)

	descriptions = append(descriptions, apDesc)
	files = append(files, filesAttach...)

	adFormDescriptions, adFormFilesAttach := runCtx.getAdditionalForms(nodeName)
	if len(adFormDescriptions) != 0 {
		descriptions = append(descriptions, adFormDescriptions...)
		files = append(files, adFormFilesAttach...)
	}

	return descriptions, files, nil
}

func (runCtx *BlockRunContext) getAdditionalForms(nodeName string) ([]om.OrderedMap, []e.Attachment) {
	additionalForms, err := runCtx.Services.Storage.GetAdditionalDescriptionForms(runCtx.WorkNumber, nodeName)
	if err != nil {
		return nil, nil
	}

	var (
		descriptions = make([]om.OrderedMap, 0)
		files        = make([]e.Attachment, 0)
	)

	for _, form := range additionalForms {
		var formBlock FormData
		if marshalErr := json.Unmarshal(runCtx.VarStore.State[form.Name], &formBlock); marshalErr != nil {
			return nil, nil
		}

		attachmentFiles := getAdditionalAttachList(form, &formBlock)

		adDesc := flatArray(form.Description)

		additionalAttach, getAdAttachErr := runCtx.GetAttachmentFiles(&adDesc, attachmentFiles)
		if getAdAttachErr != nil {
			return nil, nil
		}

		adDesc = GetConvertDesc(adDesc, formBlock.Keys, formBlock.HiddenFields)

		files = append(files, additionalAttach...)
		descriptions = append(descriptions, adDesc)
	}

	return descriptions, files
}

//nolint:gocognit // it's ok
func getAdditionalAttachList(form entity.DescriptionForm, formData *FormData) []string {
	attachmentFiles := make([]string, 0)

	for k, v := range form.Description.Values() {
		notHiddenAttachmentFields := filterHiddenAttachmentFields(formData.AttachmentFields, formData.HiddenFields)

		for _, attachVal := range notHiddenAttachmentFields {
			if attachVal != k {
				continue
			}

			switch attach := v.(type) {
			case om.OrderedMap:
				file, attachOk := v.(om.OrderedMap)
				if !attachOk {
					continue
				}

				if fileID, fileOK := file.Get("file_id"); fileOK {
					attachmentFiles = append(attachmentFiles, fileID.(string))
				}
			case []interface{}:
				for _, val := range attach {
					valMap := val.(om.OrderedMap)
					if fileID, fileOK := valMap.Get("file_id"); fileOK {
						attachmentFiles = append(attachmentFiles, fileID.(string))
					}
				}
			}
		}
	}

	return attachmentFiles
}

func (runCtx *BlockRunContext) GetAttachmentFiles(desc *om.OrderedMap, addAttach []string) ([]e.Attachment, error) {
	var (
		err         error
		filesAttach []fileregistry.FileInfo
	)

	if addAttach == nil {
		filesAttach, err = runCtx.makeNotificationAttachment()
	} else {
		filesAttach, err = runCtx.makeNotificationFormAttachment(addAttach)
	}

	if err != nil {
		return nil, err
	}

	attachments, err := runCtx.GetAttach(filesAttach)
	if err != nil {
		return nil, err
	}

	if attachments != nil {
		desc.Set(attachLinks, attachments.AttachLinks)
		desc.Set(attachExist, attachments.AttachExists)
		desc.Set(attachList, attachments.AttachmentsList)
	}

	return attachments.AttachmentsList, nil
}

func GetConvertDesc(descriptions om.OrderedMap, keys map[string]string, hiddenFields []string) om.OrderedMap {
	var (
		newDesc    = *om.New()
		spaceCount = 1
	)

	descriptions = checkGroup(descriptions)

	for k, v := range descriptions.Values() {
		if utils.IsContainsInSlice(k, hiddenFields) {
			continue
		}

		if strings.Contains(k, "(") {
			k = strings.Split(k, "(")[0]
		}

		if k == attachLinks || k == attachExist || k == attachList {
			newDesc.Set(k, v)

			continue
		}

		ruKey, ok := keys[k]
		if !ok {
			continue
		}

		ruKey = cleanKey(ruKey)
		if _, existKey := newDesc.Get(ruKey); !existKey {
			newDesc.Set(ruKey, v)

			continue
		}

		newDesc.Set(fmt.Sprintf("%s %-*s", ruKey, spaceCount, " "), v)
		spaceCount++
	}

	return newDesc
}

func checkGroup(schema om.OrderedMap) om.OrderedMap {
	for k, v := range schema.Values() {
		val, ok := v.(om.OrderedMap)
		if !ok {
			continue
		}

		if _, user := val.Get("email"); user {
			continue
		}

		for key, value := range val.Values() {
			schema.Set(key, value)
		}

		schema.Delete(k)
	}

	return schema
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
