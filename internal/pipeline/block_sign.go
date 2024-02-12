package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/iancoleman/orderedmap"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	file_registry "gitlab.services.mts.ru/jocasta/pipeliner/internal/fileregistry"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sla"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

const (
	autoSigner = "auto_signer"

	keyOutputSigner          = "signer"
	keyOutputSignDecision    = "decision"
	keyOutputSignComment     = "comment"
	keyOutputSignAttachments = "attachments"
	keyOutputSignatures      = "signatures"

	SignDecisionSigned              SignDecision = "signed"   // signed by signer
	SignDecisionRejected            SignDecision = "rejected" // rejected by signer or by additional approver
	SignDecisionError               SignDecision = "error"
	SignDecisionAddApproverApproved SignDecision = "approved" // approved by additional approver

	signActionSign                  = "sign_sign"
	signActionReject                = "sign_reject"
	signActionTakeInWork            = "sign_start_work"
	signActionAddApprovers          = "add_approvers"
	signActionAdditionalApprovement = "additional_approvement"
	signActionAdditionalReject      = "additional_reject"

	signatureTypeActionParamsKey    = "signature_type"
	signatureCarrierActionParamsKey = "signature_carrier"

	signatureINNParamsKey   = "inn"
	signatureSNILSParamsKey = "snils"
	signatureFilesParamsKey = "files"

	reentrySignComment = "Произошла ошибка подписания. Требуется повторное подписание"
)

type GoSignBlock struct {
	Name      string
	ShortName string
	Title     string
	Input     map[string]string
	Output    map[string]string
	Sockets   []script.Socket
	State     *SignData

	RunContext *BlockRunContext

	expectedEvents map[string]struct{}
	happenedEvents []entity.NodeEvent
}

func (gb *GoSignBlock) CurrentExecutorData() CurrentExecutorData {
	return CurrentExecutorData{}
}

func (gb *GoSignBlock) GetNewEvents() []entity.NodeEvent {
	return gb.happenedEvents
}

func (gb *GoSignBlock) GetState() interface{} {
	return gb.State
}

func (gb *GoSignBlock) Next(_ *store.VariableStore) ([]string, bool) {
	var key string

	if gb.State != nil && gb.State.Decision != nil {
		//nolint:exhaustive //не хотим обрабатывать остальные случаи
		switch *gb.State.Decision {
		case SignDecisionSigned:
			key = signedSocketID
		case SignDecisionRejected:
			key = rejectedSocketID
		case SignDecisionError:
			key = errorSocketID
		}
	}

	nexts, ok := script.GetNexts(gb.Sockets, key)
	if !ok {
		return nil, false
	}

	return nexts, true
}

func (gb *GoSignBlock) GetTaskHumanStatus() (status TaskHumanStatus, comment, action string) {
	if gb.State != nil && gb.State.Decision != nil {
		if *gb.State.Decision == SignDecisionRejected {
			return StatusRejected, "", ""
		}

		if *gb.State.Decision == SignDecisionError {
			return StatusProcessingError, "", ""
		}

		return StatusSigned, "", ""
	}

	if gb.State.Reentered {
		return StatusSigning, reentrySignComment, ""
	}

	return StatusSigning, "", ""
}

func (gb *GoSignBlock) GetStatus() Status {
	if gb.State != nil && gb.State.Decision != nil {
		if *gb.State.Decision == SignDecisionRejected {
			return StatusNoSuccess
		}

		if *gb.State.Decision == SignDecisionError {
			return StatusError
		}

		return StatusFinished
	}

	return StatusRunning
}

func (gb *GoSignBlock) UpdateManual() bool {
	return true
}

func (gb *GoSignBlock) isSignerActed(login string) bool {
	//nolint:gocritic //в этом проекте не принято использовать поинтеры в коллекциях
	for _, s := range gb.State.SignLog {
		if s.LogType != SignerLogDecision {
			continue
		}

		if s.Login == login {
			return true
		}
	}

	return false
}

func (gb *GoSignBlock) signActions(login string) []MemberAction {
	if gb.State.Decision != nil {
		return []MemberAction{}
	}

	//nolint:gocritic //в этом проекте не принято использовать поинтеры в коллекциях
	for _, s := range gb.State.SignLog {
		if s.Login == login && s.LogType == SignerLogDecision {
			return []MemberAction{}
		}
	}

	rejectAction := MemberAction{
		ID:   signActionReject,
		Type: ActionTypeSecondary,
	}

	addApproversAction := MemberAction{
		ID:   signActionAddApprovers,
		Type: ActionTypeOther,
	}

	if gb.State.SignatureType == script.SignatureTypeUKEP {
		takeInWorkAction := MemberAction{
			ID:   signActionTakeInWork,
			Type: ActionTypePrimary,
			Params: map[string]interface{}{
				signatureTypeActionParamsKey:    gb.State.SignatureType,
				signatureCarrierActionParamsKey: gb.State.SignatureCarrier,
				signatureINNParamsKey:           gb.State.SigningParams.INN,
				signatureSNILSParamsKey:         gb.State.SigningParams.SNILS,
				signatureFilesParamsKey:         gb.State.SigningParams.Files,
			},
		}

		if gb.State.IsTakenInWork && login != gb.State.WorkerLogin {
			takeInWorkAction.Params["disabled"] = true
			rejectAction.Params = map[string]interface{}{"disabled": true}
		}

		return []MemberAction{takeInWorkAction, rejectAction, addApproversAction}
	}

	signAction := []MemberAction{{
		ID:   signActionSign,
		Type: ActionTypePrimary,
		Params: map[string]interface{}{
			signatureTypeActionParamsKey: gb.State.SignatureType,
		},
	}}

	signAction = append(signAction, rejectAction)

	fillFormNames, existEmptyForm := gb.getFormNamesToFill()
	if existEmptyForm {
		for i := 0; i < len(signAction); i++ {
			item := &signAction[i]

			if item.ID != signActionSign {
				continue
			}

			item.Params = map[string]interface{}{"disabled": true}
		}
	}

	if len(fillFormNames) != 0 {
		signAction = append(signAction, MemberAction{
			ID:   formFillFormAction,
			Type: ActionTypeCustom,
			Params: map[string]interface{}{
				formName: fillFormNames,
			},
		})
	}

	signAction = append(signAction, addApproversAction)

	return signAction
}

//nolint:dupl //its not duplicate
func (gb *GoSignBlock) getFormNamesToFill() ([]string, bool) {
	var (
		actions   = make([]string, 0)
		emptyForm = false
		l         = logger.GetLogger(context.Background())
	)

	for _, form := range gb.State.FormsAccessibility {
		formState, ok := gb.RunContext.VarStore.State[form.NodeID]
		if !ok {
			continue
		}

		switch form.AccessType {
		case readWriteAccessType:
			actions = append(actions, form.NodeID)
		case requiredFillAccessType:
			actions = append(actions, form.NodeID)

			existEmptyForm := gb.checkForEmptyForm(formState, l)
			if existEmptyForm {
				emptyForm = true
			}
		}
	}

	return actions, emptyForm
}

func (gb *GoSignBlock) checkForEmptyForm(formState json.RawMessage, l logger.Logger) bool {
	var formData FormData
	if err := json.Unmarshal(formState, &formData); err != nil {
		l.Error(err)

		return true
	}

	if !formData.IsFilled {
		return true
	}

	for _, v := range formData.ChangesLog {
		if _, findOk := gb.State.Signers[v.Executor]; findOk {
			return false
		}
	}

	return true
}

func (gb *GoSignBlock) signAddActions(a *AdditionalSignApprover) []MemberAction {
	if gb.State.Decision != nil || a.Decision != nil {
		return []MemberAction{}
	}

	return []MemberAction{
		{
			ID:   signActionAdditionalApprovement,
			Type: ActionTypePrimary,
		},
		{
			ID:   signActionAdditionalReject,
			Type: ActionTypeSecondary,
		},
		{
			ID:   signActionAddApprovers,
			Type: ActionTypeOther,
		},
	}
}

func (gb *GoSignBlock) Members() []Member {
	members := make([]Member, 0)
	for login := range gb.State.Signers {
		members = append(members, Member{
			Login:                login,
			Actions:              gb.signActions(login),
			IsActed:              gb.isSignerActed(login),
			ExecutionGroupMember: false,
		})
	}

	for i := 0; i < len(gb.State.AdditionalApprovers); i++ {
		addApprover := gb.State.AdditionalApprovers[i]

		members = append(members, Member{
			Login:                addApprover.ApproverLogin,
			Actions:              gb.signAddActions(&addApprover),
			IsActed:              gb.isSignerActed(addApprover.ApproverLogin),
			ExecutionGroupMember: false,
		})
	}

	return members
}

func (gb *GoSignBlock) getDeadline(ctx context.Context, workType string) (time.Time, error) {
	slaInfoPtr, getSLAInfoErr := gb.RunContext.Services.SLAService.GetSLAInfoPtr(ctx, sla.InfoDTO{
		TaskCompletionIntervals: []entity.TaskCompletionInterval{{
			StartedAt:  gb.RunContext.CurrBlockStartTime,
			FinishedAt: gb.RunContext.CurrBlockStartTime.Add(time.Hour * 24 * 100),
		}},
		WorkType: sla.WorkHourType(workType),
	})
	if getSLAInfoErr != nil {
		return time.Time{}, errors.Wrap(getSLAInfoErr, "can not get slaInfo")
	}

	return gb.RunContext.Services.SLAService.ComputeMaxDate(gb.RunContext.CurrBlockStartTime, float32(*gb.State.SLA), slaInfoPtr), nil
}

//nolint:dupl,gocyclo //Need here
func (gb *GoSignBlock) Deadlines(ctx context.Context) ([]Deadline, error) {
	deadlines := make([]Deadline, 0, 2)

	if gb.State.Decision != nil {
		return []Deadline{}, nil
	}

	if gb.State.CheckSLA != nil && *gb.State.CheckSLA {
		slaInfoPtr, getSLAInfoErr := gb.RunContext.Services.SLAService.GetSLAInfoPtr(ctx, sla.InfoDTO{
			TaskCompletionIntervals: []entity.TaskCompletionInterval{{
				StartedAt:  gb.RunContext.CurrBlockStartTime,
				FinishedAt: gb.RunContext.CurrBlockStartTime.Add(time.Hour * 24 * 100),
			}},
			WorkType: sla.WorkHourType(*gb.State.WorkType),
		})

		if getSLAInfoErr != nil {
			return nil, getSLAInfoErr
		}

		deadline := gb.RunContext.Services.SLAService.ComputeMaxDate(gb.RunContext.CurrBlockStartTime, float32(*gb.State.SLA), slaInfoPtr)

		if !gb.State.SLAChecked {
			deadlines = append(
				deadlines,
				Deadline{
					Deadline: deadline,
					Action:   entity.TaskUpdateActionSLABreach,
				},
			)
		}

		if *gb.State.SLA > 8 && !gb.State.DayBeforeSLAChecked {
			notifyBeforeDayExpireSLA := deadline.Add(-8 * time.Hour)

			deadlines = append(deadlines,
				Deadline{
					Deadline: notifyBeforeDayExpireSLA,
					Action:   entity.TaskUpdateActionDayBeforeSLABreach,
				},
			)
		}
	}

	return deadlines, nil
}

func (gb *GoSignBlock) setWorkTypeAndDeadline(ctx context.Context, params *script.SignParams) error {
	if params.WorkType != nil {
		gb.State.WorkType = params.WorkType
	} else {
		task, getVersionErr := gb.RunContext.Services.Storage.GetVersionByWorkNumber(ctx, gb.RunContext.WorkNumber)
		if getVersionErr != nil {
			return getVersionErr
		}

		processSLASettings, getVersionErr := gb.RunContext.Services.Storage.GetSLAVersionSettings(
			ctx, task.VersionID.String())
		if getVersionErr != nil {
			return getVersionErr
		}

		gb.State.WorkType = &processSLASettings.WorkType
	}

	deadline, err := gb.getDeadline(ctx, *gb.State.WorkType)
	if err != nil {
		return err
	}

	gb.State.Deadline = deadline

	return nil
}

type setSignersByParamsDTO struct {
	Type    script.SignerType
	GroupID string
	Signer  string
}

func (gb *GoSignBlock) setSignersByParams(ctx context.Context, dto *setSignersByParamsDTO) error {
	switch dto.Type {
	case script.SignerTypeUser:
		gb.State.Signers = map[string]struct{}{
			dto.Signer: {},
		}
	case script.SignerTypeGroup:
		workGroup, errGroup := gb.RunContext.Services.ServiceDesc.GetWorkGroup(ctx, dto.GroupID)
		if errGroup != nil {
			return errors.Wrap(errGroup, "can`t get signer group with id: "+dto.GroupID)
		}

		if len(workGroup.People) == 0 {
			return errors.New("zero signers in group: " + dto.GroupID)
		}

		gb.State.Signers = make(map[string]struct{})
		for i := range workGroup.People {
			gb.State.Signers[workGroup.People[i].Login] = struct{}{}
		}

		gb.State.SignerGroupID = dto.GroupID
		gb.State.SignerGroupName = workGroup.GroupName
	case script.SignerTypeFromSchema:
		variableStorage, grabStorageErr := gb.RunContext.VarStore.GrabStorage()
		if grabStorageErr != nil {
			return grabStorageErr
		}

		signersFromSchema := make(map[string]struct{})

		signersVars := strings.Split(dto.Signer, ";")
		for i := range signersVars {
			resolvedEntities, resolveErr := getUsersFromVars(
				variableStorage,
				map[string]struct{}{
					signersVars[i]: {},
				},
			)
			if resolveErr != nil {
				return resolveErr
			}

			for signerLogin := range resolvedEntities {
				signersFromSchema[signerLogin] = struct{}{}
			}
		}

		gb.State.Signers = signersFromSchema
	}

	return nil
}

func (gb *GoSignBlock) handleDayBeforeSLANotifications(ctx context.Context) error {
	if gb.State.DayBeforeSLAChecked {
		return nil
	}

	if err := gb.handleNotifications(ctx); err != nil {
		return err
	}

	gb.State.DayBeforeSLAChecked = true

	return nil
}

//nolint:dupl // maybe later
func (gb *GoSignBlock) handleNotifications(ctx context.Context) error {
	l := logger.GetLogger(ctx)

	if gb.RunContext.skipNotifications {
		return nil
	}

	signers := getSliceFromMapOfStrings(gb.State.Signers)

	description, files, err := gb.RunContext.makeNotificationDescription(gb.Name)
	if err != nil {
		return err
	}

	slaDeadline := ""

	if gb.State.SLA != nil && gb.State.WorkType != nil {
		slaInfoPtr, getSLAInfoErr := gb.RunContext.Services.SLAService.GetSLAInfoPtr(ctx, sla.InfoDTO{
			TaskCompletionIntervals: []entity.TaskCompletionInterval{{
				StartedAt:  gb.RunContext.CurrBlockStartTime,
				FinishedAt: gb.RunContext.CurrBlockStartTime.Add(time.Hour * 24 * 100),
			}},
			WorkType: sla.WorkHourType(*gb.State.WorkType),
		})
		if getSLAInfoErr != nil {
			return getSLAInfoErr
		}

		slaDeadline = gb.RunContext.Services.SLAService.ComputeMaxDateFormatted(gb.RunContext.CurrBlockStartTime,
			*gb.State.SLA, slaInfoPtr)
	}

	emails := make(map[string]mail.Template, 0)

	usersNotToNotify := gb.getUsersNotToNotifySet()

	for _, login := range signers {
		if _, ok := usersNotToNotify[login]; ok {
			continue
		}

		em, getUserEmailErr := gb.RunContext.Services.People.GetUserEmail(ctx, login)
		if getUserEmailErr != nil {
			l.WithField("login", login).WithError(getUserEmailErr).Warning("couldn't get email")

			continue
		}

		emails[em] = mail.NewSignerNotificationTpl(
			&mail.SignerNotifTemplate{
				WorkNumber:  gb.RunContext.WorkNumber,
				Name:        gb.RunContext.NotifName,
				SdURL:       gb.RunContext.Services.Sender.SdAddress,
				Deadline:    slaDeadline,
				AutoReject:  gb.State.AutoReject != nil && *gb.State.AutoReject,
				Description: description,
			})
	}

	if len(emails) == 0 {
		return nil
	}

	for i := range emails {
		item := emails[i]

		iconsName := append([]string{item.Image}, gb.getNotificationImages(description)...)

		iconFiles, filesErr := gb.RunContext.GetIcons(iconsName)
		if filesErr != nil {
			return err
		}

		iconFiles = append(iconFiles, files...)

		if sendErr := gb.RunContext.Services.Sender.SendNotification(ctx, []string{i}, iconFiles,
			emails[i]); sendErr != nil {
			return sendErr
		}
	}

	return nil
}

func lookForFileIDInObject(object map[string]interface{}) (string, error) {
	existingFileID, ok := object["file_id"]
	if !ok {
		return "", errors.New("file_id does not exist in object")
	}

	fileID, ok := existingFileID.(string)
	if !ok {
		return "", errors.New("failed to type assert path to string")
	}

	return fileID, nil
}

func ValidateFiles(file interface{}) ([]entity.Attachment, error) {
	resFiles := make([]entity.Attachment, 0)

	switch f := file.(type) {
	case map[string]interface{}:
		fileID, err := lookForFileIDInObject(f)
		if err != nil {
			return nil, err
		}

		resFiles = append(resFiles, entity.Attachment{FileID: fileID})
	case []interface{}:
		filesFromInterfaces := make([]entity.Attachment, 0)

		for _, item := range f {
			object, ok := item.(map[string]interface{})
			if !ok {
				continue
			}

			fileID, err := lookForFileIDInObject(object)
			if err != nil {
				continue
			}

			filesFromInterfaces = append(filesFromInterfaces, entity.Attachment{FileID: fileID})
		}

		if len(filesFromInterfaces) == 0 {
			return nil, errors.New("did not find file in array")
		}

		resFiles = append(resFiles, filesFromInterfaces...)
	default:
		return nil, errors.New("did not get an object or an array to look for file")
	}

	return resFiles, nil
}

//nolint:dupl //its not duplicate
func (gb *GoSignBlock) createState(ctx context.Context, ef *entity.EriusFunc) error {
	l := logger.GetLogger(ctx)

	var params script.SignParams

	err := json.Unmarshal(ef.Params, &params)
	if err != nil {
		return errors.Wrap(err, "can not get sign parameters")
	}

	if err = params.Validate(); err != nil {
		return errors.Wrap(err, "invalid sign parameters")
	}

	gb.State = &SignData{
		Type:               params.Type,
		SigningRule:        params.SigningRule,
		SignLog:            make([]SignLogEntry, 0),
		Signatures:         make([]fileSignaturePair, 0),
		SigningParamsPaths: params.SigningParamsPaths,
		FormsAccessibility: params.FormsAccessibility,
		SignatureType:      params.SignatureType,
		SignatureCarrier:   params.SignatureCarrier,
		SLA:                params.SLA,
		CheckSLA:           params.CheckSLA,
		AutoReject:         params.AutoReject,
		WorkType:           params.WorkType,
	}

	if gb.State.SigningRule == "" {
		gb.State.SigningRule = script.AnyOfSigningRequired
	}

	variableStorage, grabStorageErr := gb.RunContext.VarStore.GrabStorage()
	if grabStorageErr != nil {
		return grabStorageErr
	}

	if params.Type == script.SignerTypeGroup && params.SignerGroupIDPath != "" {
		groupID := getVariable(variableStorage, params.SignerGroupIDPath)
		if groupID == nil {
			return errors.New("can't find group id in variables")
		}

		params.SignerGroupID = fmt.Sprintf("%v", groupID)
	}

	if params.SignatureType == script.SignatureTypeUKEP &&
		(params.SignatureCarrier == script.SignatureCarrierToken ||
			params.SignatureCarrier == script.SignatureCarrierAll) {
		inn := getVariable(variableStorage, params.SigningParamsPaths.INN)

		innString, ok := inn.(string)
		if !ok {
			l.Error(errors.New("could not find inn"))
		}

		gb.State.SigningParams.INN = innString

		snils := getVariable(variableStorage, params.SigningParamsPaths.SNILS)

		snilsString, ok := snils.(string)
		if !ok {
			l.Error(errors.New("could not find snils"))
		}

		gb.State.SigningParams.SNILS = snilsString

		filesForSigningParams := make([]entity.Attachment, 0)

		for _, pathToFiles := range params.SigningParamsPaths.Files {
			filesInterface := getVariable(variableStorage, pathToFiles)

			files, err := ValidateFiles(filesInterface)
			if err != nil {
				l.Error(err)

				continue
			}

			filesForSigningParams = append(filesForSigningParams, files...)
		}

		gb.State.SigningParams.Files = filesForSigningParams
	}

	if deadlineErr := gb.setWorkTypeAndDeadline(ctx, &params); deadlineErr != nil {
		return deadlineErr
	}

	setErr := gb.setSignersByParams(ctx, &setSignersByParamsDTO{
		Type:    params.Type,
		GroupID: params.SignerGroupID,
		Signer:  params.Signer,
	})
	if setErr != nil {
		return setErr
	}

	return gb.handleNotifications(ctx)
}

func (gb *GoSignBlock) Model() script.FunctionModel {
	return script.FunctionModel{
		ID:        BlockGoSignID,
		BlockType: script.TypeGo,
		Title:     gb.Title,
		Inputs:    nil,
		Outputs: &script.JSONSchema{
			Type: "object",
			Properties: script.JSONSchemaProperties{
				keyOutputSigner: {
					Type:        "object",
					Description: "signer login",
					Format:      "SsoPerson",
					Properties:  people.GetSsoPersonSchemaProperties(),
				},
				keyOutputSignDecision: {
					Type:        "string",
					Description: "sign result",
				},
				keyOutputSignComment: {
					Type:        "string",
					Description: "sign comment",
				},
				keyOutputSignAttachments: {
					Type:        "array",
					Description: "signed files",
					Items: &script.ArrayItems{
						Type:   "object",
						Format: "file",
						Properties: map[string]script.JSONSchemaPropertiesValue{
							"file_id": {
								Type:        "string",
								Description: "file id in file Registry",
							},
							"external_link": {
								Type:        "string",
								Description: "link to file in another system",
							},
						},
					},
				},
				keyOutputSignatures: {
					Type:        "array",
					Description: "signatures",
					Items: &script.ArrayItems{
						Type: "object",
						Properties: map[string]script.JSONSchemaPropertiesValue{
							"file": {
								Type:        "object",
								Description: "file to sign",
								Format:      "file",
								Properties: map[string]script.JSONSchemaPropertiesValue{
									"file_id": {
										Type:        "string",
										Description: "file id in file Registry",
									},
									"external_link": {
										Type:        "string",
										Description: "link to file in another system",
									},
								},
							},
							"signature_file": {
								Type:        "object",
								Description: "signature file",
								Format:      "file",
								Properties: map[string]script.JSONSchemaPropertiesValue{
									"file_id": {
										Type:        "string",
										Description: "file id in file Registry",
									},
									"external_link": {
										Type:        "string",
										Description: "link to file in another system",
									},
								},
							},
						},
					},
				},
			},
		},
		Params: &script.FunctionParams{
			Type: BlockGoSignID,
			Params: &script.SignParams{
				Type:               "",
				SignatureType:      "",
				FormsAccessibility: []script.FormAccessibility{},
			},
		},
		Sockets: []script.Socket{
			script.SignedSocket,
			script.RejectedSocket,
			script.ErrorSocket,
		},
	}
}

func (gb *GoSignBlock) BlockAttachments() (ids []string) {
	ids = make([]string, 0)

	for i := range gb.State.SignLog {
		for _, a := range gb.State.SignLog[i].Attachments {
			ids = append(ids, a.FileID)
		}
	}

	return utils.UniqueStrings(ids)
}

func (gb *GoSignBlock) loadState(raw json.RawMessage) error {
	return json.Unmarshal(raw, &gb.State)
}

// nolint:dupl,unparam // another block
func createGoSignBlock(ctx context.Context, name string, ef *entity.EriusFunc, runCtx *BlockRunContext,
	expectedEvents map[string]struct{},
) (*GoSignBlock, bool, error) {
	if ef.ShortTitle == "" {
		return nil, false, errors.New(ef.Title + " block short title is empty")
	}

	b := &GoSignBlock{
		Name:       name,
		ShortName:  ef.ShortTitle,
		Title:      ef.Title,
		Input:      map[string]string{},
		Output:     map[string]string{},
		Sockets:    entity.ConvertSocket(ef.Sockets),
		RunContext: runCtx,

		expectedEvents: expectedEvents,
		happenedEvents: make([]entity.NodeEvent, 0),
	}

	for _, v := range ef.Input {
		b.Input[v.Name] = v.Global
	}

	if ef.Output != nil {
		//nolint:gocritic //в этом проекте не принято использовать поинтеры в коллекциях
		for propertyName, v := range ef.Output.Properties {
			b.Output[propertyName] = v.Global
		}
	}

	reEntry := runCtx.UpdateData == nil

	rawState, ok := runCtx.VarStore.State[name]

	if ok && !reEntry {
		if err := b.loadState(rawState); err != nil {
			return nil, false, err
		}

		return b, reEntry, nil
	}

	if err := b.createState(ctx, ef); err != nil {
		return nil, false, err
	}

	b.RunContext.VarStore.AddStep(b.Name)

	b.State.Reentered = reEntry && ok

	_, ok = b.expectedEvents[eventStart]
	if ok {
		err := b.makeExpectedEvents(ctx, runCtx, name, ef)
		if err != nil {
			return nil, false, err
		}
	}

	return b, reEntry, nil
}

func (gb *GoSignBlock) makeExpectedEvents(ctx context.Context, runCtx *BlockRunContext, name string, ef *entity.EriusFunc) error {
	status, _, _ := gb.GetTaskHumanStatus()

	event, err := runCtx.MakeNodeStartEvent(
		ctx,
		MakeNodeStartEventArgs{
			NodeName:      name,
			NodeShortName: ef.ShortTitle,
			HumanStatus:   status,
			NodeStatus:    gb.GetStatus(),
		},
	)
	if err != nil {
		return err
	}

	gb.happenedEvents = append(gb.happenedEvents, event)

	return nil
}

func (gb *GoSignBlock) getUsersNotToNotifySet() map[string]struct{} {
	usersNotToNotify := make(map[string]struct{})

	for i := range gb.State.SignLog {
		if gb.State.SignLog[i].LogType == SignerLogDecision {
			usersNotToNotify[gb.State.SignLog[i].Login] = struct{}{}
		}
	}

	return usersNotToNotify
}

func (gb *GoSignBlock) getNotificationImages(descriptions []orderedmap.OrderedMap) []string {
	images := make([]string, 0)

	for i := range descriptions {
		links, link := descriptions[i].Get("attachLinks")
		if link {
			attachFiles, ok := links.([]file_registry.AttachInfo)
			if ok && len(attachFiles) != 0 {
				images = append(images, downloadImg)

				break
			}
		}
	}

	return images
}
