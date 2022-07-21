package pipeline

import (
	c "context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

func (gb *GoApproverBlock) setApproverDecision(ctx c.Context, sID uuid.UUID, a string, u ApproverUpdateParams) error {
	step, err := gb.Pipeline.Storage.GetTaskStepById(ctx, sID)
	if err != nil {
		return err
	} else if step == nil {
		return errors.New("can't get step from database")
	}

	// get state from step.State
	stepData, ok := step.State[gb.Name]
	if !ok {
		return errors.New("can't get step state")
	}

	var state ApproverData
	err = json.Unmarshal(stepData, &state)
	if err != nil {
		return errors.Wrap(err, "invalid format of go-approver-block state")
	}

	state.DidSLANotification = gb.State.DidSLANotification
	gb.State = &state

	err = gb.State.SetDecision(
		a,
		u.Decision,
		u.Comment,
	)
	if err != nil {
		return err
	}

	step.State[gb.Name], err = json.Marshal(gb.State)
	if err != nil {
		return err
	}

	content, err := json.Marshal(store.NewFromStep(step))
	if err != nil {
		return err
	}

	err = gb.Pipeline.Storage.UpdateStepContext(ctx, &db.UpdateStepRequest{
		Id:          sID,
		Content:     content,
		BreakPoints: step.BreakPoints,
		HasError:    false,
		Status:      step.Status,
	})
	if err != nil {
		return err
	}

	return nil
}

type setEditingAppDTO struct {
	stepId       uuid.UUID
	approver     string
	initiator    string
	workNumber   string
	workTitle    string
	updateParams updateEditingParams
}

func (gb *GoApproverBlock) setEditingApp(ctx c.Context, dto *setEditingAppDTO) error {
	step, err := gb.Pipeline.Storage.GetTaskStepById(ctx, dto.stepId)
	if err != nil {
		return err
	} else if step == nil {
		return errors.New("can't get step from database")
	}

	// get state from step.State
	stepData, ok := step.State[gb.Name]
	if !ok {
		return errors.New("can't get step state")
	}

	var state ApproverData
	err = json.Unmarshal(stepData, &state)
	if err != nil {
		return errors.Wrap(err, "invalid format of go-approver-block state")
	}

	state.DidSLANotification = gb.State.DidSLANotification
	gb.State = &state

	errSet := gb.State.SetEditingApp(dto.approver, dto.updateParams.Comment, dto.updateParams.Attachments)
	if errSet != nil {
		return errSet
	}

	step.State[gb.Name], err = json.Marshal(gb.State)
	if err != nil {
		return err
	}

	content, err := json.Marshal(store.NewFromStep(step))
	if err != nil {
		return err
	}

	err = gb.Pipeline.Storage.UpdateStepContext(ctx, &db.UpdateStepRequest{
		Id:          dto.stepId,
		Content:     content,
		BreakPoints: step.BreakPoints,
		HasError:    false,
		Status:      string(StatusWait),
	})
	if err != nil {
		return err
	}

	initiatorEmail, emailErr := gb.Pipeline.People.GetUserEmail(ctx, dto.initiator)
	if emailErr != nil {
		return emailErr
	}

	tpl := mail.NewAnswerSendToEditTemplate(dto.workNumber, dto.workTitle, gb.Pipeline.Sender.SdAddress)
	err = gb.Pipeline.Sender.SendNotification(ctx, []string{initiatorEmail}, tpl)
	if err != nil {
		return err
	}

	return nil
}

func (gb *GoApproverBlock) Update(ctx c.Context, data *script.BlockUpdateData) (interface{}, error) {
	if data == nil {
		return nil, errors.New("empty data")
	}

	if data.Action == string(entity.TaskUpdateActionApprovement) {
		var updateParams ApproverUpdateParams
		err := json.Unmarshal(data.Parameters, &updateParams)
		if err != nil {
			return nil, errors.New("can't assert provided data")
		}

		return nil, gb.setApproverDecision(ctx, data.Id, data.ByLogin, updateParams)
	}

	if data.Action == string(entity.TaskUpdateActionSendEditApp) {
		var updateParams updateEditingParams
		err := json.Unmarshal(data.Parameters, &updateParams)
		if err != nil {
			return nil, errors.New("can't assert provided data")
		}

		return nil, gb.setEditingApp(ctx, &setEditingAppDTO{
			stepId:       data.Id,
			approver:     data.ByLogin,
			initiator:    data.Author,
			workNumber:   data.WorkNumber,
			workTitle:    data.WorkTitle,
			updateParams: updateParams,
		})
	}

	if data.Action == string(entity.TaskUpdateActionCreateNewWork) {
		// TODO: make new process with edited application
		return nil, nil
	}

	return nil, errors.New("cant`t update execution block, unknown action: " + data.Action)
}
