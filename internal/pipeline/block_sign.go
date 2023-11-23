package pipeline

import (
	c "context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"

	e "gitlab.services.mts.ru/abp/mail/pkg/email"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sla"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

const (
	autoSigner = "auto_signer"

	keyOutputSigner          = "signer"
	keyOutputSignDecision    = "decision"
	keyOutputSignComment     = "comment"
	keyOutputSignAttachments = "attachments"
	keyOutputSignatures      = "signatures"

	SignDecisionSigned   SignDecision = "signed"
	SignDecisionRejected SignDecision = "rejected"
	SignDecisionError    SignDecision = "error"

	signActionSign       = "sign_sign"
	signActionReject     = "sign_reject"
	signActionTakeInWork = "sign_start_work"

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

func (gb *GoSignBlock) GetNewEvents() []entity.NodeEvent {
	return gb.happenedEvents
}

func (gb *GoSignBlock) GetState() interface{} {
	return gb.State
}

func (gb *GoSignBlock) Next(_ *store.VariableStore) ([]string, bool) {
	var key string
	if gb.State != nil && gb.State.Decision != nil {
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

func (gb *GoSignBlock) GetTaskHumanStatus() (status TaskHumanStatus, comment string, action string) {
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
	for _, s := range gb.State.SignLog {
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

	for _, s := range gb.State.SignLog {
		if s.Login == login {
			return []MemberAction{}
		}
	}

	if gb.State.SignatureType == script.SignatureTypeUKEP {
		takeInWorkAction := MemberAction{
			Id:   signActionTakeInWork,
			Type: ActionTypePrimary,
			Params: map[string]interface{}{
				signatureTypeActionParamsKey:    gb.State.SignatureType,
				signatureCarrierActionParamsKey: gb.State.SignatureCarrier,
				signatureINNParamsKey:           gb.State.SigningParams.INN,
				signatureSNILSParamsKey:         gb.State.SigningParams.SNILS,
				signatureFilesParamsKey:         gb.State.SigningParams.Files,
			},
		}

		rejectAction := MemberAction{
			Id:   signActionReject,
			Type: ActionTypeSecondary,
		}

		if gb.State.IsTakenInWork && login != gb.State.WorkerLogin {
			takeInWorkAction.Params["disabled"] = true
			rejectAction.Params = map[string]interface{}{"disabled": true}
		}

		return []MemberAction{takeInWorkAction, rejectAction}
	}

	signAction := MemberAction{
		Id:   signActionSign,
		Type: ActionTypePrimary,
		Params: map[string]interface{}{
			signatureTypeActionParamsKey: gb.State.SignatureType,
		},
	}

	return []MemberAction{
		signAction,
		{
			Id:   signActionReject,
			Type: ActionTypeSecondary,
		}}
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
	return members
}

//nolint:dupl,gocyclo //Need here
func (gb *GoSignBlock) Deadlines(ctx c.Context) ([]Deadline, error) {
	deadlines := make([]Deadline, 0, 2)

	if gb.State.CheckSLA != nil && *gb.State.CheckSLA {
		slaInfoPtr, getSlaInfoErr := gb.RunContext.Services.SLAService.GetSLAInfoPtr(ctx, sla.InfoDto{
			TaskCompletionIntervals: []entity.TaskCompletionInterval{{StartedAt: gb.RunContext.CurrBlockStartTime,
				FinishedAt: gb.RunContext.CurrBlockStartTime.Add(time.Hour * 24 * 100)}},
			WorkType: sla.WorkHourType(*gb.State.WorkType),
		})

		if getSlaInfoErr != nil {
			return nil, getSlaInfoErr
		}

		deadline := gb.RunContext.Services.SLAService.ComputeMaxDate(gb.RunContext.CurrBlockStartTime, float32(*gb.State.SLA), slaInfoPtr)
		if !gb.State.SLAChecked {
			deadlines = append(deadlines,
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

type setSignersByParamsDTO struct {
	Type    script.SignerType
	GroupID string
	Signer  string
}

func (gb *GoSignBlock) setSignersByParams(ctx c.Context, dto *setSignersByParamsDTO) error {
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

		approversFromSchema := make(map[string]struct{})

		approversVars := strings.Split(dto.Signer, ";")
		for i := range approversVars {
			resolvedEntities, resolveErr := getUsersFromVars(
				variableStorage,
				map[string]struct{}{
					approversVars[i]: {},
				},
			)
			if resolveErr != nil {
				return resolveErr
			}

			for approverLogin := range resolvedEntities {
				approversFromSchema[approverLogin] = struct{}{}
			}
		}

		gb.State.Signers = approversFromSchema
	}

	return nil
}

func (gb *GoSignBlock) handleDayBeforeSLANotifications(ctx c.Context) error {
	if gb.State.DayBeforeSLAChecked {
		return nil
	}

	if err := gb.handleNotifications(ctx); err != nil {
		return err
	}

	gb.State.DayBeforeSLAChecked = true
	return nil
}

//nolint:dupl,gocyclo // maybe later
func (gb *GoSignBlock) handleNotifications(ctx c.Context) error {
	l := logger.GetLogger(ctx)

	if gb.RunContext.skipNotifications {
		return nil
	}

	signers := getSliceFromMapOfStrings(gb.State.Signers)
	var emailAttachment []e.Attachment

	description, err := gb.RunContext.makeNotificationDescription(gb.Name)
	if err != nil {
		return err
	}

	slaDeadline := ""

	if gb.State.SLA != nil && gb.State.WorkType != nil {
		slaInfoPtr, getSlaInfoErr := gb.RunContext.Services.SLAService.GetSLAInfoPtr(ctx, sla.InfoDto{
			TaskCompletionIntervals: []entity.TaskCompletionInterval{{StartedAt: gb.RunContext.CurrBlockStartTime,
				FinishedAt: gb.RunContext.CurrBlockStartTime.Add(time.Hour * 24 * 100)}},
			WorkType: sla.WorkHourType(*gb.State.WorkType),
		})
		if getSlaInfoErr != nil {
			return getSlaInfoErr
		}
		slaDeadline = gb.RunContext.Services.SLAService.ComputeMaxDateFormatted(gb.RunContext.CurrBlockStartTime,
			*gb.State.SLA, slaInfoPtr)
	}

	var emails = make(map[string]mail.Template, 0)
	for _, login := range signers {
		em, getUserEmailErr := gb.RunContext.Services.People.GetUserEmail(ctx, login)
		if getUserEmailErr != nil {
			l.WithField("login", login).WithError(getUserEmailErr).Warning("couldn't get email")
			continue
		}

		emails[em] = mail.NewSignerNotificationTpl(
			gb.RunContext.WorkNumber,
			gb.RunContext.NotifName,
			description,
			gb.RunContext.Services.Sender.SdAddress,
			slaDeadline,
			gb.State.AutoReject != nil && *gb.State.AutoReject,
		)
	}

	if len(emails) == 0 {
		return nil
	}

	for i := range emails {
		if sendErr := gb.RunContext.Services.Sender.SendNotification(ctx, []string{i}, emailAttachment,
			emails[i]); sendErr != nil {
			return sendErr
		}
	}

	return nil
}

func lookForFileIdInObject(object map[string]interface{}) (string, error) {
	existingFileId, ok := object["file_id"]
	if !ok {
		return "", errors.New("file_id does not exist in object")
	}
	fileID, ok := existingFileId.(string)
	if !ok {
		return "", errors.New("failed to type assert path to string")
	}
	return fileID, nil
}

func ValidateFiles(file interface{}) ([]entity.Attachment, error) {
	resFiles := make([]entity.Attachment, 0)

	switch f := file.(type) {
	case map[string]interface{}:
		fileID, err := lookForFileIdInObject(f)
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
			fileID, err := lookForFileIdInObject(object)
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

//nolint:dupl,gocyclo //its not duplicate
func (gb *GoSignBlock) createState(ctx c.Context, ef *entity.EriusFunc) error {
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
		groupId := getVariable(variableStorage, params.SignerGroupIDPath)
		if groupId == nil {
			return errors.New("can't find group id in variables")
		}
		params.SignerGroupID = fmt.Sprintf("%v", groupId)
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

func (gb *GoSignBlock) checkSLA() bool {
	return gb.State.CheckSLA != nil && *gb.State.CheckSLA
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

func (gb *GoSignBlock) loadState(raw json.RawMessage) error {
	return json.Unmarshal(raw, &gb.State)
}

// nolint:dupl,unparam // another block
func createGoSignBlock(ctx c.Context, name string, ef *entity.EriusFunc, runCtx *BlockRunContext,
	expectedEvents map[string]struct{}) (*GoSignBlock, bool, error) {
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
	} else {
		if err := b.createState(ctx, ef); err != nil {
			return nil, false, err
		}
		b.RunContext.VarStore.AddStep(b.Name)

		if reEntry && ok {
			b.State.Reentered = true
		}

		if _, ok := b.expectedEvents[eventStart]; ok {
			status, _, _ := b.GetTaskHumanStatus()
			event, err := runCtx.MakeNodeStartEvent(ctx, MakeNodeStartEventArgs{
				NodeName:      name,
				NodeShortName: ef.ShortTitle,
				HumanStatus:   status,
				NodeStatus:    b.GetStatus(),
			})
			if err != nil {
				return nil, false, err
			}
			b.happenedEvents = append(b.happenedEvents, event)
		}
	}

	return b, reEntry, nil
}
