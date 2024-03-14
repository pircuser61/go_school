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
	fileID    = "file_id"
	filesType = "files"

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

	mapFiles[filesType] = attachments

	file, err := runCtx.Services.FileRegistry.GetAttachmentsInfo(c.Background(), mapFiles)
	if err != nil {
		return nil, err
	}

	ta := make([]fileregistry.FileInfo, 0)
	for _, v := range file[filesType] {
		ta = append(ta, fileregistry.FileInfo{FileID: v.FileID, Size: v.Size, Name: v.Name})
	}

	return ta, nil
}

// nolint:gocognit //it's ok
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
				filesID, get := data.Get(fileID)
				if !get {
					continue
				}

				attachments = append(attachments, entity.Attachment{FileID: filesID.(string)})
			case []interface{}:
				for _, vv := range data {
					fileMap := vv.(om.OrderedMap)

					filesID, oks := fileMap.Get(fileID)
					if !oks {
						continue
					}

					attachments = append(attachments, entity.Attachment{FileID: filesID.(string)})
				}
			}
		}

		for _, val := range task.InitialApplication.ApplicationBody.Values() {
			group, oks := val.(om.OrderedMap)
			if !oks {
				continue
			}

			for _, vss := range group.Values() {
				switch field := vss.(type) {
				case om.OrderedMap:
					if fieldsID, okGet := field.Get(fileID); okGet {
						attachments = append(attachments, entity.Attachment{FileID: fieldsID.(string)})
					}
				case []interface{}:
					for _, vv := range field {
						fileMap := vv.(om.OrderedMap)

						filesID, okGet := fileMap.Get(fileID)
						if !okGet {
							continue
						}

						attachments = append(attachments, entity.Attachment{FileID: filesID.(string)})
					}
				}
			}
		}
	}

	mapFiles[filesType] = attachments

	file, err := runCtx.Services.FileRegistry.GetAttachmentsInfo(c.Background(), mapFiles)
	if err != nil {
		return nil, err
	}

	ta := make([]fileregistry.FileInfo, 0)
	for _, v := range file[filesType] {
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

	filesAttach, getAttachErr := runCtx.GetAttachmentFiles(&taskContext.InitialApplication.ApplicationBody, nil)
	if getAttachErr != nil {
		return nil, nil, getAttachErr
	}

	apDesc := flatArray(taskContext.InitialApplication.ApplicationBody)

	apDesc = GetConvertDesc(apDesc, taskContext.InitialApplication.Keys, taskContext.InitialApplication.HiddenFields)

	descriptions = append(descriptions, apDesc)
	files = append(files, filesAttach...)

	adFormDescriptions, adFormFilesAttach := runCtx.getAdditionalForms(nodeName)
	if len(adFormDescriptions) != 0 {
		descriptions = append(descriptions, adFormDescriptions...)
		files = append(files, adFormFilesAttach...)
	}

	cleanName(files)

	return descriptions, files, nil
}

func cleanName(files []e.Attachment) {
	for i := range files {
		if strings.Contains(files[i].Name, "UTF-8") {
			s := strings.Split(files[i].Name, "UTF-8")
			k := strings.ReplaceAll(s[1], "''", "")
			k = strings.ReplaceAll(k, "\"", "")
			files[i].Name = k
		}
	}
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

				if fileID, fileOK := file.Get(fileID); fileOK {
					attachmentFiles = append(attachmentFiles, fileID.(string))
				}
			case []interface{}:
				for _, val := range attach {
					valMap := val.(om.OrderedMap)
					if fileID, fileOK := valMap.Get(fileID); fileOK {
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

	if len(attachments.AttachmentsList) != 0 || len(attachments.AttachLinks) != 0 {
		desc.Set(attachLinks, attachments.AttachLinks)
		desc.Set(attachExist, attachments.AttachExists)
		desc.Set(attachList, attachments.AttachmentsList)
	}

	return attachments.AttachmentsList, nil
}

//nolint:gocognit //it's ok
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

		keysSplit := make([]string, 0)
		if strings.Contains(k, "(") {
			keysSplit = strings.Split(k, "(")
		}

		if len(keysSplit) == 0 {
			if k == attachLinks || k == attachExist || k == attachList {
				newDesc.Set(k, v)

				continue
			}
		} else {
			k = keysSplit[0]
			k = strings.TrimSpace(k)
		}

		var (
			ruKey string
			ok    bool
		)

		if len(keysSplit) > 0 {
			nameKey := strings.Replace(keysSplit[1], ")", "", 1)
			ruKey, ok = keys[nameKey]
		}

		if !ok {
			ruKey, ok = keys[k]
			if !ok {
				continue
			}
		}

		if v == "" {
			continue
		}

		if _, existKey := newDesc.Get(ruKey); existKey {
			newDesc.Set(fmt.Sprintf("%s %-*s", ruKey, spaceCount, " "), v)
			spaceCount++

			continue
		}

		if len(keysSplit) < 1 {
			newDesc.Set(ruKey, v)

			continue
		}

		nameKey := strings.Replace(keysSplit[1], ")", "", 1)
		if nameKey == fileID {
			continue
		}

		if ok {
			key := fmt.Sprintf("%s (%s)", ruKey, nameKey)

			if num, oks := v.(float64); oks {
				strNum := strings.Split(fmt.Sprintf("%f", num), ".")[0]
				newDesc.Set(key, strNum)

				continue
			}

			newDesc.Set(key, v)

			continue
		}

		newDesc.Set(ruKey, v)
	}

	return newDesc
}

func checkGroup(schema om.OrderedMap) om.OrderedMap {
	const email = "email"

	for k, v := range schema.Values() {
		val, ok := v.(om.OrderedMap)
		if !ok {
			continue
		}

		if _, user := val.Get(email); user {
			continue
		}

		for key, value := range val.Values() {
			values, oks := value.(om.OrderedMap)
			if !oks {
				key = fmt.Sprintf("%s (%s)", k, key)
				schema.Set(key, value)

				continue
			}

			if _, user := values.Get(email); user {
				continue
			}

			for ky, vl := range values.Values() {
				ky = fmt.Sprintf("%s (%s)", key, ky)
				schema.Set(ky, vl)

				continue
			}

			if _, okay := schema.Get(k); okay {
				continue
			}

			key = fmt.Sprintf("%s (%s)", k, key)
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
