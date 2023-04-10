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
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
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

func (gb *GoApproverBlock) setApproverDecision(u approverUpdateParams) error {
	if errUpdate := gb.State.SetDecision(gb.RunContext.UpdateData.ByLogin, u.internalDecision,
		u.Comment, u.Attachments, gb.RunContext.Delegations); errUpdate != nil {
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

		delegations, err := gb.RunContext.HumanTasks.GetDelegationsByLogins(ctx, logins)
		if err != nil {
			log.WithError(err).Info(fn, fmt.Sprintf("approvers %v have no delegates", logins))
		}
		delegations = delegations.FilterByType("approvement")
		logins = delegations.GetUserInArrayWithDelegations(logins)

		var approverEmail string
		for i := range logins {
			approverEmail, err = gb.RunContext.People.GetUserEmail(ctx, logins[i])
			if err != nil {
				log.WithError(err).Warning(fn, fmt.Sprintf("approver login %s not found", logins[i]))
				continue
			}
			emails = append(emails, approverEmail)
		}

		if len(emails) == 0 {
			return nil
		}

		task, getVersionErr := gb.RunContext.Storage.GetVersionByWorkNumber(ctx, gb.RunContext.WorkNumber)
		if getVersionErr != nil {
			return getVersionErr
		}

		processSettings, getVersionErr := gb.RunContext.Storage.GetVersionSettings(ctx, task.VersionID.String())
		if getVersionErr != nil {
			return getVersionErr
		}

		taskRunContext, getDataErr := gb.RunContext.Storage.GetTaskRunContext(ctx, gb.RunContext.WorkNumber)
		if getDataErr != nil {
			return getDataErr
		}

		login := task.Author

		recipient := getRecipientFromState(&taskRunContext.InitialApplication.ApplicationBody)

		if recipient != "" {
			login = recipient
		}

		lastWorksForUser := make([]*entity.EriusTask, 0)

		if processSettings.ResubmissionPeriod > 0 {
			var getWorksErr error
			lastWorksForUser, getWorksErr = gb.RunContext.Storage.GetWorksForUserWithGivenTimeRange(
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

		err = gb.RunContext.Sender.SendNotification(ctx, emails, nil,
			mail.NewApprovementSLATpl(
				gb.RunContext.WorkNumber,
				gb.RunContext.WorkTitle,
				gb.RunContext.Sender.SdAddress,
				gb.State.ApproveStatusName,
				lastWorksForUser,
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

//nolint:dupl,gocyclo //its not duplicate
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

		delegations, err := gb.RunContext.HumanTasks.GetDelegationsByLogins(ctx, logins)
		if err != nil {
			log.WithError(err).Info(fn, fmt.Sprintf("approvers %v have no delegates", logins))
		}
		delegations = delegations.FilterByType("approvement")
		logins = delegations.GetUserInArrayWithDelegations(logins)

		var approverEmail string
		for i := range logins {
			approverEmail, err = gb.RunContext.People.GetUserEmail(ctx, logins[i])
			if err != nil {
				log.WithError(err).Warning(fn, fmt.Sprintf("approver login %s not found", logins[i]))
				continue
			}
			emails = append(emails, approverEmail)
		}

		if len(emails) == 0 {
			return nil
		}

		task, getVersionErr := gb.RunContext.Storage.GetVersionByWorkNumber(ctx, gb.RunContext.WorkNumber)
		if getVersionErr != nil {
			return getVersionErr
		}

		processSettings, getVersionErr := gb.RunContext.Storage.GetVersionSettings(ctx, task.VersionID.String())
		if getVersionErr != nil {
			return getVersionErr
		}

		taskRunContext, getDataErr := gb.RunContext.Storage.GetTaskRunContext(ctx, gb.RunContext.WorkNumber)
		if getDataErr != nil {
			return getDataErr
		}

		login := task.Author

		recipient := getRecipientFromState(&taskRunContext.InitialApplication.ApplicationBody)

		if recipient != "" {
			login = recipient
		}

		lastWorksForUser := make([]*entity.EriusTask, 0)

		if processSettings.ResubmissionPeriod > 0 {
			var getWorksErr error
			lastWorksForUser, getWorksErr = gb.RunContext.Storage.GetWorksForUserWithGivenTimeRange(
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

		err = gb.RunContext.Sender.SendNotification(ctx, emails, nil,
			mail.NewApprovementHalfSLATpl(
				gb.RunContext.WorkNumber,
				gb.RunContext.WorkTitle,
				gb.RunContext.Sender.SdAddress,
				gb.State.ApproveStatusName,
				lastWorksForUser,
			),
		)
		if err != nil {
			return err
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

	if stopErr := gb.RunContext.Storage.StopTaskBlocks(ctx, gb.RunContext.TaskID); stopErr != nil {
		return stopErr
	}

	if stopErr := gb.RunContext.updateTaskStatus(ctx, db.RunStatusFinished); stopErr != nil {
		return stopErr
	}

	if stopErr := gb.RunContext.Storage.SendTaskToArchive(ctx, gb.RunContext.TaskID); stopErr != nil {
		return stopErr
	}

	loginsToNotify := []string{gb.RunContext.Initiator}

	var em string
	var err error
	emails := make([]string, 0, len(loginsToNotify))
	for _, login := range loginsToNotify {
		em, err = gb.RunContext.People.GetUserEmail(ctx, login)
		if err != nil {
			log.WithError(err).Warning(fn, fmt.Sprintf("login %s not found", login))
			continue
		}

		emails = append(emails, em)
	}

	tpl := mail.NewReworkSLATpl(gb.RunContext.WorkNumber, gb.RunContext.Sender.SdAddress, gb.State.ReworkSLA)
	err = gb.RunContext.Sender.SendNotification(ctx, emails, nil, tpl)
	if err != nil {
		return err
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
		em, err := gb.RunContext.People.GetUserEmail(ctx, login)
		if err != nil {
			log.WithError(err).Warning(fn, fmt.Sprintf("login %s not found", login))
			continue
		}

		emails = append(emails, em)
	}

	tpl := mail.NewDayBeforeRequestAddInfoSLABreached(gb.RunContext.WorkNumber, gb.RunContext.Sender.SdAddress)
	err := gb.RunContext.Sender.SendNotification(ctx, emails, nil, tpl)
	if err != nil {
		return err
	}

	gb.State.CheckDayBeforeSLARequestInfo = false

	return nil
}

//nolint:dupl // dont duplicate
func (gb *GoApproverBlock) HandleBreachedSLARequestAddInfo(ctx c.Context) error {
	const fn = "pipeline.approver.HandleBreachedSLARequestAddInfo"
	var comment = "заявка автоматически перенесена в архив по истечении 3 дней"

	log := logger.GetLogger(ctx)

	decision := ApproverDecisionRejected
	gb.State.Decision = &decision
	gb.State.Comment = &comment

	if stopErr := gb.RunContext.Storage.StopTaskBlocks(ctx, gb.RunContext.TaskID); stopErr != nil {
		return stopErr
	}

	if stopErr := gb.RunContext.updateTaskStatus(ctx, db.RunStatusFinished); stopErr != nil {
		return stopErr
	}

	if stopErr := gb.RunContext.Storage.SendTaskToArchive(ctx, gb.RunContext.TaskID); stopErr != nil {
		return stopErr
	}

	approvers := getSliceFromMapOfStrings(gb.State.Approvers)
	delegates, getDelegationsErr := gb.RunContext.HumanTasks.GetDelegationsByLogins(ctx, approvers)
	if getDelegationsErr != nil {
		return getDelegationsErr
	}
	delegates = delegates.FilterByType("approvement")

	loginsToNotify := delegates.GetUserInArrayWithDelegations(approvers)
	loginsToNotify = append(loginsToNotify, gb.RunContext.Initiator)

	var em string
	var err error
	emails := make([]string, 0, len(loginsToNotify))
	for _, login := range loginsToNotify {
		em, err = gb.RunContext.People.GetUserEmail(ctx, login)
		if err != nil {
			log.WithError(err).Warning(fn, fmt.Sprintf("login %s not found", login))
			continue
		}

		emails = append(emails, em)
	}

	tpl := mail.NewRequestAddInfoSLABreached(gb.RunContext.WorkNumber, gb.RunContext.Sender.SdAddress)
	err = gb.RunContext.Sender.SendNotification(ctx, emails, nil, tpl)
	if err != nil {
		return err
	}

	return nil
}

//nolint:gocyclo //its ok here
func (gb *GoApproverBlock) setEditApplication(ctx c.Context, updateParams approverUpdateEditingParams) error {
	errSet := gb.State.setEditApp(gb.RunContext.UpdateData.ByLogin, updateParams, gb.RunContext.Delegations)
	if errSet != nil {
		return errSet
	}

	err := gb.notificateNeedRework(ctx)
	if err != nil {
		return err
	}

	return nil
}

//nolint:gocyclo //ok
func (gb *GoApproverBlock) updateRequestApproverInfo(ctx c.Context) (err error) {
	var updateParams requestInfoParams
	var delegations = gb.RunContext.Delegations

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

	delegateFor, isDelegate := gb.State.userIsDelegate(gb.RunContext.UpdateData.ByLogin, delegations)

	if updateParams.Type == RequestAddInfoType {
		if !(gb.State.userIsAnyApprover(gb.RunContext.UpdateData.ByLogin) || isDelegate) {
			return NewUserIsNotPartOfProcessErr()
		}

		err := gb.notificateNeedMoreInfo(ctx)
		if err != nil {
			return err
		}

		gb.State.CheckDayBeforeSLARequestInfo = true
	}

	if updateParams.Type == ReplyAddInfoType {
		var initiator = gb.RunContext.Initiator
		var currentLogin = gb.RunContext.UpdateData.ByLogin

		if len(gb.State.AddInfo) == 0 {
			return errors.New("don't answer after request")
		}

		if currentLogin != initiator {
			return NewUserIsNotPartOfProcessErr()
		}

		if updateParams.LinkId == nil {
			return errors.New("linkId is null when reply")
		}

		linkId = updateParams.LinkId
		linkErr := setLinkIdRequest(id, *updateParams.LinkId, gb.State.AddInfo)
		if linkErr != nil {
			return linkErr
		}

		workHours := getWorkHoursBetweenDates(
			gb.State.AddInfo[len(gb.State.AddInfo)-1].CreatedAt,
			time.Now(),
		)
		gb.State.IncreaseSLA(workHours)

		err := gb.notificateNewInfoRecieved(ctx)
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
		DelegateFor: delegateFor,
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

		updateParams.internalDecision = updateParams.Decision.ToDecision()

		if errUpdate := gb.setApproverDecision(updateParams); errUpdate != nil {
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

		loginsToNotify, err := gb.State.SetDecisionByAdditionalApprover(gb.RunContext.UpdateData.ByLogin,
			updateParams, gb.RunContext.Delegations)

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
		if errUpdate := gb.setEditApplication(ctx, updateParams); errUpdate != nil {
			return nil, errUpdate
		}

	case string(entity.TaskUpdateActionRequestApproveInfo):
		if errUpdate := gb.updateRequestApproverInfo(ctx); errUpdate != nil {
			return nil, errUpdate
		}

	case string(entity.TaskUpdateActionCancelApp):
		if errUpdate := gb.cancelPipeline(ctx); errUpdate != nil {
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

	return nil, nil
}

// nolint:dupl // another action
func (gb *GoApproverBlock) cancelPipeline(ctx c.Context) error {
	var currentLogin = gb.RunContext.UpdateData.ByLogin
	var initiator = gb.RunContext.Initiator

	if currentLogin != initiator {
		return NewUserIsNotPartOfProcessErr()
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

func (gb *GoApproverBlock) notificateAdditionalApprovers(ctx c.Context, logins, attachsId []string) error {
	delegates, err := gb.RunContext.HumanTasks.GetDelegationsByLogins(ctx, logins)
	if err != nil {
		return err
	}
	delegates = delegates.FilterByType("approvement")

	loginsToNotify := delegates.GetUserInArrayWithDelegations(logins)

	emails := make([]string, 0, len(loginsToNotify))
	for _, login := range loginsToNotify {
		approverEmail, emailErr := gb.RunContext.People.GetUserEmail(ctx, login)
		if emailErr != nil {
			return emailErr
		}

		emails = append(emails, approverEmail)
	}

	attachFiles, err := gb.RunContext.ServiceDesc.GetAttachments(ctx, map[string][]string{"Ids": attachsId})
	if err != nil {
		return err
	}

	files := make([]email.Attachment, 0)
	for k := range attachFiles {
		files = append(files, attachFiles[k]...)
	}

	emails = utils.UniqueStrings(emails)

	task, getVersionErr := gb.RunContext.Storage.GetVersionByWorkNumber(ctx, gb.RunContext.WorkNumber)
	if getVersionErr != nil {
		return getVersionErr
	}

	processSettings, getVersionErr := gb.RunContext.Storage.GetVersionSettings(ctx, task.VersionID.String())
	if getVersionErr != nil {
		return getVersionErr
	}

	taskRunContext, getDataErr := gb.RunContext.Storage.GetTaskRunContext(ctx, gb.RunContext.WorkNumber)
	if getDataErr != nil {
		return getDataErr
	}

	login := task.Author

	recipient := getRecipientFromState(&taskRunContext.InitialApplication.ApplicationBody)

	if recipient != "" {
		login = recipient
	}

	lastWorksForUser := make([]*entity.EriusTask, 0)

	if processSettings.ResubmissionPeriod > 0 {
		var getWorksErr error
		lastWorksForUser, getWorksErr = gb.RunContext.Storage.GetWorksForUserWithGivenTimeRange(ctx,
			processSettings.ResubmissionPeriod,
			login,
			task.VersionID.String(),
			gb.RunContext.WorkNumber,
		)
		if getWorksErr != nil {
			return getWorksErr
		}
	}

	for i := range emails {
		tpl := mail.NewAddApproversTpl(
			gb.RunContext.WorkNumber,
			gb.RunContext.WorkTitle,
			gb.RunContext.Sender.SdAddress,
			gb.State.ApproveStatusName,
			lastWorksForUser,
		)

		err = gb.RunContext.Sender.SendNotification(ctx, []string{emails[i]}, files, tpl)
		if err != nil {
			return err
		}
	}

	return nil
}

// notificateDecisionMadeByAdditionalApprover notifies requesting approvers
// and the task initiator that an additional approver has left a review
func (gb *GoApproverBlock) notificateDecisionMadeByAdditionalApprover(ctx c.Context, loginsToNotify []string) error {
	delegates, err := gb.RunContext.HumanTasks.GetDelegationsByLogins(ctx, loginsToNotify)
	if err != nil {
		return err
	}
	delegates = delegates.FilterByType("approvement")

	loginsWithDelegates := delegates.GetUserInArrayWithDelegations(loginsToNotify)

	emailsToNotify := make([]string, 0, len(loginsWithDelegates))
	for _, login := range loginsWithDelegates {
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

	tpl := mail.NewDecisionMadeByAdditionalApprover(
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

func (gb *GoApproverBlock) notificateNeedRework(ctx c.Context) error {
	l := logger.GetLogger(ctx)

	delegates, err := gb.RunContext.HumanTasks.GetDelegationsFromLogin(ctx, gb.RunContext.Initiator)
	if err != nil {
		return err
	}

	loginsToNotify := delegates.GetUserInArrayWithDelegations([]string{gb.RunContext.Initiator})

	var em string
	emails := make([]string, 0, len(loginsToNotify))
	for _, login := range loginsToNotify {
		em, err = gb.RunContext.People.GetUserEmail(ctx, login)
		if err != nil {
			l.WithField("login", login).WithError(err).Warning("couldn't get email")
			continue
		}

		emails = append(emails, em)
	}

	tpl := mail.NewAnswerSendToEditTpl(gb.RunContext.WorkNumber, gb.RunContext.WorkTitle, gb.RunContext.Sender.SdAddress)
	err = gb.RunContext.Sender.SendNotification(ctx, emails, nil, tpl)
	if err != nil {
		return err
	}

	return nil
}

func (gb *GoApproverBlock) notificateNewInfoRecieved(ctx c.Context) error {
	l := logger.GetLogger(ctx)

	logins := []string{gb.RunContext.UpdateData.ByLogin}
	for i := range gb.State.AdditionalApprovers {
		logins = append(logins, gb.State.AdditionalApprovers[i].ApproverLogin)
	}

	delegates, err := gb.RunContext.HumanTasks.GetDelegationsByLogins(ctx, logins)
	if err != nil {
		return err
	}

	delegates = delegates.FilterByType("approvement")
	loginsToNotify := delegates.GetUserInArrayWithDelegations(logins)

	var em string
	emails := make([]string, 0, len(loginsToNotify))
	for _, login := range loginsToNotify {
		em, err = gb.RunContext.People.GetUserEmail(ctx, login)
		if err != nil {
			l.WithField("login", login).WithError(err).Warning("couldn't get email")
			return err
		}

		emails = append(emails, em)
	}

	tpl := mail.NewAnswerApproverInfoTpl(gb.RunContext.WorkNumber, gb.RunContext.WorkTitle, gb.RunContext.Sender.SdAddress)
	err = gb.RunContext.Sender.SendNotification(ctx, emails, nil, tpl)
	if err != nil {
		return err
	}

	return nil
}

func (gb *GoApproverBlock) notificateNeedMoreInfo(ctx c.Context) error {
	l := logger.GetLogger(ctx)

	loginsToNotify := make([]string, 0)
	for login := range gb.State.Approvers {
		loginsToNotify = append(loginsToNotify, login)
	}

	emails := make([]string, 0, len(loginsToNotify))
	for _, login := range loginsToNotify {
		em, err := gb.RunContext.People.GetUserEmail(ctx, login)
		if err != nil {
			l.WithField("login", login).WithError(err).Warning("couldn't get email")
			return err
		}

		emails = append(emails, em)
	}

	tpl := mail.NewRequestApproverInfoTpl(gb.RunContext.WorkNumber, gb.RunContext.WorkTitle, gb.RunContext.Sender.SdAddress)

	err := gb.RunContext.Sender.SendNotification(ctx, emails, nil, tpl)
	if err != nil {
		return err
	}

	return nil
}
