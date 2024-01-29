package pipeline

import (
	"context"

	"github.com/pkg/errors"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

func (gb *GoExecutionBlock) handleTaskUpdateAction(ctx context.Context) error {
	data := gb.RunContext.UpdateData
	if data == nil {
		return errors.New("empty data")
	}

	gb.RunContext.Delegations = gb.RunContext.Delegations.FilterByType("execution")

	err := gb.handleAction(ctx, entity.TaskUpdateAction(data.Action))
	if err != nil {
		return err
	}

	return nil
}

//nolint:gocognit,gocyclo // вся сложность функции состоит в switch case, под каждым вызывается одна-две функции
func (gb *GoExecutionBlock) handleAction(ctx context.Context, action entity.TaskUpdateAction) error {
	//nolint:exhaustive //нам не нужно обрабатывать остальные случаи
	switch action {
	case entity.TaskUpdateActionSLABreach:
		errUpdate := gb.handleBreachedSLA(ctx)
		if errUpdate != nil {
			return errUpdate
		}
	case entity.TaskUpdateActionHalfSLABreach:
		gb.handleHalfSLABreached(ctx)
	case entity.TaskUpdateActionReworkSLABreach:
		errUpdate := gb.handleReworkSLABreached(ctx)
		if errUpdate != nil {
			return errUpdate
		}
	case entity.TaskUpdateActionExecution:
		if !gb.State.IsTakenInWork {
			return errors.New("is not taken in work")
		}

		errUpdate := gb.updateDecision(ctx)
		if errUpdate != nil {
			return errUpdate
		}
	case entity.TaskUpdateActionChangeExecutor:
		if !gb.State.IsTakenInWork {
			return errors.New("is not taken in work")
		}

		errUpdate := gb.changeExecutor(ctx)
		if errUpdate != nil {
			return errUpdate
		}
	case entity.TaskUpdateActionRequestExecutionInfo:
		if !gb.State.IsTakenInWork {
			return errors.New("is not taken in work")
		}

		errUpdate := gb.updateRequestInfo(ctx)
		if errUpdate != nil {
			return errUpdate
		}
	case entity.TaskUpdateActionReplyExecutionInfo:
		if !gb.State.IsTakenInWork {
			return errors.New("is not taken in work")
		}

		errUpdate := gb.updateReplyInfo(ctx)
		if errUpdate != nil {
			return errUpdate
		}
	case entity.TaskUpdateActionExecutorStartWork:
		if gb.State.IsTakenInWork {
			return errors.New("is already taken in work")
		}

		errUpdate := gb.executorStartWork(ctx)
		if errUpdate != nil {
			return errUpdate
		}
	case entity.TaskUpdateActionExecutorSendEditApp:
		if !gb.State.IsTakenInWork {
			return errors.New("is not taken in work")
		}

		errUpdate := gb.toEditApplication(ctx)
		if errUpdate != nil {
			return errUpdate
		}
	case entity.TaskUpdateActionDayBeforeSLARequestAddInfo:
		errUpdate := gb.handleBreachedDayBeforeSLARequestAddInfo(ctx)
		if errUpdate != nil {
			return errUpdate
		}
	case entity.TaskUpdateActionSLABreachRequestAddInfo:
		errUpdate := gb.HandleBreachedSLARequestAddInfo(ctx)
		if errUpdate != nil {
			return errUpdate
		}
	}

	return nil
}
