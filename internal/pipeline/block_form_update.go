package pipeline

import (
	c "context"
	"encoding/json"
	"fmt"
	"time"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sla"
)

type updateFillFormParams struct {
	Description     string                 `json:"description"`
	ApplicationBody map[string]interface{} `json:"application_body"`
	BlockId         string                 `json:"block_id"`
}

func (a *updateFillFormParams) Validate() error {
	if a.BlockId == "" || a.Description == "" || len(a.ApplicationBody) == 0 {
		return errors.New("empty form data")
	}

	return nil
}

//nolint:gocyclo //ok
func (gb *GoFormBlock) Update(ctx c.Context) (interface{}, error) {
	updateInOtherBlocks := false
	data := gb.RunContext.UpdateData
	if data == nil {
		return nil, errors.New("empty data")
	}

	switch data.Action {
	case string(entity.TaskUpdateActionSLABreach):
		if errUpdate := gb.handleBreachedSLA(ctx); errUpdate != nil {
			return nil, errUpdate
		}
	case string(entity.TaskUpdateActionHalfSLABreach):
		if errUpdate := gb.handleHalfSLABreached(ctx); errUpdate != nil {
			return nil, errUpdate
		}
	case string(entity.TaskUpdateActionRequestFillForm):
		if !gb.State.IsTakenInWork {
			return nil, errors.New("is not taken in work")
		}
		if errFill := gb.handleRequestFillForm(ctx, data); errFill != nil {
			return nil, errFill
		}
		updateInOtherBlocks = true
	case string(entity.TaskUpdateActionFormExecutorStartWork):
		if gb.State.IsTakenInWork {
			return nil, errors.New("is already taken in work")
		}
		if errUpdate := gb.formExecutorStartWork(ctx); errUpdate != nil {
			return nil, errUpdate
		}
	}

	var stateBytes []byte
	stateBytes, err := json.Marshal(gb.State)
	if err != nil {
		return nil, err
	}

	gb.RunContext.VarStore.ReplaceState(gb.Name, stateBytes)

	if len(gb.State.ApplicationBody) > 0 {
		if _, ok := gb.expectedEvents[eventEnd]; ok {
			status, _ := gb.GetTaskHumanStatus()
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

	if updateInOtherBlocks {
		taskId := gb.RunContext.TaskID.String()
		err = gb.RunContext.Services.Storage.UpdateBlockStateInOthers(ctx, gb.Name, taskId, stateBytes)
		if err != nil {
			return nil, err
		}

		executor, _ := gb.RunContext.VarStore.GetValue(gb.Output[keyOutputFormExecutor])
		body, _ := gb.RunContext.VarStore.GetValue(gb.Output[keyOutputFormBody])

		blockValues := map[string]interface{}{
			gb.Name + ".executor":         executor,
			gb.Name + ".application_body": body,
		}

		errBlockVariables := gb.RunContext.Services.Storage.UpdateBlockVariablesInOthers(ctx, taskId, blockValues)
		if errBlockVariables != nil {
			return nil, errBlockVariables
		}
	}

	return nil, nil
}

func (gb *GoFormBlock) handleRequestFillForm(ctx c.Context, data *script.BlockUpdateData) error {
	var updateParams updateFillFormParams
	err := json.Unmarshal(data.Parameters, &updateParams)
	if err != nil {
		return errors.New("can't assert provided data")
	}

	if updateParams.BlockId != gb.Name {
		return fmt.Errorf("wrong form id: %s, gb.Name: %s", updateParams.BlockId, gb.Name)
	}

	if gb.State.IsFilled {
		isAllowed, checkEditErr := gb.RunContext.Services.Storage.CheckUserCanEditForm(ctx, gb.RunContext.WorkNumber,
			gb.Name, data.ByLogin)
		if checkEditErr != nil {
			return checkEditErr
		}
		if !isAllowed {
			return NewUserIsNotPartOfProcessErr()
		}

		if gb.State.ActualExecutor != nil && *gb.State.ActualExecutor == AutoFillUser {
			gb.State.ActualExecutor = &data.ByLogin
		}
	} else {
		_, executorFound := gb.State.Executors[data.ByLogin]

		if !executorFound {
			return NewUserIsNotPartOfProcessErr()
		}

		gb.State.ActualExecutor = &data.ByLogin
		gb.State.IsFilled = true
	}

	gb.State.ApplicationBody = updateParams.ApplicationBody
	gb.State.Description = updateParams.Description

	gb.State.ChangesLog = append([]ChangesLogItem{
		{
			Description:     updateParams.Description,
			ApplicationBody: updateParams.ApplicationBody,
			CreatedAt:       time.Now(),
			Executor:        data.ByLogin,
			DelegateFor:     "",
		},
	}, gb.State.ChangesLog...)

	personData, err := gb.RunContext.Services.ServiceDesc.GetSsoPerson(ctx, *gb.State.ActualExecutor)
	if err != nil {
		return err
	}

	gb.RunContext.VarStore.SetValue(gb.Output[keyOutputFormExecutor], personData)
	gb.RunContext.VarStore.SetValue(gb.Output[keyOutputFormBody], gb.State.ApplicationBody)

	return nil
}

//nolint:dupl //its not duplicate
func (gb *GoFormBlock) handleBreachedSLA(ctx c.Context) error {
	const fn = "pipeline.form.handleBreachedSLA"

	if !gb.State.CheckSLA {
		gb.State.SLAChecked = true
		return nil
	}

	log := logger.GetLogger(ctx)

	if gb.State.SLA >= 8 {
		emails := make([]string, 0, len(gb.State.Executors))
		logins := getSliceFromMapOfStrings(gb.State.Executors)

		for i := range logins {
			executorEmail, err := gb.RunContext.Services.People.GetUserEmail(ctx, logins[i])
			if err != nil {
				log.WithError(err).Warning(fn, fmt.Sprintf("executor login %s not found", logins[i]))
				continue
			}
			emails = append(emails, executorEmail)
		}

		if len(emails) == 0 {
			return nil
		}
		err := gb.RunContext.Services.Sender.SendNotification(
			ctx,
			emails,
			nil,
			mail.NewFormSLATpl(
				gb.RunContext.WorkNumber,
				gb.RunContext.NotifName,
				gb.RunContext.Services.Sender.SdAddress,
			))
		if err != nil {
			return err
		}
	}

	gb.State.HalfSLAChecked = true
	gb.State.SLAChecked = true

	return nil
}

//nolint:dupl //its not duplicate
func (gb *GoFormBlock) handleHalfSLABreached(ctx c.Context) error {
	const fn = "pipeline.form.handleHalfSLABreached"

	if gb.State.HalfSLAChecked {
		return nil
	}

	log := logger.GetLogger(ctx)

	if gb.State.SLA >= 8 {
		emails := make([]string, 0, len(gb.State.Executors))
		logins := getSliceFromMapOfStrings(gb.State.Executors)

		for i := range logins {
			executorEmail, err := gb.RunContext.Services.People.GetUserEmail(ctx, logins[i])
			if err != nil {
				log.WithError(err).Warning(fn, fmt.Sprintf("executor login %s not found", logins[i]))
				continue
			}
			emails = append(emails, executorEmail)
		}

		if len(emails) == 0 {
			return nil
		}
		err := gb.RunContext.Services.Sender.SendNotification(
			ctx,
			emails,
			nil,
			mail.NewFormDayHalfSLATpl(
				gb.RunContext.WorkNumber,
				gb.RunContext.NotifName,
				gb.RunContext.Services.Sender.SdAddress,
			))
		if err != nil {
			return err
		}
	}

	gb.State.HalfSLAChecked = true

	return nil
}

func (gb *GoFormBlock) formExecutorStartWork(ctx c.Context) (err error) {
	var currentLogin = gb.RunContext.UpdateData.ByLogin
	_, executorFound := gb.State.Executors[currentLogin]

	_, isDelegate := gb.RunContext.Delegations.FindDelegatorFor(currentLogin, getSliceFromMapOfStrings(gb.State.Executors))
	if !(executorFound || isDelegate) {
		return NewUserIsNotPartOfProcessErr()
	}

	executorLogins := make(map[string]struct{}, 0)
	for i := range gb.State.Executors {
		executorLogins[i] = gb.State.Executors[i]
	}

	gb.State.Executors = map[string]struct{}{
		gb.RunContext.UpdateData.ByLogin: {},
	}

	gb.State.IsTakenInWork = true

	slaInfoPtr, getSlaInfoErr := gb.RunContext.Services.SLAService.GetSLAInfoPtr(ctx, sla.InfoDto{
		TaskCompletionIntervals: []entity.TaskCompletionInterval{{StartedAt: gb.RunContext.CurrBlockStartTime,
			FinishedAt: gb.RunContext.CurrBlockStartTime.Add(time.Hour * 24 * 100)}},
		WorkType: sla.WorkHourType(gb.State.WorkType),
	})

	if getSlaInfoErr != nil {
		return getSlaInfoErr
	}
	workHours := gb.RunContext.Services.SLAService.GetWorkHoursBetweenDates(
		gb.RunContext.CurrBlockStartTime,
		time.Now(),
		slaInfoPtr,
	)
	gb.State.IncreaseSLA(workHours)

	if err = gb.emailGroupExecutors(ctx, gb.RunContext.UpdateData.ByLogin, executorLogins); err != nil {
		return nil
	}

	return nil
}

func (a *FormData) IncreaseSLA(addSla int) {
	a.SLA += addSla
}

func (gb *GoFormBlock) emailGroupExecutors(ctx c.Context, loginTakenInWork string, logins map[string]struct{}) (err error) {
	executors := getSliceFromMapOfStrings(logins)

	emails := make([]string, 0, len(executors))
	for _, login := range executors {
		if login != loginTakenInWork {
			email, emailErr := gb.RunContext.Services.People.GetUserEmail(ctx, login)
			if emailErr != nil {
				return emailErr
			}

			emails = append(emails, email)
		}
	}

	formExecutorSSOPerson, getUserErr := gb.RunContext.Services.People.GetUser(ctx, loginTakenInWork)

	if getUserErr != nil {
		return getUserErr
	}

	typedSSOPerson, convertErr := formExecutorSSOPerson.ToUserinfo()
	if convertErr != nil {
		return convertErr
	}

	tpl := mail.NewFormExecutionTakenInWorkTpl(gb.RunContext.WorkNumber,
		gb.RunContext.NotifName,
		typedSSOPerson.FullName,
		gb.RunContext.Services.Sender.SdAddress,
	)

	if errSend := gb.RunContext.Services.Sender.SendNotification(ctx, emails, nil, tpl); errSend != nil {
		return errSend
	}

	emailTakenInWork, emailErr := gb.RunContext.Services.People.GetUserEmail(ctx, loginTakenInWork)
	if emailErr != nil {
		return emailErr
	}

	slaInfoPtr, getSlaInfoErr := gb.RunContext.Services.SLAService.GetSLAInfoPtr(ctx, sla.InfoDto{
		TaskCompletionIntervals: []entity.TaskCompletionInterval{{StartedAt: gb.RunContext.CurrBlockStartTime,
			FinishedAt: gb.RunContext.CurrBlockStartTime.Add(time.Hour * 24 * 100)}},
		WorkType: sla.WorkHourType(gb.State.WorkType),
	})

	if getSlaInfoErr != nil {
		return getSlaInfoErr
	}
	tpl = mail.NewFormPersonExecutionNotificationTemplate(gb.RunContext.WorkNumber,
		gb.RunContext.NotifName,
		gb.RunContext.Services.Sender.SdAddress,
		gb.RunContext.Services.SLAService.ComputeMaxDateFormatted(gb.RunContext.CurrBlockStartTime, gb.State.SLA,
			slaInfoPtr),
	)

	if sendErr := gb.RunContext.Services.Sender.SendNotification(ctx, []string{emailTakenInWork}, nil,
		tpl); sendErr != nil {
		return sendErr
	}

	return nil
}
