package pipeline

import (
	c "context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	om "github.com/iancoleman/orderedmap"
	"go.opencensus.io/trace"

	e "gitlab.services.mts.ru/abp/mail/pkg/email"
	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/fileregistry"
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
	fileIDKey   = "file_id"
	fileLinkKey = "external_link"

	decisionAttachmentsKey = "decision_attachments"
	attachmentsKey         = "attachments"

	filesType = "files"

	attachLinksKey = "attachLinks"
	attachExistKey = "attachExist"
	attachListKey  = "attachList"

	approverBlockType  = "approver"
	executionBlockType = "execution"
	signBlockType      = "sign"
)

//nolint:all // ok
func (runCtx *BlockRunContext) handleInitiatorNotify(ctx c.Context, params handleInitiatorNotifyParams) error {
	ctx, span := trace.StartSpan(ctx, "handle_initiator_notify")
	defer span.End()

	var email string

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

	description, files, err := runCtx.makeNotificationDescription(ctx, params.step, true)
	if err != nil {
		return err
	}

	log := logger.GetLogger(ctx)

	email, err = runCtx.Services.People.GetUserEmail(ctx, runCtx.Initiator)
	if err != nil {
		log.WithField("login", runCtx.Initiator).WithError(err).Warning("couldn't get email")

		return nil
	}

	if params.action == "" {
		params.action = statusToTaskState[params.status]
	}

	tmpl := mail.NewAppInitiatorStatusNotificationTpl(
		&mail.SignerNotifTemplate{
			WorkNumber:  runCtx.WorkNumber,
			Name:        runCtx.NotifName,
			SdURL:       runCtx.Services.Sender.SdAddress,
			JocastaURL:  runCtx.Services.JocastaURL,
			Description: description,
			Action:      params.action,
		})

	iconsName := []string{tmpl.Image}

	for _, v := range description {
		links, link := v.Get(attachLinksKey)
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

	if sendErr := runCtx.Services.Sender.SendNotification(ctx, []string{email}, files, tmpl); sendErr != nil {
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

func (runCtx *BlockRunContext) makeNotificationFormAttachment(ctx c.Context, files []string) ([]fileregistry.FileInfo, error) {
	attachments := make([]entity.Attachment, 0)
	mapFiles := make(map[string][]entity.Attachment)

	for _, v := range files {
		attachments = append(attachments, entity.Attachment{FileID: v})
	}

	mapFiles[filesType] = attachments

	file, err := runCtx.Services.FileRegistry.GetAttachmentsInfo(ctx, mapFiles)
	if err != nil {
		return nil, err
	}

	ta := make([]fileregistry.FileInfo, 0)
	for _, v := range file[filesType] {
		ta = append(ta, fileregistry.FileInfo{FileID: v.FileID, Size: v.Size, Name: v.Name})
	}

	return ta, nil
}

// nolint:gocognit,gocyclo //it's ok
func (runCtx *BlockRunContext) makeNotificationAttachment(ctx c.Context) ([]fileregistry.FileInfo, []fileregistry.AttachInfo, error) {
	if runCtx.skipNotifications {
		return []fileregistry.FileInfo{}, []fileregistry.AttachInfo{}, nil
	}

	task, err := runCtx.Services.Storage.GetTaskRunContext(ctx, runCtx.WorkNumber)
	if err != nil {
		return nil, nil, err
	}

	attachmentsList := make([]entity.Attachment, 0)
	attachmentsLinks := make([]fileregistry.AttachInfo, 0)

	mapFiles := make(map[string][]entity.Attachment)

	if getEmailAttachErr := runCtx.getEmailAttachments(&attachmentsList, &attachmentsLinks); getEmailAttachErr != nil {
		return nil, nil, getEmailAttachErr
	}

	if getUpdateParamsErr := runCtx.getUpdateParamsAttachments(&attachmentsList, &attachmentsLinks); getUpdateParamsErr != nil {
		return nil, nil, getUpdateParamsErr
	}

	for key, val := range task.InitialApplication.ApplicationBody.Values() {
		if utils.IsContainsInSlice(key, task.InitialApplication.HiddenFields) {
			continue
		}

		switch item := val.(type) {
		case []interface{}:
			for _, value := range item {
				fileMap, isMap := value.(om.OrderedMap)
				if !isMap {
					continue
				}

				if filesID, isFileID := fileMap.Get(fileIDKey); isFileID {
					attachmentsList = append(attachmentsList, entity.Attachment{FileID: filesID.(string)})
				}

				if fileLink, isFileLink := fileMap.Get(fileLinkKey); isFileLink {
					attachmentsLinks = append(attachmentsLinks, fileregistry.AttachInfo{ExternalLink: fileLink.(string)})
				}
			}
		case om.OrderedMap:
			if filesID, okGet := item.Get(fileIDKey); okGet {
				attachmentsList = append(attachmentsList, entity.Attachment{FileID: filesID.(string)})
			}

			if fileLink, isFileLink := item.Get(fileLinkKey); isFileLink {
				attachmentsLinks = append(attachmentsLinks, fileregistry.AttachInfo{ExternalLink: fileLink.(string)})
			}

			for keys, values := range item.Values() {
				if utils.IsContainsInSlice(keys, task.InitialApplication.HiddenFields) {
					continue
				}

				switch field := values.(type) {
				case om.OrderedMap:
					if fieldsID, okGet := field.Get(fileIDKey); okGet {
						attachmentsList = append(attachmentsList, entity.Attachment{FileID: fieldsID.(string)})
					}

					if fileLink, isFileLink := field.Get(fileLinkKey); isFileLink {
						attachmentsLinks = append(attachmentsLinks, fileregistry.AttachInfo{ExternalLink: fileLink.(string)})
					}
				case []interface{}:
					for _, value := range field {
						fileMap, isMap := value.(om.OrderedMap)
						if !isMap {
							continue
						}

						if filesID, okGet := fileMap.Get(fileIDKey); okGet {
							attachmentsList = append(attachmentsList, entity.Attachment{FileID: filesID.(string)})
						}

						if fileLink, isFileLink := fileMap.Get(fileLinkKey); isFileLink {
							attachmentsLinks = append(attachmentsLinks, fileregistry.AttachInfo{ExternalLink: fileLink.(string)})
						}
					}
				}
			}
		default:
			if filesID, okGet := task.InitialApplication.ApplicationBody.Get(fileIDKey); okGet {
				attachmentsList = append(attachmentsList, entity.Attachment{FileID: filesID.(string)})
			}

			if fileLink, isFileLink := task.InitialApplication.ApplicationBody.Get(fileLinkKey); isFileLink {
				attachmentsLinks = append(attachmentsLinks, fileregistry.AttachInfo{ExternalLink: fileLink.(string)})
			}
		}
	}

	mapFiles[filesType] = attachmentsList

	file, err := runCtx.Services.FileRegistry.GetAttachmentsInfo(ctx, mapFiles)
	if err != nil {
		return nil, nil, err
	}

	attachFiles := make([]fileregistry.FileInfo, 0)
	for _, v := range file[filesType] {
		attachFiles = append(attachFiles, fileregistry.FileInfo{FileID: v.FileID, Size: v.Size, Name: v.Name})
	}

	return attachFiles, attachmentsLinks, nil
}

// nolint:lll //it'ok
func (runCtx *BlockRunContext) getUpdateParamsAttachments(attachmentsList *[]entity.Attachment, attachmentsLinks *[]fileregistry.AttachInfo) error {
	if runCtx.UpdateData == nil || runCtx.UpdateData.Parameters == nil {
		return nil
	}

	params := make(map[string]interface{}, 0)

	err := json.Unmarshal(runCtx.UpdateData.Parameters, &params)
	if err != nil {
		return err
	}

	attachParams, isAttach := params[attachmentsKey]
	if !isAttach {
		return nil
	}

	attachArray, isArray := attachParams.([]interface{})
	if !isArray {
		return nil
	}

	for _, v := range attachArray {
		attachItem, isMap := v.(map[string]interface{})
		if !isMap {
			continue
		}

		filesID, isFileID := attachItem[fileIDKey]
		if isFileID {
			*attachmentsList = append(*attachmentsList, entity.Attachment{FileID: filesID.(string)})
		}

		externalLink, isExternalLink := attachItem[fileLinkKey]
		if isExternalLink {
			*attachmentsLinks = append(*attachmentsLinks, fileregistry.AttachInfo{ExternalLink: externalLink.(string)})
		}
	}

	return nil
}

// nolint:lll //it'ok
func (runCtx *BlockRunContext) getEmailAttachments(attachmentsList *[]entity.Attachment, attachmentsLinks *[]fileregistry.AttachInfo) error {
	for k, v := range runCtx.VarStore.Steps {
		isNotification := strings.Contains(v, "notification")
		if !isNotification {
			continue
		}

		blockName := runCtx.VarStore.Steps[k-1]

		block := runCtx.VarStore.State[blockName]

		if block == nil {
			continue
		}

		blockParams := make(map[string]interface{})
		if err := json.Unmarshal(block, &blockParams); err != nil {
			return err
		}

		attach, exAttach := blockParams[decisionAttachmentsKey]
		if !exAttach {
			continue
		}

		attachArray, isArray := attach.([]interface{})
		if !isArray {
			continue
		}

		for _, item := range attachArray {
			attachItem, isMap := item.(map[string]interface{})
			if !isMap {
				continue
			}

			filesID, isFieldID := attachItem[fileIDKey]
			if isFieldID {
				*attachmentsList = append(*attachmentsList, entity.Attachment{FileID: filesID.(string)})
			}

			externalLink, isExternalLink := attachItem[fileLinkKey]
			if isExternalLink {
				*attachmentsLinks = append(*attachmentsLinks, fileregistry.AttachInfo{ExternalLink: externalLink.(string)})
			}
		}
	}

	return nil
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
func (runCtx *BlockRunContext) makeNotificationDescription(ctx c.Context, stepName string, isInitiator bool) (
	[]om.OrderedMap, []e.Attachment, error,
) {
	taskContext, err := runCtx.Services.Storage.GetTaskRunContext(ctx, runCtx.WorkNumber)
	if err != nil {
		return nil, nil, err
	}

	var (
		descriptions = make([]om.OrderedMap, 0)
		files        = make([]e.Attachment, 0)
	)

	filesAttach, getAttachErr := runCtx.GetAttachmentFiles(ctx, &taskContext.InitialApplication.ApplicationBody, nil)
	if getAttachErr != nil {
		return nil, nil, getAttachErr
	}

	apDesc := flatArray(taskContext.InitialApplication.ApplicationBody)

	apDesc = convertDesc(apDesc, taskContext.InitialApplication.Keys, taskContext.InitialApplication.HiddenFields)

	descriptions = append(descriptions, apDesc)
	files = append(files, filesAttach...)

	adFormDescriptions, adFormFilesAttach := runCtx.getForms(ctx, stepName, isInitiator)
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

func (runCtx *BlockRunContext) getForms(ctx c.Context, stepName string, isInitiator bool) ([]om.OrderedMap, []e.Attachment) {
	forms, err := runCtx.Services.Storage.GetAdditionalDescriptionForms(runCtx.WorkNumber, stepName)
	if err != nil {
		return nil, nil
	}

	var (
		descriptions = make([]om.OrderedMap, 0)
		files        = make([]e.Attachment, 0)
	)

	for _, form := range forms {
		var formBlock FormData
		if marshalErr := json.Unmarshal(runCtx.VarStore.State[form.Name], &formBlock); marshalErr != nil {
			continue
		}

		_, isInitiatorExecutor := formBlock.Executors[runCtx.Initiator]
		if formBlock.HideFormFromInitiator && isInitiator && !isInitiatorExecutor {
			continue
		}

		attachmentFiles := getAdditionalAttachList(form, &formBlock)

		adDesc := flatArray(form.Description)

		additionalAttach, getAdAttachErr := runCtx.GetAttachmentFiles(ctx, &adDesc, attachmentFiles)
		if getAdAttachErr != nil {
			return nil, nil
		}

		adDesc = convertDesc(adDesc, formBlock.Keys, formBlock.HiddenFields)

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

				if filesID, fileOK := file.Get(fileIDKey); fileOK {
					attachmentFiles = append(attachmentFiles, filesID.(string))
				}
			case []interface{}:
				for _, val := range attach {
					valMap, isMap := val.(om.OrderedMap)
					if !isMap {
						continue
					}

					if filesID, fileOK := valMap.Get(fileIDKey); fileOK {
						attachmentFiles = append(attachmentFiles, filesID.(string))
					}
				}
			}
		}
	}

	return attachmentFiles
}

func (runCtx *BlockRunContext) GetAttachmentFiles(ctx c.Context, desc *om.OrderedMap, addAttach []string) ([]e.Attachment, error) {
	var (
		err              error
		filesAttach      []fileregistry.FileInfo
		filesAttachLinks []fileregistry.AttachInfo
	)

	if addAttach == nil {
		filesAttach, filesAttachLinks, err = runCtx.makeNotificationAttachment(ctx)
	} else {
		filesAttach, err = runCtx.makeNotificationFormAttachment(ctx, addAttach)
	}

	if err != nil {
		return nil, err
	}

	if len(filesAttachLinks) != 0 {
		return []e.Attachment{}, nil
	}

	attachments, err := runCtx.GetAttach(ctx, filesAttach)
	if err != nil {
		return nil, err
	}

	if len(filesAttach) != 0 || len(attachments.AttachLinks) != 0 || len(attachments.AttachmentsList) != 0 {
		desc.Set(attachLinksKey, attachments.AttachLinks)
		desc.Set(attachExistKey, attachments.AttachExists)
		desc.Set(attachListKey, attachments.AttachmentsList)
	}

	return attachments.AttachmentsList, nil
}

//nolint:gocognit //it's ok
func convertDesc(descriptions om.OrderedMap, keys map[string]string, hiddenFields []string) om.OrderedMap {
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
			if k == attachLinksKey || k == attachExistKey || k == attachListKey {
				newDesc.Set(k, v)

				continue
			}
		} else {
			k = keysSplit[0]
			k = strings.TrimSpace(k)
		}

		// skip hidden fields from flattened arrays
		if utils.IsContainsInSlice(k, hiddenFields) {
			continue
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
		if nameKey == fileIDKey {
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

func flatArray(desc om.OrderedMap) om.OrderedMap {
	res := om.New()

	for key, value := range desc.Values() {
		if key == fileLinkKey || key == fileIDKey {
			continue
		}

		array, ok := value.([]interface{})
		if ok {
			for ky, vl := range array {
				switch item := vl.(type) {
				case om.OrderedMap:
					for k, v := range item.Values() {
						if k == fileLinkKey || k == fileIDKey {
							continue
						}

						res.Set(fmt.Sprintf("%s(%s)", key, k), v)
					}
				default:
					res.Set(key+"("+strconv.Itoa(ky)+")", value)
				}
			}
		} else {
			res.Set(key, value)
		}
	}

	return *res
}
