package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	e "gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sla"
)

type updateFillFormParams struct {
	Description     string                 `json:"description"`
	ApplicationBody map[string]interface{} `json:"application_body"`
	BlockID         string                 `json:"block_id"`
}

func (a *updateFillFormParams) Validate() error {
	if a.BlockID == "" || a.Description == "" || len(a.ApplicationBody) == 0 {
		return errors.New("empty form data")
	}

	return nil
}

//nolint:gocyclo,gocognit //ok
func (gb *GoFormBlock) Update(ctx context.Context) (interface{}, error) {
	data := gb.RunContext.UpdateData
	if data == nil {
		return nil, errors.New("empty data")
	}

	wasAlreadyFilled := len(gb.State.ApplicationBody) > 0
	updateInOtherBlocks := false

	executorsLogins := make(map[string]struct{}, 0)
	for i := range gb.State.Executors {
		executorsLogins[i] = gb.State.Executors[i]
	}

	switch data.Action {
	case string(e.TaskUpdateActionSLABreach):
		if errUpdate := gb.handleBreachedSLA(ctx); errUpdate != nil {
			return nil, errUpdate
		}
	case string(e.TaskUpdateActionHalfSLABreach):
		if errUpdate := gb.handleHalfSLABreached(ctx); errUpdate != nil {
			return nil, errUpdate
		}
	case string(e.TaskUpdateActionRequestFillForm):
		if !gb.State.IsTakenInWork {
			return nil, errors.New("is not taken in work")
		}

		if errFill := gb.handleRequestFillForm(ctx, data); errFill != nil {
			return nil, errFill
		}

		updateInOtherBlocks = true
	case string(e.TaskUpdateActionFormExecutorStartWork):
		if gb.State.IsTakenInWork {
			return nil, errors.New("is already taken in work")
		}

		if errUpdate := gb.formExecutorStartWork(ctx, executorsLogins); errUpdate != nil {
			return nil, errUpdate
		}
	case string(e.TaskUpdateActionReload):
	}

	deadline, deadlineErr := gb.getDeadline(ctx, gb.State.WorkType)
	if deadlineErr != nil {
		return nil, deadlineErr
	}

	gb.State.Deadline = deadline

	err := gb.setEvents(ctx, &setFormEventsDto{
		wasAlreadyFilled: wasAlreadyFilled,
		executorsLogins:  executorsLogins,
	})
	if err != nil {
		return nil, err
	}

	var stateBytes []byte

	stateBytes, err = json.Marshal(gb.State)
	if err != nil {
		return nil, err
	}

	gb.RunContext.VarStore.ReplaceState(gb.Name, stateBytes)

	if updateInOtherBlocks {
		taskID := gb.RunContext.TaskID.String()

		err = gb.RunContext.Services.Storage.UpdateBlockStateInOthers(ctx, gb.Name, taskID, stateBytes)
		if err != nil {
			return nil, err
		}

		executor, _ := gb.RunContext.VarStore.GetValue(gb.Output[keyOutputFormExecutor])
		body, _ := gb.RunContext.VarStore.GetValue(gb.Output[keyOutputFormBody])

		blockValues := map[string]interface{}{
			gb.Name + ".executor":         executor,
			gb.Name + ".application_body": body,
		}

		errBlockVariables := gb.RunContext.Services.Storage.UpdateBlockVariablesInOthers(ctx, taskID, blockValues)
		if errBlockVariables != nil {
			return nil, errBlockVariables
		}
	}

	return nil, nil
}

func (gb *GoFormBlock) checkFormFilled() error {
	l := logger.GetLogger(context.Background())

	for _, form := range gb.State.FormsAccessibility {
		formState, ok := gb.RunContext.VarStore.State[form.NodeID]
		if !ok {
			continue
		}

		if gb.Name == form.NodeID {
			continue
		}

		if form.AccessType == requiredFillAccessType {
			if gb.checkForEmptyForm(formState, l) {
				comment := fmt.Sprintf("%s have empty form", form.NodeID)

				return errors.New(comment)
			}
		}
	}

	return nil
}

func (gb *GoFormBlock) handleRequestFillForm(ctx context.Context, data *script.BlockUpdateData) error {
	var updateParams updateFillFormParams

	isWorkOnEditing, err := gb.RunContext.Services.Storage.CheckIsOnEditing(ctx, gb.RunContext.TaskID.String())
	if err != nil {
		return err
	}

	if isWorkOnEditing {
		return errors.New("work is on editing by initiator")
	}

	err = json.Unmarshal(data.Parameters, &updateParams)
	if err != nil {
		return errors.New("can't assert provided data")
	}

	if updateParams.BlockID != gb.Name {
		return fmt.Errorf("wrong form id: %s, gb.Name: %s", updateParams.BlockID, gb.Name)
	}

	err = gb.handleStateFullness(ctx, data)
	if err != nil {
		return err
	}

	if formErr := gb.checkFormFilled(); formErr != nil {
		return formErr
	}

	gb.State.ApplicationBody = updateParams.ApplicationBody
	gb.State.Description = updateParams.Description

	gb.State.ChangesLog = append(
		[]ChangesLogItem{
			{
				Description:     updateParams.Description,
				ApplicationBody: updateParams.ApplicationBody,
				CreatedAt:       time.Now(),
				Executor:        data.ByLogin,
			},
		},
		gb.State.ChangesLog...,
	)

	schema, getSchemaErr := gb.RunContext.Services.ServiceDesc.GetSchemaByID(ctx, gb.State.SchemaID)
	if getSchemaErr != nil {
		return getSchemaErr
	}

	byteSchema, marshalErr := json.Marshal(schema)
	if marshalErr != nil {
		return marshalErr
	}

	byteApplicationBody, marshalApBodyErr := json.Marshal(gb.State.ApplicationBody)
	if marshalApBodyErr != nil {
		return marshalApBodyErr
	}

	if validErr := script.ValidateJSONByJSONSchema(string(byteApplicationBody), string(byteSchema)); validErr != nil {
		return validErr
	}

	if gb.State.ActualExecutor != nil {
		personData, err := gb.RunContext.Services.ServiceDesc.GetSsoPerson(ctx, *gb.State.ActualExecutor)
		if err != nil {
			return err
		}

		if valOutputFormExecutor, ok := gb.Output[keyOutputFormExecutor]; ok {
			gb.RunContext.VarStore.SetValue(valOutputFormExecutor, personData)
		}
	}

	if valOutputFormBody, ok := gb.Output[keyOutputFormBody]; ok {
		gb.RunContext.VarStore.SetValue(valOutputFormBody, gb.State.ApplicationBody)
	}

	return nil
}

func (gb *GoFormBlock) handleStateFullness(ctx context.Context, data *script.BlockUpdateData) error {
	if gb.State.IsFilled {
		err := gb.handleFilledState(ctx, data)
		if err != nil {
			return err
		}
	} else {
		err := gb.handleEmptyState(data)
		if err != nil {
			return err
		}
	}

	return nil
}

func (gb *GoFormBlock) handleFilledState(ctx context.Context, data *script.BlockUpdateData) error {
	isAllowed, checkEditErr := gb.RunContext.Services.Storage.CheckUserCanEditForm(
		ctx,
		gb.RunContext.WorkNumber,
		gb.Name,
		data.ByLogin,
	)
	if checkEditErr != nil {
		return checkEditErr
	}

	if !isAllowed {
		return NewUserIsNotPartOfProcessErr()
	}

	isActualUserEqualAutoFillUser := gb.State.ActualExecutor != nil && *gb.State.ActualExecutor == AutoFillUser

	if isActualUserEqualAutoFillUser {
		gb.State.ActualExecutor = &data.ByLogin
	}

	return nil
}

func (gb *GoFormBlock) handleEmptyState(data *script.BlockUpdateData) error {
	_, executorFound := gb.State.Executors[data.ByLogin]
	if !executorFound {
		return NewUserIsNotPartOfProcessErr()
	}

	gb.State.IsExpired = gb.State.Deadline.Before(time.Now())

	gb.State.ActualExecutor = &data.ByLogin
	gb.State.IsFilled = true

	return nil
}

//nolint:dupl //its not duplicate
func (gb *GoFormBlock) handleBreachedSLA(ctx context.Context) error {
	const fn = "pipeline.form.handleBreachedSLA"

	if !gb.State.CheckSLA {
		gb.State.SLAChecked = true

		return nil
	}

	log := logger.GetLogger(ctx)

	if gb.State.SLA >= 8 {
		emails := make([]string, 0, len(gb.State.Executors))
		logins := getSliceFromMap(gb.State.Executors)

		for i := range logins {
			executorEmail, err := gb.RunContext.Services.People.GetUserEmail(ctx, logins[i])
			if err != nil {
				log.WithError(err).Warning(fn, fmt.Sprintf("executor login %s not found", logins[i]))

				continue
			}

			emails = append(emails, executorEmail)
		}

		tpl := mail.NewFormSLATpl(
			gb.RunContext.WorkNumber,
			gb.RunContext.NotifName,
			gb.RunContext.Services.Sender.SdAddress,
		)

		filesList := []string{tpl.Image}

		files, iconEerr := gb.RunContext.GetIcons(filesList)
		if iconEerr != nil {
			return iconEerr
		}

		if len(emails) == 0 {
			return nil
		}

		err := gb.RunContext.Services.Sender.SendNotification(
			ctx,
			emails,
			files,
			tpl,
		)
		if err != nil {
			return err
		}
	}

	gb.State.HalfSLAChecked = true
	gb.State.SLAChecked = true

	return nil
}

//nolint:dupl //its not duplicate
func (gb *GoFormBlock) handleHalfSLABreached(ctx context.Context) error {
	const fn = "pipeline.form.handleHalfSLABreached"

	if gb.State.HalfSLAChecked {
		return nil
	}

	if gb.State.SLA >= 8 {
		logins := getSliceFromMap(gb.State.Executors)
		emails := gb.mapLoginsToEmails(ctx, fn, logins)

		slaInfoPtr, getSLAInfoErr := gb.RunContext.Services.SLAService.GetSLAInfoPtr(
			ctx,
			sla.InfoDTO{
				TaskCompletionIntervals: []e.TaskCompletionInterval{
					{
						StartedAt:  gb.RunContext.CurrBlockStartTime,
						FinishedAt: gb.RunContext.CurrBlockStartTime.Add(time.Hour * 24 * 100),
					},
				},
				WorkType: sla.WorkHourType(gb.State.WorkType),
			},
		)
		if getSLAInfoErr != nil {
			return getSLAInfoErr
		}

		tpl := mail.NewFormDayHalfSLATpl(
			gb.RunContext.WorkNumber,
			gb.RunContext.NotifName,
			gb.RunContext.Services.Sender.SdAddress,
			gb.RunContext.Services.SLAService.ComputeMaxDateFormatted(
				gb.RunContext.CurrBlockStartTime,
				gb.State.SLA,
				slaInfoPtr,
			),
		)

		filesList := []string{tpl.Image}

		files, iconEerr := gb.RunContext.GetIcons(filesList)
		if iconEerr != nil {
			return iconEerr
		}

		if len(emails) == 0 {
			return nil
		}

		err := gb.RunContext.Services.Sender.SendNotification(
			ctx,
			emails,
			files,
			tpl,
		)
		if err != nil {
			return err
		}
	}

	gb.State.HalfSLAChecked = true

	return nil
}

func (gb *GoFormBlock) mapLoginsToEmails(ctx context.Context, fn string, logins []string) []string {
	log := logger.GetLogger(ctx)
	emails := make([]string, 0, len(logins))

	for i := range logins {
		executorEmail, err := gb.RunContext.Services.People.GetUserEmail(ctx, logins[i])
		if err != nil {
			log.WithError(err).Warning(fn, fmt.Sprintf("executor login %s not found", logins[i]))

			continue
		}

		emails = append(emails, executorEmail)
	}

	return emails
}

func (gb *GoFormBlock) formExecutorStartWork(ctx context.Context, executorLogins map[string]struct{}) (err error) {
	currentLogin := gb.RunContext.UpdateData.ByLogin
	_, executorFound := gb.State.Executors[currentLogin]

	_, isDelegate := gb.RunContext.Delegations.FindDelegatorFor(currentLogin, getSliceFromMap(gb.State.Executors))
	if !(executorFound || isDelegate) {
		return NewUserIsNotPartOfProcessErr()
	}

	gb.State.Executors = map[string]struct{}{
		gb.RunContext.UpdateData.ByLogin: {},
	}

	gb.State.IsTakenInWork = true

	slaInfoPtr, getSLAInfoErr := gb.RunContext.Services.SLAService.GetSLAInfoPtr(ctx, sla.InfoDTO{
		TaskCompletionIntervals: []e.TaskCompletionInterval{{
			StartedAt:  gb.RunContext.CurrBlockStartTime,
			FinishedAt: gb.RunContext.CurrBlockStartTime.Add(time.Hour * 24 * 100),
		}},
		WorkType: sla.WorkHourType(gb.State.WorkType),
	})

	if getSLAInfoErr != nil {
		return getSLAInfoErr
	}

	workHours := gb.RunContext.Services.SLAService.GetWorkHoursBetweenDates(
		gb.RunContext.CurrBlockStartTime,
		time.Now(),
		slaInfoPtr,
	)

	gb.State.IncreaseSLA(workHours)

	byLogin := gb.RunContext.UpdateData.ByLogin

	if err = gb.emailGroupExecutors(ctx, byLogin, executorLogins); err != nil {
		return nil
	}

	return nil
}

func (a *FormData) IncreaseSLA(addSLA int) {
	a.SLA += addSLA
}

func (gb *GoFormBlock) emailGroupExecutors(ctx context.Context, loginTakenInWork string, logins map[string]struct{}) (err error) {
	log := logger.GetLogger(ctx)
	executors := getSliceFromMap(logins)

	emails := make([]string, 0, len(executors))

	for _, login := range executors {
		if login != loginTakenInWork {
			email, emailErr := gb.RunContext.Services.People.GetUserEmail(ctx, login)
			if emailErr != nil {
				log.WithField("login", login).WithError(emailErr).Warning("couldn't get email")

				continue
			}

			emails = append(emails, email)
		}
	}

	formExecutorSSOPerson, getUserErr := gb.RunContext.Services.People.GetUser(ctx, loginTakenInWork, true)
	if getUserErr != nil {
		return getUserErr
	}

	typeExecutor, convertErr := formExecutorSSOPerson.ToUserinfo()
	if convertErr != nil {
		return convertErr
	}

	initiator, err := gb.RunContext.Services.People.GetUser(ctx, gb.RunContext.Initiator, false)
	if err != nil {
		return getUserErr
	}

	inititatorInfo, err := initiator.ToUserinfo()
	if err != nil {
		return convertErr
	}

	description, files, err := gb.RunContext.makeNotificationDescription(ctx, gb.Name)
	if err != nil {
		return err
	}

	filesAttach, _, err := gb.RunContext.makeNotificationAttachment(ctx)
	if err != nil {
		return err
	}

	attachment, err := gb.RunContext.GetAttach(ctx, filesAttach)
	if err != nil {
		return err
	}

	tpl := mail.NewFormExecutionTakenInWorkTpl(
		&mail.ExecutorNotifTemplate{
			WorkNumber:  gb.RunContext.WorkNumber,
			Name:        gb.RunContext.NotifName,
			SdURL:       gb.RunContext.Services.Sender.SdAddress,
			Description: description,
			Executor:    typeExecutor,
			Initiator:   inititatorInfo,
			Mailto:      gb.RunContext.Services.Sender.FetchEmail,
		})

	iconsName := []string{tpl.Image, userImg}

	iconFiles, iconErr := gb.RunContext.GetIcons(iconsName)
	if iconErr != nil {
		return err
	}

	files = append(files, iconFiles...)

	if errSend := gb.RunContext.Services.Sender.SendNotification(ctx, emails, files, tpl); errSend != nil {
		return errSend
	}

	emailTakenInWork, emailErr := gb.RunContext.Services.People.GetUserEmail(ctx, loginTakenInWork)
	if emailErr != nil {
		return emailErr
	}

	slaInfoPtr, getSLAInfoErr := gb.RunContext.Services.SLAService.GetSLAInfoPtr(ctx, sla.InfoDTO{
		TaskCompletionIntervals: []e.TaskCompletionInterval{{
			StartedAt:  gb.RunContext.CurrBlockStartTime,
			FinishedAt: gb.RunContext.CurrBlockStartTime.Add(time.Hour * 24 * 100),
		}},
		WorkType: sla.WorkHourType(gb.State.WorkType),
	})

	if getSLAInfoErr != nil {
		return getSLAInfoErr
	}

	tpl = mail.NewFormPersonExecutionNotificationTemplate(gb.RunContext.WorkNumber,
		gb.RunContext.NotifName,
		gb.RunContext.Services.Sender.SdAddress,
		gb.RunContext.Services.SLAService.ComputeMaxDateFormatted(
			gb.RunContext.CurrBlockStartTime,
			gb.State.SLA,
			slaInfoPtr,
		),
	)

	icons := []string{tpl.Image}

	iconFiles, iconErr = gb.RunContext.GetIcons(icons)
	if iconErr != nil {
		return iconErr
	}

	iconFiles = append(iconFiles, attachment.AttachmentsList...)

	if sendErr := gb.RunContext.Services.Sender.SendNotification(
		ctx, []string{emailTakenInWork},
		iconFiles,
		tpl,
	); sendErr != nil {
		return sendErr
	}

	return nil
}
