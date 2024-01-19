package pipeline

import (
	c "context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sla"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

const (
	workingHours = 8
)

type approverUpdateEditingParams struct {
	Comment     string              `json:"comment"`
	Attachments []entity.Attachment `json:"attachments"`
}

type approverUpdateParams struct {
	Decision         ApproverAction      `json:"decision"`
	Comment          string              `json:"comment"`
	Attachments      []entity.Attachment `json:"attachments"`
	Username         string              `json:"username"`
	internalDecision ApproverDecision
}

type additionalApproverUpdateParams struct {
	Decision    ApproverDecision    `json:"decision"`
	Comment     string              `json:"comment"`
	Attachments []entity.Attachment `json:"attachments"`
}

type requestInfoParams struct {
	Type        AdditionalInfoType  `json:"type"`
	Comment     string              `json:"comment"`
	Attachments []entity.Attachment `json:"attachments"`
	LinkID      *string             `json:"link_id,omitempty"`
}

type replyInfoParams struct {
	Comment     string              `json:"comment"`
	Attachments []entity.Attachment `json:"attachments"`
	LinkID      *string             `json:"link_id,omitempty"`
}

type addApproversParams struct {
	AdditionalApproversLogins []string            `json:"additionalApprovers"`
	Question                  string              `json:"question"`
	Attachments               []entity.Attachment `json:"attachments"`
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

func (gb *GoApproverBlock) setApproveDecision(ctx c.Context, u *approverUpdateParams) error {
	byLogin := gb.RunContext.UpdateData.ByLogin

	err := gb.State.SetDecision(byLogin, u.Comment, u.internalDecision, u.Attachments, gb.RunContext.Delegations)
	if err != nil {
		return err
	}

	if gb.State.Decision == nil {
		return nil
	}

	person, err := gb.RunContext.Services.ServiceDesc.GetSsoPerson(ctx, *gb.State.ActualApprover)
	if err != nil {
		return err
	}

	gb.RunContext.VarStore.SetValue(gb.Output[keyOutputApprover], person)
	gb.RunContext.VarStore.SetValue(gb.Output[keyOutputDecision], gb.State.Decision.String())
	gb.RunContext.VarStore.SetValue(gb.Output[keyOutputComment], gb.State.Comment)

	return nil
}

//nolint:dupl //its not duplicate
func (gb *GoApproverBlock) handleBreachedSLA(ctx c.Context) error {
	const fn = "pipeline.approver.handleBreachedSLA"

	if !gb.State.CheckSLA {
		gb.State.SLAChecked = true
		gb.State.HalfSLAChecked = true

		return nil
	}

	log := logger.GetLogger(ctx)

	if gb.State.SLA >= 8 {
		seenAdditionalApprovers := map[string]bool{}
		emails := make([]string, 0, len(gb.State.Approvers)+len(gb.State.AdditionalApprovers))
		logins := getSliceFromMapOfStrings(gb.State.Approvers)

		for _, additionalApprover := range gb.State.AdditionalApprovers {
			// check if approver has not decisioned, and we did not see approver before
			if additionalApprover.Decision != nil || seenAdditionalApprovers[additionalApprover.ApproverLogin] {
				continue
			}

			seenAdditionalApprovers[additionalApprover.ApproverLogin] = true

			logins = append(logins, additionalApprover.ApproverLogin)
		}

		delegations, err := gb.RunContext.Services.HumanTasks.GetDelegationsByLogins(ctx, logins)
		if err != nil {
			log.WithError(err).Info(fn, fmt.Sprintf("approvers %v have no delegates", logins))
		}

		delegations = delegations.FilterByType("approvement")
		logins = delegations.GetUserInArrayWithDelegations(logins)

		var approverEmail string

		for i := range logins {
			approverEmail, err = gb.RunContext.Services.People.GetUserEmail(ctx, logins[i])
			if err != nil {
				log.WithError(err).Warning(fn, fmt.Sprintf("approver login %s not found", logins[i]))

				continue
			}

			emails = append(emails, approverEmail)
		}

		tpl := mail.NewApprovementSLATpl(
			gb.RunContext.WorkNumber,
			gb.RunContext.NotifName,
			gb.RunContext.Services.Sender.SdAddress,
			gb.State.ApproveStatusName,
		)

		filesList := []string{tpl.Image}

		files, iconEerr := gb.RunContext.GetIcons(filesList)
		if iconEerr != nil {
			return iconEerr
		}

		if len(emails) == 0 {
			return nil
		}

		err = gb.RunContext.Services.Sender.SendNotification(ctx, emails, files, tpl)
		if err != nil {
			return err
		}
	}

	if gb.State.AutoAction != nil {
		gb.RunContext.UpdateData.ByLogin = AutoApprover
		if setErr := gb.setApproveDecision(ctx,
			&approverUpdateParams{
				internalDecision: gb.State.AutoAction.ToDecision(),
				Comment:          AutoActionComment,
			}); setErr != nil {
			return setErr
		}
	}

	gb.State.SLAChecked = true
	gb.State.HalfSLAChecked = true

	return nil
}

//nolint:dupl //its not duplicate
func (gb *GoApproverBlock) handleHalfBreachedSLA(ctx c.Context) (err error) {
	const fn = "pipeline.approver.handleHalfBreachedSLA"

	if !gb.State.CheckSLA {
		gb.State.SLAChecked = true
		gb.State.HalfSLAChecked = true

		return nil
	}

	log := logger.GetLogger(ctx)

	if gb.State.SLA >= 8 {
		seenAdditionalApprovers := map[string]bool{}
		emails := make([]string, 0, len(gb.State.Approvers)+len(gb.State.AdditionalApprovers))
		logins := getSliceFromMapOfStrings(gb.State.Approvers)

		for _, additionalApprover := range gb.State.AdditionalApprovers {
			// check if approver has not decisioned, and we did not see approver before
			if additionalApprover.Decision != nil || seenAdditionalApprovers[additionalApprover.ApproverLogin] {
				continue
			}

			seenAdditionalApprovers[additionalApprover.ApproverLogin] = true

			logins = append(logins, additionalApprover.ApproverLogin)
		}

		delegations, err := gb.RunContext.Services.HumanTasks.GetDelegationsByLogins(ctx, logins)
		if err != nil {
			log.WithError(err).Info(fn, fmt.Sprintf("approvers %v have no delegates", logins))
		}

		delegations = delegations.FilterByType("approvement")
		logins = delegations.GetUserInArrayWithDelegations(logins)

		var approverEmail string

		for i := range logins {
			approverEmail, err = gb.RunContext.Services.People.GetUserEmail(ctx, logins[i])
			if err != nil {
				log.WithError(err).Warning(fn, fmt.Sprintf("approver login %s not found", logins[i]))

				continue
			}

			emails = append(emails, approverEmail)
		}

		if len(emails) == 0 {
			return nil
		}

		task, getVersionErr := gb.RunContext.Services.Storage.GetVersionByWorkNumber(ctx, gb.RunContext.WorkNumber)
		if getVersionErr != nil {
			return getVersionErr
		}

		processSettings, getVersionErr := gb.RunContext.Services.Storage.GetVersionSettings(ctx, task.VersionID.String())
		if getVersionErr != nil {
			return getVersionErr
		}

		taskRunContext, getDataErr := gb.RunContext.Services.Storage.GetTaskRunContext(ctx, gb.RunContext.WorkNumber)
		if getDataErr != nil {
			return getDataErr
		}

		login := task.Author

		recipient := getRecipientFromState(&taskRunContext.InitialApplication.ApplicationBody)

		if recipient != "" {
			login = recipient
		}

		if processSettings.ResubmissionPeriod > 0 {
			var getWorksErr error

			_, getWorksErr = gb.RunContext.Services.Storage.GetWorksForUserWithGivenTimeRange(
				ctx,
				processSettings.ResubmissionPeriod,
				login,
				task.VersionID.String(),
				gb.RunContext.WorkNumber,
			)
			if getWorksErr != nil {
				return getWorksErr
			}
		}

		slaInfoPtr, getSLAInfoErr := gb.RunContext.Services.SLAService.GetSLAInfoPtr(ctx, sla.InfoDTO{
			TaskCompletionIntervals: []entity.TaskCompletionInterval{{
				StartedAt:  gb.RunContext.CurrBlockStartTime,
				FinishedAt: gb.RunContext.CurrBlockStartTime.Add(time.Hour * 24 * 100),
			}},
			WorkType: sla.WorkHourType(gb.State.WorkType),
		})
		if getSLAInfoErr != nil {
			return getSLAInfoErr
		}

		lastWorksForUser := make([]*entity.EriusTask, 0)

		if processSettings.ResubmissionPeriod > 0 {
			var getWorksErr error

			lastWorksForUser, getWorksErr = gb.RunContext.Services.Storage.GetWorksForUserWithGivenTimeRange(
				ctx,
				processSettings.ResubmissionPeriod,
				login,
				task.VersionID.String(),
				gb.RunContext.WorkNumber,
			)
			if getWorksErr != nil {
				return getWorksErr
			}
		}

		tpl := mail.NewApprovementHalfSLATpl(
			gb.RunContext.WorkNumber,
			gb.RunContext.NotifName,
			gb.RunContext.Services.Sender.SdAddress,
			gb.State.ApproveStatusName,
			gb.RunContext.Services.SLAService.ComputeMaxDateFormatted(gb.RunContext.CurrBlockStartTime, gb.State.SLA,
				slaInfoPtr),
			lastWorksForUser,
		)

		files := []string{tpl.Image}

		if len(lastWorksForUser) != 0 {
			files = append(files, warningImg)
		}

		iconFiles, iconErr := gb.RunContext.GetIcons(files)
		if iconErr != nil {
			return err
		}

		errSend := gb.RunContext.Services.Sender.SendNotification(ctx, emails, iconFiles, tpl)
		if errSend != nil {
			return errSend
		}
	}

	gb.State.HalfSLAChecked = true

	return nil
}

// nolint:dupl // another action
func (gb *GoApproverBlock) handleReworkSLABreached(ctx c.Context) error {
	const fn = "pipeline.approver.handleReworkSLABreached"

	if !gb.State.CheckReworkSLA {
		return nil
	}

	log := logger.GetLogger(ctx)

	decision := ApproverDecisionRejected
	gb.State.Decision = &decision
	gb.State.EditingApp = nil

	comment := fmt.Sprintf("заявка автоматически перенесена в архив по истечении %d дней", gb.State.ReworkSLA/8)
	gb.State.Comment = &comment

	if stopErr := gb.RunContext.Services.Storage.StopTaskBlocks(ctx, gb.RunContext.TaskID); stopErr != nil {
		return stopErr
	}

	if stopErr := gb.RunContext.updateTaskStatus(ctx, db.RunStatusFinished, "", db.SystemLogin); stopErr != nil {
		return stopErr
	}

	if stopErr := gb.RunContext.Services.Storage.SendTaskToArchive(ctx, gb.RunContext.TaskID); stopErr != nil {
		return stopErr
	}

	loginsToNotify := []string{gb.RunContext.Initiator}

	var (
		em  string
		err error
	)

	emails := make([]string, 0, len(loginsToNotify))

	for _, login := range loginsToNotify {
		em, err = gb.RunContext.Services.People.GetUserEmail(ctx, login)
		if err != nil {
			log.WithError(err).Warning(fn, fmt.Sprintf("login %s not found", login))

			continue
		}

		emails = append(emails, em)
	}

	tpl := mail.NewReworkSLATpl(gb.RunContext.WorkNumber, gb.RunContext.NotifName,
		gb.RunContext.Services.Sender.SdAddress, gb.State.ReworkSLA, gb.State.CheckSLA)

	filesList := []string{tpl.Image}

	files, iconEerr := gb.RunContext.GetIcons(filesList)
	if iconEerr != nil {
		return iconEerr
	}

	err = gb.RunContext.Services.Sender.SendNotification(ctx, emails, files, tpl)
	if err != nil {
		return err
	}

	nodeEvents, err := gb.RunContext.GetCancelledStepsEvents(ctx)
	if err != nil {
		return err
	}

	for _, event := range nodeEvents {
		// event for this node will spawn later
		if event.NodeName == gb.Name {
			continue
		}

		gb.happenedEvents = append(gb.happenedEvents, event)
	}

	return nil
}

func (gb *GoApproverBlock) handleBreachedDayBeforeSLARequestAddInfo(ctx c.Context) error {
	const fn = "pipeline.approver.handleBreachedDayBeforeSLARequestAddInfo"

	if !gb.State.CheckDayBeforeSLARequestInfo {
		return nil
	}

	log := logger.GetLogger(ctx)

	loginsToNotify := []string{gb.RunContext.Initiator}

	emails := make([]string, 0, len(loginsToNotify))

	for _, login := range loginsToNotify {
		em, err := gb.RunContext.Services.People.GetUserEmail(ctx, login)
		if err != nil {
			log.WithError(err).Warning(fn, fmt.Sprintf("login %s not found", login))

			continue
		}

		emails = append(emails, em)
	}

	tpl := mail.NewDayBeforeRequestAddInfoSLABreached(
		gb.RunContext.WorkNumber,
		gb.RunContext.NotifName,
		gb.RunContext.Services.Sender.SdAddress,
	)

	filesList := []string{tpl.Image}

	files, iconEerr := gb.RunContext.GetIcons(filesList)
	if iconEerr != nil {
		return iconEerr
	}

	err := gb.RunContext.Services.Sender.SendNotification(ctx, emails, files, tpl)
	if err != nil {
		return err
	}

	gb.State.CheckDayBeforeSLARequestInfo = false

	return nil
}

//nolint:dupl // dont duplicate
func (gb *GoApproverBlock) HandleBreachedSLARequestAddInfo(ctx c.Context) error {
	const (
		fn = "pipeline.approver.HandleBreachedSLARequestAddInfo"
	)

	comment := "заявка автоматически перенесена в архив по истечении 3 дней"

	log := logger.GetLogger(ctx)

	decision := ApproverDecisionRejected
	gb.State.Decision = &decision
	gb.State.Comment = &comment

	if stopErr := gb.RunContext.Services.Storage.StopTaskBlocks(ctx, gb.RunContext.TaskID); stopErr != nil {
		return stopErr
	}

	if stopErr := gb.RunContext.updateTaskStatus(ctx, db.RunStatusFinished, "", db.SystemLogin); stopErr != nil {
		return stopErr
	}

	if stopErr := gb.RunContext.Services.Storage.SendTaskToArchive(ctx, gb.RunContext.TaskID); stopErr != nil {
		return stopErr
	}

	approvers := getSliceFromMapOfStrings(gb.State.Approvers)

	delegates, getDelegationsErr := gb.RunContext.Services.HumanTasks.GetDelegationsByLogins(ctx, approvers)
	if getDelegationsErr != nil {
		return getDelegationsErr
	}

	delegates = delegates.FilterByType("approvement")

	loginsToNotify := delegates.GetUserInArrayWithDelegations(approvers)
	loginsToNotify = append(loginsToNotify, gb.RunContext.Initiator)

	var (
		em  string
		err error
	)

	emails := make([]string, 0, len(loginsToNotify))

	for _, login := range loginsToNotify {
		em, err = gb.RunContext.Services.People.GetUserEmail(ctx, login)
		if err != nil {
			log.WithError(err).Warning(fn, fmt.Sprintf("login %s not found", login))

			continue
		}

		emails = append(emails, em)
	}

	tpl := mail.NewRequestAddInfoSLABreached(gb.RunContext.WorkNumber, gb.RunContext.NotifName,
		gb.RunContext.Services.Sender.SdAddress, gb.State.ReworkSLA)

	filesList := []string{tpl.Image}

	files, iconEerr := gb.RunContext.GetIcons(filesList)
	if iconEerr != nil {
		return iconEerr
	}

	err = gb.RunContext.Services.Sender.SendNotification(ctx, emails, files, tpl)
	if err != nil {
		return err
	}

	nodeEvents, err := gb.RunContext.GetCancelledStepsEvents(ctx)
	if err != nil {
		return err
	}

	for i := range nodeEvents {
		event := nodeEvents[i]
		// event for this node will spawn later
		if event.NodeName == gb.Name {
			continue
		}

		gb.happenedEvents = append(gb.happenedEvents, event)
	}

	return nil
}

//nolint:gocyclo //its ok here
func (gb *GoApproverBlock) toEditApplication(ctx c.Context, updateParams approverUpdateEditingParams) error {
	if gb.State.Decision != nil {
		return errors.New("decision already set")
	}

	_, approverFound := gb.State.Approvers[gb.RunContext.UpdateData.ByLogin]
	delegateFor, isDelegate := gb.RunContext.Delegations.FindDelegatorFor(
		gb.RunContext.UpdateData.ByLogin, getSliceFromMapOfStrings(gb.State.Approvers))

	if !(approverFound || isDelegate) && gb.RunContext.UpdateData.ByLogin != AutoApprover {
		return NewUserIsNotPartOfProcessErr()
	}

	if gb.isNextBlockServiceDesk() {
		errSet := gb.State.setEditAppToInitiator(gb.RunContext.UpdateData.ByLogin, delegateFor, updateParams)
		if errSet != nil {
			return errSet
		}

		if err := gb.notifyNeedRework(ctx); err != nil {
			return err
		}

		err := gb.RunContext.Services.Storage.FinishTaskBlocks(ctx, gb.RunContext.TaskID, []string{gb.Name}, false)
		if err != nil {
			return err
		}
	} else {
		if editErr := gb.State.setEditToNextBlock(gb.RunContext.UpdateData.ByLogin, delegateFor,
			updateParams); editErr != nil {
			return editErr
		}

		person, personErr := gb.RunContext.Services.ServiceDesc.GetSsoPerson(ctx, gb.RunContext.UpdateData.ByLogin)
		if personErr != nil {
			return personErr
		}

		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputApprover], person)
		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputDecision], ApproverDecisionSentToEdit)
		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputComment], updateParams.Comment)
	}

	return nil
}

func (gb *GoApproverBlock) isNextBlockServiceDesk() bool {
	for i := range gb.Sockets {
		if gb.Sockets[i].Id == approverEditAppSocketID &&
			utils.IsContainsInSlice("servicedesk_application_0", gb.Sockets[i].NextBlockIds) {
			return true
		}
	}

	return false
}

func (gb *GoApproverBlock) updateRequestApproverInfo(ctx c.Context) (err error) {
	var updateParams requestInfoParams

	delegations := gb.RunContext.Delegations.FilterByType("approvement")

	if err = json.Unmarshal(gb.RunContext.UpdateData.Parameters, &updateParams); err != nil {
		return errors.New("can't assert provided update requestApproverInfo data")
	}

	if gb.State.Decision != nil {
		return errors.New("decision already set")
	}

	var (
		id     = uuid.NewString()
		linkID *string
	)

	delegateFor, isDelegate := gb.State.userIsDelegate(gb.RunContext.UpdateData.ByLogin, delegations)

	if updateParams.Type == RequestAddInfoType {
		if !(gb.State.userIsAnyApprover(gb.RunContext.UpdateData.ByLogin) || isDelegate) {
			return NewUserIsNotPartOfProcessErr()
		}

		err = gb.notifyNeedMoreInfo(ctx)
		if err != nil {
			return err
		}

		gb.State.CheckDayBeforeSLARequestInfo = true
	}

	if updateParams.Type == ReplyAddInfoType {
		var (
			initiator    = gb.RunContext.Initiator
			currentLogin = gb.RunContext.UpdateData.ByLogin
		)

		if len(gb.State.AddInfo) == 0 {
			return errors.New("don't answer after request")
		}

		if currentLogin != initiator {
			return NewUserIsNotPartOfProcessErr()
		}

		if updateParams.LinkID == nil {
			return errors.New("linkId is null when reply")
		}

		parentEntry := gb.State.findAddInfoLogEntry(*updateParams.LinkID)
		if parentEntry == nil || parentEntry.Type == ReplyAddInfoType ||
			gb.State.addInfoLogEntryHasResponse(*updateParams.LinkID) {
			return errors.New("bad linkId to submit an answer")
		}

		linkID = updateParams.LinkID

		approverLogin, linkErr := setLinkIDRequest(id, *updateParams.LinkID, gb.State.AddInfo)
		if linkErr != nil {
			return linkErr
		}

		err = gb.notifyNewInfoReceived(ctx, approverLogin)
		if err != nil {
			return err
		}
	}

	gb.State.AddInfo = append(gb.State.AddInfo, AdditionalInfo{
		ID:          id,
		Type:        updateParams.Type,
		Comment:     updateParams.Comment,
		Attachments: updateParams.Attachments,
		LinkID:      linkID,
		Login:       gb.RunContext.UpdateData.ByLogin,
		CreatedAt:   time.Now(),
		DelegateFor: delegateFor,
	})

	return nil
}

//nolint:gocyclo //ok
func (gb *GoApproverBlock) updateReplyApproverInfo(ctx c.Context) (err error) {
	var updateParams replyInfoParams

	if err = json.Unmarshal(gb.RunContext.UpdateData.Parameters, &updateParams); err != nil {
		return errors.New("can't assert provided update replyInfoParams data")
	}

	if gb.State.Decision != nil {
		return errors.New("decision already set")
	}

	var (
		id           = uuid.NewString()
		linkID       *string
		initiator    = gb.RunContext.Initiator
		currentLogin = gb.RunContext.UpdateData.ByLogin
	)

	if len(gb.State.AddInfo) == 0 {
		return errors.New("don't answer after request")
	}

	if currentLogin != initiator {
		return NewUserIsNotPartOfProcessErr()
	}

	if updateParams.LinkID == nil {
		return errors.New("linkId is null when reply")
	}

	parentEntry := gb.State.findAddInfoLogEntry(*updateParams.LinkID)
	if parentEntry == nil || parentEntry.Type == ReplyAddInfoType ||
		gb.State.addInfoLogEntryHasResponse(*updateParams.LinkID) {
		return errors.New("bad linkId to submit an answer")
	}

	linkID = updateParams.LinkID

	approverLogin, linkErr := setLinkIDRequest(id, *updateParams.LinkID, gb.State.AddInfo)
	if linkErr != nil {
		return linkErr
	}

	err = gb.notifyNewInfoReceived(ctx, approverLogin)
	if err != nil {
		return err
	}

	gb.State.AddInfo = append(gb.State.AddInfo, AdditionalInfo{
		ID:          id,
		Type:        ReplyAddInfoType,
		Comment:     updateParams.Comment,
		Attachments: updateParams.Attachments,
		LinkID:      linkID,
		Login:       gb.RunContext.UpdateData.ByLogin,
		CreatedAt:   time.Now(),
		DelegateFor: "",
	})

	return nil
}

func setLinkIDRequest(replyID, linkID string, addInfo []AdditionalInfo) (string, error) {
	for i := range addInfo {
		if addInfo[i].ID == linkID {
			addInfo[i].LinkID = &replyID
			return addInfo[i].Login, nil
		}
	}

	return "", errors.New("not found request by linkId")
}

func (gb *GoApproverBlock) actionAcceptable(action ApproverAction) bool {
	for _, a := range gb.State.ActionList {
		if a.ID == string(action) {
			return true
		}
	}

	return false
}

func (gb *GoApproverBlock) Update(ctx c.Context) (interface{}, error) {
	data := gb.RunContext.UpdateData
	if data == nil {
		return nil, errors.New("empty data")
	}

	gb.RunContext.Delegations = gb.RunContext.Delegations.FilterByType("approvement")

	switch data.Action {
	case string(entity.TaskUpdateActionSLABreach):
		if errUpdate := gb.handleBreachedSLA(ctx); errUpdate != nil {
			return nil, errUpdate
		}
	case string(entity.TaskUpdateActionHalfSLABreach):
		if errUpdate := gb.handleHalfBreachedSLA(ctx); errUpdate != nil {
			return nil, errUpdate
		}
	case string(entity.TaskUpdateActionReworkSLABreach):
		if errUpdate := gb.handleReworkSLABreached(ctx); errUpdate != nil {
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

		login := gb.RunContext.UpdateData.ByLogin
		if login == ServiceAccount || login == ServiceAccountStage || login == ServiceAccountDev {
			gb.RunContext.UpdateData.ByLogin = updateParams.Username
		}

		updateParams.internalDecision = updateParams.Decision.ToDecision()

		if errUpdate := gb.setApproveDecision(ctx, &updateParams); errUpdate != nil {
			return nil, errUpdate
		}

	case string(entity.TaskUpdateActionAdditionalApprovement):
		var updateParams additionalApproverUpdateParams

		if err := json.Unmarshal(data.Parameters, &updateParams); err != nil {
			return nil, errors.Errorf("can't assert provided data: %v", err)
		}

		if err := updateParams.Validate(); err != nil {
			return nil, err
		}

		loginsToNotify, err := gb.State.SetDecisionByAdditionalApprover(gb.RunContext.UpdateData.ByLogin,
			updateParams, gb.RunContext.Delegations)
		if err != nil {
			return nil, err
		}

		loginsToNotify = append(loginsToNotify, gb.RunContext.Initiator)

		err = gb.notifyDecisionMadeByAdditionalApprover(ctx, loginsToNotify)
		if err != nil {
			return nil, err
		}

	case string(entity.TaskUpdateActionApproverSendEditApp):
		var updateParams approverUpdateEditingParams

		if err := json.Unmarshal(data.Parameters, &updateParams); err != nil {
			return nil, errors.New("can't assert provided data")
		}

		if errUpdate := gb.toEditApplication(ctx, updateParams); errUpdate != nil {
			return nil, errUpdate
		}

	case string(entity.TaskUpdateActionRequestApproveInfo):
		if errUpdate := gb.updateRequestApproverInfo(ctx); errUpdate != nil {
			return nil, errUpdate
		}

	case string(entity.TaskUpdateActionReplyApproverInfo):
		if errUpdate := gb.updateReplyApproverInfo(ctx); errUpdate != nil {
			return nil, errUpdate
		}

	case string(entity.TaskUpdateActionAddApprovers):
		var updateParams addApproversParams
		if err := json.Unmarshal(data.Parameters, &updateParams); err != nil {
			return nil, errors.New("can't assert provided data")
		}

		if errUpdate := gb.addApprovers(ctx, updateParams); errUpdate != nil {
			return nil, errUpdate
		}

	case string(entity.TaskUpdateActionDayBeforeSLARequestAddInfo):
		if errUpdate := gb.handleBreachedDayBeforeSLARequestAddInfo(ctx); errUpdate != nil {
			return nil, errUpdate
		}
	case string(entity.TaskUpdateActionSLABreachRequestAddInfo):
		if errUpdate := gb.HandleBreachedSLARequestAddInfo(ctx); errUpdate != nil {
			return nil, errUpdate
		}
	}

	var stateBytes []byte

	stateBytes, err := json.Marshal(gb.State)
	if err != nil {
		return nil, err
	}

	gb.RunContext.VarStore.ReplaceState(gb.Name, stateBytes)

	if gb.State.Decision != nil {
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
	}

	return nil, nil
}

func (gb *GoApproverBlock) addApprovers(ctx c.Context, u addApproversParams) error {
	logApprovers := []string{}
	delegateFor, isDelegate := gb.State.userIsDelegate(gb.RunContext.UpdateData.ByLogin, gb.RunContext.Delegations)

	if !(gb.State.userIsAnyApprover(gb.RunContext.UpdateData.ByLogin) || isDelegate) {
		return NewUserIsNotPartOfProcessErr()
	}

	crTime := time.Now()

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
		approverLogEntry := ApproverLogEntry{
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

		err := gb.notifyAdditionalApprovers(ctx, logApprovers, u.Attachments)
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
