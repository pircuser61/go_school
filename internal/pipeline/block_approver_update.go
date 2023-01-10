package pipeline

import (
	c "context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/abp/mail/pkg/email"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	human_tasks "gitlab.services.mts.ru/jocasta/pipeliner/internal/human-tasks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

type approverUpdateEditingParams struct {
	Comment     string   `json:"comment"`
	Attachments []string `json:"attachments"`
}

type approverUpdateParams struct {
	Decision         ApproverAction `json:"decision"`
	Comment          string         `json:"comment"`
	Attachments      []string       `json:"attachments"`
	internalDecision ApproverDecision
}

type additionalApproverUpdateParams struct {
	Decision    ApproverDecision `json:"decision"`
	Comment     string           `json:"comment"`
	Attachments []string         `json:"attachments"`
}

type requestInfoParams struct {
	Type        AdditionalInfoType `json:"type"`
	Comment     string             `json:"comment"`
	Attachments []string           `json:"attachments"`
	LinkId      *string            `json:"link_id,omitempty"`
}

type addApproversParams struct {
	AdditionalApproversLogins []string `json:"additionalApprovers"`
	Question                  string   `json:"question"`
	Attachments               []string `json:"attachments"`
}

func (a *additionalApproverUpdateParams) Validate() error {
	if a.Decision != ApproverDecisionApproved && a.Decision != ApproverDecisionRejected {
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

func (gb *GoApproverBlock) setApproverDecision(u approverUpdateParams, delegations human_tasks.Delegations) error {
	if errUpdate := gb.State.SetDecision(gb.RunContext.UpdateData.ByLogin, u.internalDecision,
		u.Comment, u.Attachments, delegations); errUpdate != nil {
		return errUpdate
	}

	if gb.State.Decision != nil {
		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputApprover], &gb.State.ActualApprover)
		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputDecision], gb.State.Decision.String())
		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputComment], gb.State.Comment)
	}

	return nil
}

//nolint:dupl,gocyclo //its not duplicate
func (gb *GoApproverBlock) handleBreachedSLA(ctx c.Context) error {
	const funcName = "pipeline.approver.handleBreachedSLA"

	if !gb.State.CheckSLA {
		gb.State.SLAChecked = true
		gb.State.HalfSLAChecked = true
		return nil
	}

	log := logger.GetLogger(ctx)

	if gb.State.SLA >= 1 {
		seenAdditionalApprovers := map[string]bool{}
		emails := make([]string, 0, len(gb.State.Approvers)+len(gb.State.AdditionalApprovers))
		approversLogins := make([]string, 0, len(gb.State.Approvers))

		for approverLogin := range gb.State.Approvers {
			approverEmail, err := gb.RunContext.People.GetUserEmail(ctx, approverLogin)
			if err != nil {
				log.Warning(funcName, fmt.Sprintf("approver login %s not found", approverLogin))
				continue
			}
			emails = append(emails, approverEmail)
			approversLogins = append(approversLogins, approverLogin)
		}

		delegations, err := gb.RunContext.HumanTasks.GetDelegationsByLogins(ctx, approversLogins)
		if err != nil {
			log.Info(funcName, fmt.Sprintf("approvers %v have no delegates", approversLogins))
		}

		var delegationEmail string
		for i := range delegations {
			delegationEmail, err = gb.RunContext.People.GetUserEmail(ctx, delegations[i].ToLogin)
			if err != nil {
				log.Warning(funcName, fmt.Sprintf("delegation login %s not found", delegations[i].ToLogin))
				continue
			}
			emails = append(emails, delegationEmail)
		}

		for _, additionalApprover := range gb.State.AdditionalApprovers {
			// check if approver has not decisioned, and we did not see approver before
			if additionalApprover.Decision != nil || seenAdditionalApprovers[additionalApprover.ApproverLogin] {
				continue
			}
			seenAdditionalApprovers[additionalApprover.ApproverLogin] = true
			userEmail, err := gb.RunContext.People.GetUserEmail(ctx, additionalApprover.ApproverLogin)
			if err != nil {
				log.Warning(funcName, fmt.Sprintf("additionalApprover login %s not found", additionalApprover.ApproverLogin))
				continue
			}
			emails = append(emails, userEmail)
		}
		if len(emails) == 0 {
			return nil
		}
		err = gb.RunContext.Sender.SendNotification(ctx, emails, nil,
			mail.NewApprovementSLATemplate(
				gb.RunContext.WorkNumber,
				gb.RunContext.WorkTitle,
				gb.RunContext.Sender.SdAddress,
				gb.State.ApproveStatusName,
			),
		)
		if err != nil {
			return err
		}
	}
	if gb.State.AutoAction != nil {
		gb.RunContext.UpdateData.ByLogin = AutoApprover
		if setErr := gb.setApproverDecision(
			approverUpdateParams{
				internalDecision: (*gb.State.AutoAction).ToDecision(),
				Comment:          AutoActionComment,
			}, []human_tasks.Delegation{}); setErr != nil {
			return setErr
		}
	}

	gb.State.SLAChecked = true
	gb.State.HalfSLAChecked = true
	return nil
}

//nolint:dupl //its not duplicate
func (gb *GoApproverBlock) handleHalfBreachedSLA(ctx c.Context) error {
	const funcName = "pipeline.approver.handleHalfBreachedSLA"

	if !gb.State.CheckSLA {
		gb.State.SLAChecked = true
		gb.State.HalfSLAChecked = true
		return nil
	}

	log := logger.GetLogger(ctx)

	if gb.State.SLA >= 1 {
		seenAdditionalApprovers := map[string]bool{}
		emails := make([]string, 0, len(gb.State.Approvers)+len(gb.State.AdditionalApprovers))
		approversLogins := make([]string, 0, len(gb.State.Approvers))

		for approverLogin := range gb.State.Approvers {
			em, err := gb.RunContext.People.GetUserEmail(ctx, approverLogin)
			if err != nil {
				continue
			}
			emails = append(emails, em)
			approversLogins = append(approversLogins, approverLogin)
		}

		delegations, err := gb.RunContext.HumanTasks.GetDelegationsByLogins(ctx, approversLogins)
		if err != nil {
			log.Info(funcName, fmt.Sprintf("approvers %v have no delegates", approversLogins))
		}

		var delegationEmail string
		for i := range delegations {
			delegationEmail, err = gb.RunContext.People.GetUserEmail(ctx, delegations[i].ToLogin)
			if err != nil {
				log.Warning(funcName, fmt.Sprintf("delegation login %s not found", delegations[i].ToLogin))
				continue
			}
			emails = append(emails, delegationEmail)
		}

		for _, additionalApprover := range gb.State.AdditionalApprovers {
			// check if approver has not decisioned, and we did not see approver before
			if additionalApprover.Decision != nil || seenAdditionalApprovers[additionalApprover.ApproverLogin] {
				continue
			}
			seenAdditionalApprovers[additionalApprover.ApproverLogin] = true
			userEmail, err := gb.RunContext.People.GetUserEmail(ctx, additionalApprover.ApproverLogin)
			if err != nil {
				continue
			}
			emails = append(emails, userEmail)
		}
		if len(emails) == 0 {
			return nil
		}
		err = gb.RunContext.Sender.SendNotification(ctx, emails, nil,
			mail.NewApprovementHalfSLATemplate(
				gb.RunContext.WorkNumber,
				gb.RunContext.WorkTitle,
				gb.RunContext.Sender.SdAddress,
				gb.State.ApproveStatusName,
			),
		)
		if err != nil {
			return err
		}
	}

	gb.State.HalfSLAChecked = true
	return nil
}

//nolint:gocyclo //its ok here
func (gb *GoApproverBlock) setEditApplication(ctx c.Context, updateParams approverUpdateEditingParams,
	delegations human_tasks.Delegations) error {
	errSet := gb.State.setEditApp(gb.RunContext.UpdateData.ByLogin, updateParams, delegations)
	if errSet != nil {
		return errSet
	}

	initiatorEmail, emailErr := gb.RunContext.People.GetUserEmail(ctx, gb.RunContext.Initiator)
	if emailErr != nil {
		return emailErr
	}

	tpl := mail.NewAnswerSendToEditTemplate(gb.RunContext.WorkNumber, gb.RunContext.WorkTitle, gb.RunContext.Sender.SdAddress)
	err := gb.RunContext.Sender.SendNotification(ctx, []string{initiatorEmail}, nil, tpl)
	if err != nil {
		return err
	}

	return nil
}

//nolint:gocyclo //ok
func (gb *GoApproverBlock) updateRequestApproverInfo(ctx c.Context, delegations human_tasks.Delegations) (err error) {
	var updateParams requestInfoParams
	if err = json.Unmarshal(gb.RunContext.UpdateData.Parameters, &updateParams); err != nil {
		return errors.New("can't assert provided update requestApproverInfo data")
	}

	if gb.State.Decision != nil {
		return errors.New("decision already set")
	}

	var (
		id     = uuid.NewString()
		linkId *string
	)

	if updateParams.Type == RequestAddInfoType {
		var delegateFor = delegations.DelegateFor(gb.RunContext.UpdateData.ByLogin)
		if !gb.State.userIsAnyApprover(gb.RunContext.UpdateData.ByLogin, delegateFor) {
			return fmt.Errorf("%s not found in approvers or delegates", gb.RunContext.UpdateData.ByLogin)
		}

		authorEmail, emailErr := gb.RunContext.People.GetUserEmail(ctx, gb.RunContext.Initiator)
		if emailErr != nil {
			return emailErr
		}

		tpl := mail.NewRequestApproverInfoTemplate(gb.RunContext.WorkNumber, gb.RunContext.WorkTitle, gb.RunContext.Sender.SdAddress)
		if sendErr := gb.RunContext.Sender.SendNotification(ctx, []string{authorEmail}, nil, tpl); sendErr != nil {
			return sendErr
		}
	}

	if updateParams.Type == ReplyAddInfoType {
		if len(gb.State.AddInfo) == 0 {
			return errors.New("don't answer after request")
		}

		if updateParams.LinkId == nil {
			return errors.New("linkId is null when reply")
		}

		linkId = updateParams.LinkId
		linkErr := setLinkIdRequest(id, *updateParams.LinkId, gb.State.AddInfo)
		if linkErr != nil {
			return linkErr
		}

		if len(gb.State.RequestApproverInfoLog) > 0 {
			workHours := getWorkWorkHoursBetweenDates(
				gb.State.RequestApproverInfoLog[len(gb.State.RequestApproverInfoLog)-1].CreatedAt,
				time.Now(),
			)
			gb.State.IncreaseSLA(workHours)
		}

		tpl := mail.NewAnswerApproverInfoTemplate(gb.RunContext.WorkNumber, gb.RunContext.WorkTitle, gb.RunContext.Sender.SdAddress)

		approverEmail, emailErr := gb.RunContext.People.GetUserEmail(ctx, gb.RunContext.UpdateData.ByLogin)
		if emailErr != nil {
			return emailErr
		}

		err = gb.RunContext.Sender.SendNotification(ctx, []string{approverEmail}, nil, tpl)
		if err != nil {
			return err
		}
	}

	gb.State.AddInfo = append(gb.State.AddInfo, AdditionalInfo{
		Id:          id,
		Type:        updateParams.Type,
		Comment:     updateParams.Comment,
		Attachments: updateParams.Attachments,
		LinkId:      linkId,
		Login:       gb.RunContext.UpdateData.ByLogin,
		CreatedAt:   time.Now(),
	})

	return nil
}

func setLinkIdRequest(replyId, linkId string, addInfo []AdditionalInfo) error {
	for i := range addInfo {
		if addInfo[i].Id == linkId {
			addInfo[i].LinkId = &replyId
			return nil
		}
	}

	return errors.New("not found request by linkId")
}

func (gb *GoApproverBlock) actionAcceptable(action ApproverAction) bool {
	for _, a := range gb.State.ActionList {
		if a.Id == string(action) {
			return true
		}
	}
	return false
}

//nolint:gocyclo //its ok here
func (gb *GoApproverBlock) Update(ctx c.Context) (interface{}, error) {
	data := gb.RunContext.UpdateData
	if data == nil {
		return nil, errors.New("empty data")
	}

	delegates := getDelegates(gb.RunContext.VarStore)

	switch data.Action {
	case string(entity.TaskUpdateActionSLABreach):
		if errUpdate := gb.handleBreachedSLA(ctx); errUpdate != nil {
			return nil, errUpdate
		}
	case string(entity.TaskUpdateActionHalfSLABreach):
		if errUpdate := gb.handleHalfBreachedSLA(ctx); errUpdate != nil {
			return nil, errUpdate
		}

	case string(entity.TaskUpdateActionApprovement):
		var updateParams approverUpdateParams

		if err := json.Unmarshal(data.Parameters, &updateParams); err != nil {
			return nil, errors.New("can't assert provided data")
		}

		if !gb.actionAcceptable(updateParams.Decision) {
			return nil, errors.New("unacceptable action")
		}

		updateParams.internalDecision = updateParams.Decision.ToDecision()

		if errUpdate := gb.setApproverDecision(updateParams, delegates); errUpdate != nil {
			return nil, errUpdate
		}

	case string(entity.TaskUpdateActionAdditionalApprovement):
		var updateParams additionalApproverUpdateParams

		if err := json.Unmarshal(data.Parameters, &updateParams); err != nil {
			return nil, errors.New("can't assert provided data")
		}

		if err := updateParams.Validate(); err != nil {
			return nil, err
		}

		loginsToNotify, err := gb.State.SetDecisionByAdditionalApprover(gb.RunContext.UpdateData.ByLogin, updateParams, delegates)
		if err != nil {
			return nil, err
		}

		loginsToNotify = append(loginsToNotify, gb.RunContext.Initiator)
		err = gb.notificateDecisionMadeByAdditionalApprover(ctx, loginsToNotify)
		if err != nil {
			return nil, err
		}

	case string(entity.TaskUpdateActionApproverSendEditApp):
		var updateParams approverUpdateEditingParams

		if err := json.Unmarshal(data.Parameters, &updateParams); err != nil {
			return nil, errors.New("can't assert provided data")
		}
		if errUpdate := gb.setEditApplication(ctx, updateParams, delegates); errUpdate != nil {
			return nil, errUpdate
		}

	case string(entity.TaskUpdateActionRequestApproveInfo):
		if errUpdate := gb.updateRequestApproverInfo(ctx, delegates); errUpdate != nil {
			return nil, errUpdate
		}

	case string(entity.TaskUpdateActionCancelApp):
		if errUpdate := gb.cancelPipeline(ctx, delegates); errUpdate != nil {
			return nil, errUpdate
		}

	case string(entity.TaskUpdateActionAddApprovers):
		var updateParams addApproversParams
		if err := json.Unmarshal(data.Parameters, &updateParams); err != nil {
			return nil, errors.New("can't assert provided data")
		}
		if errUpdate := gb.addApprovers(ctx, updateParams, delegates); errUpdate != nil {
			return nil, errUpdate
		}
	}

	var stateBytes []byte
	stateBytes, err := json.Marshal(gb.State)
	if err != nil {
		return nil, err
	}

	gb.RunContext.VarStore.ReplaceState(gb.Name, stateBytes)

	return nil, nil
}

// nolint:dupl // another action
func (gb *GoApproverBlock) cancelPipeline(ctx c.Context, delegations human_tasks.Delegations) error {
	var currentLogin = gb.RunContext.UpdateData.ByLogin
	var initiator = gb.RunContext.Initiator

	var delegateFor = delegations.DelegateFor(currentLogin)

	if currentLogin != initiator && delegateFor != "" {
		return fmt.Errorf("%s is not an initiator or delegate", currentLogin)
	}

	gb.State.IsRevoked = true
	if stopErr := gb.RunContext.Storage.StopTaskBlocks(ctx, gb.RunContext.TaskID); stopErr != nil {
		return stopErr
	}
	if stopErr := gb.RunContext.updateTaskStatus(ctx, db.RunStatusFinished); stopErr != nil {
		return stopErr
	}
	return nil
}

func (gb *GoApproverBlock) addApprovers(ctx c.Context, u addApproversParams, delegations human_tasks.Delegations) error {
	logApprovers := []string{}

	var delegateFor = delegations.DelegateFor(gb.RunContext.UpdateData.ByLogin)
	if !gb.State.userIsAnyApprover(gb.RunContext.UpdateData.ByLogin, delegateFor) {
		return fmt.Errorf("%s not found in approvers or delegates", gb.RunContext.UpdateData.ByLogin)
	}

	crTime := time.Now()

	delegatesLogins, err := gb.RunContext.HumanTasks.GetDelegationsByLogins(ctx, u.AdditionalApproversLogins)
	if err != nil {
		return err
	}

	delegations.Append(delegatesLogins)
	gb.RunContext.VarStore.SetValue(script.DelegationsCollection, delegations)

	for i := range u.AdditionalApproversLogins {
		if gb.checkAdditionalApproverNotAdded(u.AdditionalApproversLogins[i]) {
			gb.State.AdditionalApprovers = append(gb.State.AdditionalApprovers,
				AdditionalApprover{
					ApproverLogin:     u.AdditionalApproversLogins[i],
					BaseApproverLogin: gb.RunContext.UpdateData.ByLogin,
					Question:          &u.Question,
					Attachments:       u.Attachments,
					CreatedAt:         crTime,
				})
			logApprovers = append(logApprovers, u.AdditionalApproversLogins[i])
		}
	}
	if len(logApprovers) > 0 {
		var approverLogEntry = ApproverLogEntry{
			Login:          gb.RunContext.UpdateData.ByLogin,
			Decision:       "",
			Comment:        u.Question,
			Attachments:    u.Attachments,
			CreatedAt:      crTime,
			AddedApprovers: u.AdditionalApproversLogins,
			LogType:        ApproverLogAddApprover,
			DelegateFor:    delegateFor,
		}
		gb.State.ApproverLog = append(gb.State.ApproverLog, approverLogEntry)
		err := gb.notificateAdditionalApprovers(ctx, logApprovers, u.Attachments)
		if err != nil {
			return err
		}
	}
	return nil
}

func (gb *GoApproverBlock) checkAdditionalApproverNotAdded(login string) bool {
	for _, added := range gb.State.AdditionalApprovers {
		if login == added.ApproverLogin &&
			added.BaseApproverLogin == gb.RunContext.UpdateData.ByLogin &&
			added.Decision == nil {
			return false
		}
	}
	return true
}

func (gb *GoApproverBlock) notificateAdditionalApprovers(ctx c.Context, logins, attachmentsId []string) error {
	approverEmails := []string{}
	for _, approver := range logins {
		approverEmail, emailErr := gb.RunContext.People.GetUserEmail(ctx, approver)
		if emailErr != nil {
			return emailErr
		}
		approverEmails = append(approverEmails, approverEmail)
	}
	tpl := mail.NewAddApproversTemplate(
		gb.RunContext.WorkNumber,
		gb.RunContext.WorkTitle,
		gb.RunContext.Sender.SdAddress,
		gb.State.ApproveStatusName,
	)

	attachmentFiles, err := gb.RunContext.ServiceDesc.GetAttachments(ctx, map[string][]string{"Ids": attachmentsId})
	if err != nil {
		return err
	}
	files := make([]email.Attachment, 0)
	for k := range attachmentFiles {
		files = append(files, attachmentFiles[k]...)
	}
	err = gb.RunContext.Sender.SendNotification(ctx, approverEmails, files, tpl)
	if err != nil {
		return err
	}
	return nil
}

// notificateDecisionMadeByAdditionalApprover notifies requesting approvers
// and the task initiator that an additional approver has left a review
func (gb *GoApproverBlock) notificateDecisionMadeByAdditionalApprover(ctx c.Context, loginsToNotify []string) error {
	emailsToNotify := make([]string, 0, len(loginsToNotify))
	for _, login := range loginsToNotify {
		emailToNotify, emailErr := gb.RunContext.People.GetUserEmail(ctx, login)
		if emailErr != nil {
			return emailErr
		}
		emailsToNotify = append(emailsToNotify, emailToNotify)
	}

	user, err := gb.RunContext.People.GetUser(ctx, gb.RunContext.UpdateData.ByLogin)
	if err != nil {
		return err
	}

	userInfo, err := user.ToUserinfo()
	if err != nil {
		return err
	}

	latestDecisonLog := gb.State.ApproverLog[len(gb.State.ApproverLog)-1]

	tpl := mail.NewDecisionMadeByAdditionalApproverTemplate(
		gb.RunContext.WorkNumber,
		userInfo.FullName,
		latestDecisonLog.Decision.ToRuString(),
		latestDecisonLog.Comment,
		gb.RunContext.Sender.SdAddress,
	)

	attachmentFiles, err := gb.RunContext.ServiceDesc.GetAttachments(ctx, map[string][]string{"Ids": latestDecisonLog.Attachments})
	if err != nil {
		return err
	}

	files := make([]email.Attachment, 0)
	for k := range attachmentFiles {
		files = append(files, attachmentFiles[k]...)
	}

	err = gb.RunContext.Sender.SendNotification(ctx, emailsToNotify, files, tpl)
	if err != nil {
		return err
	}

	return nil
}
