package pipeline

import (
	c "context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
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

// nolint:dupl // another action
func (gb *GoFormBlock) cancelPipeline(ctx c.Context) error {
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

	stateBytes, err := json.Marshal(gb.State)
	if err != nil {
		return err
	}

	gb.RunContext.VarStore.ReplaceState(gb.Name, stateBytes)
	return nil
}

//nolint:gocyclo //ok
func (gb *GoFormBlock) Update(ctx c.Context) (interface{}, error) {
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
	case string(entity.TaskUpdateActionCancelApp):
		if errUpdate := gb.cancelPipeline(ctx); errUpdate != nil {
			return nil, errUpdate
		}
		return nil, nil
	case string(entity.TaskUpdateActionRequestFillForm):
		if errFill := gb.handleRequestFillForm(ctx, data); errFill != nil {
			return nil, errFill
		}
	case string(entity.TaskUpdateActionFormExecutorStartWork):
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
		isAllowed, checkEditErr := gb.RunContext.Storage.CheckUserCanEditForm(ctx, gb.RunContext.WorkNumber,
			gb.Name, data.ByLogin)
		if checkEditErr != nil {
			return checkEditErr
		}
		if !isAllowed {
			return NewUserIsNotPartOfProcessErr()
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

	personData, err := gb.RunContext.ServiceDesc.GetSsoPerson(ctx, *gb.State.ActualExecutor)
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
			executorEmail, err := gb.RunContext.People.GetUserEmail(ctx, logins[i])
			if err != nil {
				log.WithError(err).Warning(fn, fmt.Sprintf("executor login %s not found", logins[i]))
				continue
			}
			emails = append(emails, executorEmail)
		}

		if len(emails) == 0 {
			return nil
		}

		err := gb.RunContext.Sender.SendNotification(
			ctx,
			emails,
			nil,
			mail.NewFormSLATpl(
				gb.RunContext.WorkNumber,
				gb.RunContext.WorkTitle,
				gb.RunContext.Sender.SdAddress,
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
			executorEmail, err := gb.RunContext.People.GetUserEmail(ctx, logins[i])
			if err != nil {
				log.WithError(err).Warning(fn, fmt.Sprintf("executor login %s not found", logins[i]))
				continue
			}
			emails = append(emails, executorEmail)
		}

		if len(emails) == 0 {
			return nil
		}

		err := gb.RunContext.Sender.SendNotification(
			ctx,
			emails,
			nil,
			mail.NewFormDayHalfSLATpl(
				gb.RunContext.WorkNumber,
				gb.RunContext.WorkTitle,
				gb.RunContext.Sender.SdAddress,
			))
		if err != nil {
			return err
		}
	}

	gb.State.HalfSLAChecked = true

	return nil
}

func (gb *GoFormBlock) formExecutorStartWork(_ c.Context) (err error) {
	if gb.State.IsTakenInWork {
		return nil
	}
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
	workHours := getWorkHoursBetweenDates(
		gb.RunContext.currBlockStartTime,
		time.Now(),
		nil,
	)
	gb.State.IncreaseSLA(workHours)

	//if err = gb.emailGroupExecutors(ctx, gb.RunContext.UpdateData.ByLogin, executorLogins); err != nil {
	//	return nil
	//}

	return nil
}

func (a *FormData) IncreaseSLA(addSla int) {
	a.SLA += addSla
}

func (a *FormData) GetIsEditable() bool {
	return !a.IsTakenInWork
}
