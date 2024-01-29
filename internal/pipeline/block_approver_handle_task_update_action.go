package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

//nolint:gocognit,gocyclo //тут большой switch case, где нибудь но он должен быть
func (gb *GoApproverBlock) handleTaskUpdateAction(ctx context.Context) error {
	data := gb.RunContext.UpdateData
	if data == nil {
		return errors.New("empty data")
	}

	gb.RunContext.Delegations = gb.RunContext.Delegations.FilterByType("approvement")

	//nolint:exhaustive //нам не нужна обработка всех возможных TaskUpdateAction
	switch entity.TaskUpdateAction(data.Action) {
	case entity.TaskUpdateActionSLABreach:
		if errUpdate := gb.handleBreachedSLA(ctx); errUpdate != nil {
			return errUpdate
		}
	case entity.TaskUpdateActionHalfSLABreach:
		if errUpdate := gb.handleHalfBreachedSLA(ctx); errUpdate != nil {
			return errUpdate
		}
	case entity.TaskUpdateActionReworkSLABreach:
		if errUpdate := gb.handleReworkSLABreached(ctx); errUpdate != nil {
			return errUpdate
		}
	case entity.TaskUpdateActionApprovement:
		var updateParams approverUpdateParams

		if err := json.Unmarshal(data.Parameters, &updateParams); err != nil {
			return errors.New("can't assert provided data")
		}

		if !gb.actionAcceptable(updateParams.Decision) {
			return errors.New("unacceptable action")
		}

		login := gb.RunContext.UpdateData.ByLogin
		if login == ServiceAccount || login == ServiceAccountStage || login == ServiceAccountDev {
			gb.RunContext.UpdateData.ByLogin = updateParams.Username
		}

		updateParams.internalDecision = updateParams.Decision.ToDecision()

		if errUpdate := gb.setApproveDecision(ctx, &updateParams); errUpdate != nil {
			return errUpdate
		}

	case entity.TaskUpdateActionAdditionalApprovement:
		var updateParams additionalApproverUpdateParams

		if err := json.Unmarshal(data.Parameters, &updateParams); err != nil {
			return fmt.Errorf("can't assert provided data: %v", err)
		}

		if err := updateParams.Validate(); err != nil {
			return err
		}

		loginsToNotify, err := gb.State.SetDecisionByAdditionalApprover(gb.RunContext.UpdateData.ByLogin,
			updateParams, gb.RunContext.Delegations)
		if err != nil {
			return err
		}

		loginsToNotify = append(loginsToNotify, gb.RunContext.Initiator)

		err = gb.notifyDecisionMadeByAdditionalApprover(ctx, loginsToNotify)
		if err != nil {
			return err
		}

	case entity.TaskUpdateActionApproverSendEditApp:
		var updateParams approverUpdateEditingParams

		if err := json.Unmarshal(data.Parameters, &updateParams); err != nil {
			return errors.New("can't assert provided data")
		}

		if errUpdate := gb.toEditApplication(ctx, updateParams); errUpdate != nil {
			return errUpdate
		}

	case entity.TaskUpdateActionRequestApproveInfo:
		if errUpdate := gb.updateRequestApproverInfo(ctx); errUpdate != nil {
			return errUpdate
		}

	case entity.TaskUpdateActionReplyApproverInfo:
		if errUpdate := gb.updateReplyApproverInfo(ctx); errUpdate != nil {
			return errUpdate
		}

	case entity.TaskUpdateActionAddApprovers:
		var updateParams addApproversParams
		if err := json.Unmarshal(data.Parameters, &updateParams); err != nil {
			return errors.New("can't assert provided data")
		}

		if errUpdate := gb.addApprovers(ctx, updateParams); errUpdate != nil {
			return errUpdate
		}

	case entity.TaskUpdateActionDayBeforeSLARequestAddInfo:
		if errUpdate := gb.handleBreachedDayBeforeSLARequestAddInfo(ctx); errUpdate != nil {
			return errUpdate
		}
	case entity.TaskUpdateActionSLABreachRequestAddInfo:
		if errUpdate := gb.HandleBreachedSLARequestAddInfo(ctx); errUpdate != nil {
			return errUpdate
		}
	}

	return nil
}
