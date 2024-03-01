package pipeline

import (
	c "context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/scheduler"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

const (
	changeWorkStatusTimeout = 20 * time.Minute
)

type signSignatureParams struct {
	Decision    SignDecision        `json:"decision"`
	Comment     string              `json:"comment,omitempty"`
	Attachments []entity.Attachment `json:"attachments"`
	Signatures  []fileSignature     `json:"signatures"`
	Username    string              `json:"username"`
}

type fileSignature struct {
	FileID          string `json:"file_id"`
	SignatureFileID string `json:"signature_file_id"`
}

type fileSignaturePair struct {
	File          entity.Attachment `json:"file"`
	SignatureFile entity.Attachment `json:"signature_file"`
}

type additionalApproverSignUpdateParams struct {
	Decision    SignDecision        `json:"decision"`
	Comment     string              `json:"comment"`
	Attachments []entity.Attachment `json:"attachments"`
}

func (a *additionalApproverSignUpdateParams) Validate() error {
	if a.Decision != SignDecisionAddApproverApproved && a.Decision != SignDecisionRejected {
		return fmt.Errorf("unknown decision %s", a.Decision)
	}

	if len(a.Attachments) > 10 {
		return fmt.Errorf("max attachments length: 10, current: %d", len(a.Attachments))
	}

	if len([]rune(a.Comment)) > 500 {
		return fmt.Errorf("max comment length 500 symbols, current: %d", len([]rune(a.Comment)))
	}

	return nil
}

type changeStatusSignatureParams struct {
	Status string `json:"status"`
}

func (gb *GoSignBlock) handleSignature(ctx c.Context, login string) error {
	log := logger.GetLogger(ctx)

	updateParams := &signSignatureParams{}

	err := json.Unmarshal(gb.RunContext.UpdateData.Parameters, updateParams)
	if err != nil {
		return errors.New("can't assert provided update data")
	}

	if updateParams.Decision != SignDecisionRejected {
		for _, v := range gb.State.FormsAccessibility {
			if v.AccessType != requiredFillAccessType {
				continue
			}

			form, _ := gb.RunContext.VarStore.State[v.NodeID]

			var formData FormData
			if unmarshalErr := json.Unmarshal(form, &formData); unmarshalErr != nil {
				return unmarshalErr
			}

			if !formData.IsFilled {
				return errors.New(fmt.Sprintf("%s is not filled", v.NodeID))
			}
		}
	}

	for _, v := range updateParams.Signatures {
		newPair := fileSignaturePair{
			File: entity.Attachment{
				FileID:       v.FileID,
				ExternalLink: "",
			},
			SignatureFile: entity.Attachment{
				FileID:       v.SignatureFileID,
				ExternalLink: "",
			},
		}
		gb.State.Signatures = append(gb.State.Signatures, newPair)
	}

	if gb.State.SignatureType == script.SignatureTypeUKEP &&
		updateParams.Decision != SignDecisionRejected {
		if !gb.State.IsTakenInWork {
			if updateParams.Username == "" {
				return errors.New("is not taken in work")
			}

			log.Info("setting signature with no 'taken in work'")
		}

		if !gb.isValidLogin(login) {
			return NewUserIsNotPartOfProcessErr()
		}
	}

	if !gb.isValidSigner(login) {
		return NewUserIsNotPartOfProcessErr()
	}

	if setErr := gb.setSignerDecision(ctx, updateParams); setErr != nil {
		return setErr
	}

	if updateParams.Decision == SignDecisionError {
		emails := make([]string, 0, len(gb.State.Signers))
		logins := getSliceFromMapOfStrings(gb.State.Signers)

		for i := range logins {
			eml, err := gb.RunContext.Services.People.GetUserEmail(ctx, logins[i])
			if err != nil {
				log.WithField("login", login).WithError(err).Warning("couldn't get email")

				continue
			}

			emails = append(emails, eml)
		}

		tpl := mail.NewSignErrorTemplate(
			gb.RunContext.WorkNumber,
			gb.RunContext.WorkTitle,
			gb.RunContext.Services.Sender.SdAddress,
		)

		filesList := []string{tpl.Image}

		files, iconEerr := gb.RunContext.GetIcons(filesList)
		if iconEerr != nil {
			return iconEerr
		}

		sendErr := gb.RunContext.Services.Sender.SendNotification(ctx, emails, files, tpl)
		if sendErr != nil {
			return sendErr
		}
	}

	return nil
}

func (gb *GoSignBlock) Update(ctx c.Context) (interface{}, error) {
	data := gb.RunContext.UpdateData
	if data == nil {
		return nil, errors.New("empty data")
	}

	//nolint:gocritic //for future actions
	switch data.Action {
	case string(entity.TaskUpdateActionSLABreach):
		if errUpdate := gb.handleBreachedSLA(ctx); errUpdate != nil {
			return nil, errUpdate
		}
	case string(entity.TaskUpdateActionDayBeforeSLABreach):
		if errUpdate := gb.handleDayBeforeSLANotifications(ctx); errUpdate != nil {
			return nil, errUpdate
		}
	case string(entity.TaskUpdateActionSign):
		if errUpdate := gb.handleSignature(ctx, data.ByLogin); errUpdate != nil {
			return nil, errUpdate
		}
	case string(entity.TaskUpdateActionAdditionalApprovement):
		if errUpdate := gb.SetDecisionByAdditionalApprover(ctx, data.ByLogin); errUpdate != nil {
			return nil, errUpdate
		}
	case string(entity.TaskUpdateActionAddApprovers):
		if errUpdate := gb.addApprovers(ctx, data.ByLogin); errUpdate != nil {
			return nil, errUpdate
		}
	case string(entity.TaskUpdateActionSignChangeWorkStatus):
		if errUpdate := gb.handleChangeWorkStatus(ctx, data.ByLogin); errUpdate != nil {
			return nil, errUpdate
		}
	}

	var stateBytes []byte

	stateBytes, err := json.Marshal(gb.State)
	if err != nil {
		return nil, err
	}

	gb.RunContext.VarStore.ReplaceState(gb.Name, stateBytes)

	if _, ok := gb.expectedEvents[eventEnd]; ok {
		status, _, _ := gb.GetTaskHumanStatus()

		event, eventErr := gb.RunContext.MakeNodeEndEvent(ctx, MakeNodeEndEventArgs{
			NodeName:      gb.Name,
			NodeShortName: gb.ShortName,
			HumanStatus:   status,
			NodeStatus:    gb.GetStatus(),
		})
		if eventErr != nil {
			return nil, eventErr
		}

		gb.happenedEvents = append(gb.happenedEvents, event)
	}

	return nil, nil
}

func (gb *GoSignBlock) addApprovers(ctx c.Context, login string) error {
	if !gb.State.userIsSignerOrAddApprover(login) {
		return NewUserIsNotPartOfProcessErr()
	}

	var updateParams addApproversParams
	if err := json.Unmarshal(gb.RunContext.UpdateData.Parameters, &updateParams); err != nil {
		return errors.New("can't assert provided data")
	}

	var logAddApprovers []string

	crTime := time.Now()

	for _, additionalApproverLogin := range updateParams.AdditionalApproversLogins {
		if gb.checkAdditionalApproverNotAdded(additionalApproverLogin) {
			gb.State.AdditionalApprovers = append(gb.State.AdditionalApprovers,
				AdditionalSignApprover{
					ApproverLogin: additionalApproverLogin,
					BaseLogin:     login,
					Question:      &updateParams.Question,
					Attachments:   updateParams.Attachments,
					CreatedAt:     crTime,
				})

			logAddApprovers = append(logAddApprovers, additionalApproverLogin)
		}
	}

	if len(logAddApprovers) > 0 {
		signerLogEntry := SignLogEntry{
			Login:          login,
			Decision:       "",
			Comment:        updateParams.Question,
			Attachments:    updateParams.Attachments,
			CreatedAt:      crTime,
			AddedApprovers: updateParams.AdditionalApproversLogins,
			LogType:        SignerLogAddApprover,
		}

		gb.State.SignLog = append(gb.State.SignLog, signerLogEntry)

		err := gb.notifyAdditionalApprovers(ctx, logAddApprovers, updateParams.Attachments)
		if err != nil {
			return err
		}
	}

	return nil
}

func (gb *GoSignBlock) checkAdditionalApproverNotAdded(login string) bool {
	for _, added := range gb.State.AdditionalApprovers {
		if login == added.ApproverLogin &&
			added.BaseLogin == gb.RunContext.UpdateData.ByLogin &&
			added.Decision == nil {
			return false
		}
	}

	return true
}

//nolint:dupl,gocyclo //its not duplicate
func (gb *GoSignBlock) handleBreachedSLA(ctx c.Context) error {
	if gb.State.CheckSLA == nil || !*gb.State.CheckSLA {
		gb.State.SLAChecked = true

		return nil
	}

	if gb.State.SLAChecked {
		return nil
	}

	if gb.State.AutoReject != nil && *gb.State.AutoReject {
		gb.RunContext.UpdateData.ByLogin = autoSigner
		gb.State.ActualSigner = &gb.RunContext.UpdateData.ByLogin

		if setErr := gb.setSignerDecision(ctx, &signSignatureParams{
			Decision: SignDecisionRejected,
			Comment:  AutoActionComment,
		}); setErr != nil {
			return setErr
		}
	}

	gb.State.SLAChecked = true

	if gb.State.SLA != nil {
		emails := make([]string, 0, len(gb.State.Signers))
		logins := getSliceFromMapOfStrings(gb.State.Signers)

		for i := range logins {
			eml, err := gb.RunContext.Services.People.GetUserEmail(ctx, logins[i])
			if err != nil {
				continue
			}

			emails = append(emails, eml)
		}

		tpl := mail.NewSignSLAExpiredTemplate(
			gb.RunContext.WorkNumber,
			gb.RunContext.NotifName,
			gb.RunContext.Services.Sender.SdAddress,
		)

		filesList := []string{tpl.Image}

		files, iconEerr := gb.RunContext.GetIcons(filesList)
		if iconEerr != nil {
			return iconEerr
		}

		if err := gb.RunContext.Services.Sender.SendNotification(ctx, emails, files, tpl); err != nil {
			return err
		}
	}

	return nil
}

func (gb *GoSignBlock) SetDecisionByAdditionalApprover(ctx c.Context, login string) error {
	var updateParams additionalApproverSignUpdateParams

	if err := json.Unmarshal(gb.RunContext.UpdateData.Parameters, &updateParams); err != nil {
		return fmt.Errorf("can't assert provided data: %v", err)
	}

	if err := updateParams.Validate(); err != nil {
		return err
	}

	loginsToNotify, err := gb.State.SetDecisionByAdditionalApprover(login, updateParams)
	if err != nil {
		return err
	}

	loginsToNotify = append(loginsToNotify, gb.RunContext.Initiator)

	err = gb.notifyDecisionMadeByAdditionalApprover(ctx, loginsToNotify)
	if err != nil {
		return err
	}

	return nil
}

func (gb *GoSignBlock) handleChangeWorkStatus(ctx c.Context, login string) error {
	log := logger.GetLogger(ctx)

	status := &changeStatusSignatureParams{Status: "end"}

	if gb.RunContext.UpdateData.Parameters != nil {
		err := json.Unmarshal(gb.RunContext.UpdateData.Parameters, status)
		if err != nil {
			return errors.New("can't assert provided update data")
		}
	}

	if gb.State.IsTakenInWork && !gb.isValidLogin(login) {
		return NewUserIsNotPartOfProcessErr()
	}

	switch {
	case !gb.State.IsTakenInWork && status.Status == "start":
		if !gb.isValidSigner(login) {
			return NewUserIsNotPartOfProcessErr()
		}

		gb.State.IsTakenInWork = true
		gb.State.WorkerLogin = login

	case gb.State.IsTakenInWork && status.Status == "end":
		gb.State.IsTakenInWork = false
		gb.State.WorkerLogin = ""

		// delete those that may exist
		err := gb.RunContext.Services.Scheduler.DeleteTask(ctx,
			&scheduler.DeleteTask{
				WorkID:   gb.RunContext.TaskID.String(),
				StepName: gb.Name,
			})
		if err != nil {
			log.WithError(err).Error("cannot delete signChangeWorkStatus timer")

			return err
		}

		return nil
	default:
		return nil
	}

	_, err := gb.RunContext.Services.Scheduler.CreateTask(ctx, &scheduler.CreateTask{
		WorkNumber:  gb.RunContext.WorkNumber,
		WorkID:      gb.RunContext.TaskID.String(),
		ActionName:  string(entity.TaskUpdateActionSignChangeWorkStatus),
		StepName:    gb.Name,
		WaitSeconds: int(changeWorkStatusTimeout.Seconds()),
	})
	if err != nil {
		log.WithError(err).Error("cannot create signChangeWorkStatus timer")

		return err
	}

	return nil
}

func (gb *GoSignBlock) setSignerDecision(ctx c.Context, u *signSignatureParams) error {
	login := gb.RunContext.UpdateData.ByLogin
	if u.Username != "" {
		login = u.Username
	}

	if errUpdate := gb.State.SetDecision(login, u); errUpdate != nil {
		return errUpdate
	}

	if gb.State.Decision != nil {
		personData, err := gb.RunContext.Services.ServiceDesc.GetSsoPerson(ctx, *gb.State.ActualSigner)
		if err != nil {
			return err
		}

		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputSigner], personData)
		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputSignDecision], gb.State.Decision)
		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputSignComment], gb.State.Comment)
		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputSignatures], gb.State.Signatures)

		resAttachments := make([]entity.Attachment, 0)

		//nolint:gocritic //в этом проекте не принято использовать поинтеры в коллекциях
		for _, l := range gb.State.SignLog {
			if l.LogType != SignerLogDecision {
				continue
			}

			resAttachments = append(resAttachments, l.Attachments...)
		}

		resAttachments = append(resAttachments, gb.State.SigningParams.Files...)

		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputSignAttachments], resAttachments)
	}

	return nil
}

func (gb *GoSignBlock) isValidSigner(login string) bool {
	if _, ok := gb.State.Signers[login]; !ok &&
		(login != ServiceAccount &&
			login != ServiceAccountStage &&
			login != ServiceAccountDev) {
		return false
	}

	return true
}

func (gb *GoSignBlock) isValidLogin(login string) bool {
	if gb.State.WorkerLogin != login &&
		(login != ServiceAccount &&
			login != ServiceAccountStage &&
			login != ServiceAccountDev) {
		return false
	}

	return true
}
