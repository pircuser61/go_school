package pipeline

import (
	c "context"
	"encoding/json"
	"fmt"
	"time"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"github.com/google/uuid"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type updateEditingParams struct {
	Comment     string   `json:"comment"`
	Attachments []string `json:"attachments"`
}

type approverUpdateParams struct {
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

func (a *approverUpdateParams) Validate() error {
	if a.Decision != ApproverDecisionApproved && a.Decision != ApproverDecisionRejected {
		return errors.New("unknown decision")
	}

	return nil
}

func (gb *GoApproverBlock) setApproverDecision(ctx c.Context, sID uuid.UUID, login string, u approverUpdateParams) error {
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
	if err = json.Unmarshal(stepData, &state); err != nil {
		return errors.Wrap(err, "invalid format of go-approver-block state")
	}

	state.DidSLANotification = gb.State.DidSLANotification
	gb.State = &state

	if errUpdate := gb.State.SetDecision(login, u.Decision, u.Comment, u.Attachments); errUpdate != nil {
		return errUpdate
	}

	if step.State[gb.Name], err = json.Marshal(gb.State); err != nil {
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

type setActionAppDTO struct {
	stepId       uuid.UUID
	approver     string
	initiator    string
	workNumber   string
	workTitle    string
	updateParams interface{}
	action       string
}

//nolint:gocyclo //its ok here
func (gb *GoApproverBlock) setEditApplication(ctx c.Context, dto *setActionAppDTO) error {
	step, err := gb.Pipeline.Storage.GetTaskStepById(ctx, dto.stepId)
	if err != nil {
		return err
	}

	if step == nil {
		return errors.New("can't get step from database")
	}

	// get state from step.State
	stepData, ok := step.State[gb.Name]
	if !ok {
		return errors.New("can't get step state")
	}

	var state ApproverData
	if err = json.Unmarshal(stepData, &state); err != nil {
		return errors.Wrap(err, "invalid format of go-approver-block state")
	}

	state.DidSLANotification = gb.State.DidSLANotification
	gb.State = &state

	params, ok := dto.updateParams.(updateEditingParams)
	if !ok {
		return errors.New("can't convert to updateEditingParams")
	}
	errSet := gb.State.setEditApp(dto.approver, params)
	if errSet != nil {
		return errSet
	}

	if step.State[gb.Name], err = json.Marshal(gb.State); err != nil {
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
		Status:      string(StatusIdle),
	})
	if err != nil {
		return err
	}

	initiatorEmail, emailErr := gb.Pipeline.People.GetUserEmail(ctx, dto.initiator)
	if emailErr != nil {
		return emailErr
	}

	tpl := mail.NewAnswerSendToEditTemplate(dto.workNumber, dto.workTitle, gb.Pipeline.Sender.SdAddress)
	err = gb.Pipeline.Sender.SendNotification(ctx, []string{initiatorEmail}, nil, tpl)
	if err != nil {
		return err
	}

	return nil
}

//nolint:gocyclo //ok
func (gb *GoApproverBlock) updateRequestApproverInfo(ctx c.Context, byLogin string, data *script.BlockUpdateData) (err error) {
	step, err := gb.Pipeline.Storage.GetTaskStepById(ctx, data.Id)
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
	if err = json.Unmarshal(stepData, &state); err != nil {
		return errors.Wrap(err, "invalid format of go-approver-block state")
	}

	gb.State = &state

	var updateParams requestInfoParams
	if err = json.Unmarshal(data.Parameters, &updateParams); err != nil {
		return errors.New("can't assert provided update requestApproverInfo data")
	}

	if gb.State.Decision != nil {
		return errors.New("decision already set")
	}

	var (
		id     = uuid.NewString()
		linkId *string
	)

	status := string(StatusIdle)
	var tpl mail.Template

	if updateParams.Type == RequestAddInfoType {
		_, ok = gb.State.Approvers[byLogin]
		if !ok && byLogin != AutoApprover {
			return fmt.Errorf("%s not found in approvers", byLogin)
		}

		authorEmail, emailErr := gb.Pipeline.People.GetUserEmail(ctx, data.Author)
		if emailErr != nil {
			return emailErr
		}

		tpl = mail.NewRequestApproverInfoTemplate(data.WorkNumber, data.WorkTitle, gb.Pipeline.Sender.SdAddress)
		if err = gb.Pipeline.Sender.SendNotification(ctx, []string{authorEmail}, nil, tpl); err != nil {
			return err
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

		status = string(StatusRunning)

		if len(gb.State.RequestApproverInfoLog) > 0 {
			workHours := getWorkWorkHoursBetweenDates(
				gb.State.RequestApproverInfoLog[len(gb.State.RequestApproverInfoLog)-1].CreatedAt,
				time.Now(),
			)
			gb.State.IncreaseSLA(workHours)
		}

		tpl = mail.NewAnswerApproverInfoTemplate(data.WorkNumber, data.WorkTitle, gb.Pipeline.Sender.SdAddress)

		approverEmail, emailErr := gb.Pipeline.People.GetUserEmail(ctx, byLogin)
		if emailErr != nil {
			return emailErr
		}

		err = gb.Pipeline.Sender.SendNotification(ctx, []string{approverEmail}, nil, tpl)
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
		Login:       byLogin,
		CreatedAt:   time.Now(),
	})

	step.State[gb.Name], err = json.Marshal(gb.State)
	if err != nil {
		return err
	}

	var content []byte
	content, err = json.Marshal(store.NewFromStep(step))
	if err != nil {
		return err
	}

	err = gb.Pipeline.Storage.UpdateStepContext(ctx, &db.UpdateStepRequest{
		Id:          data.Id,
		Content:     content,
		BreakPoints: step.BreakPoints,
		Status:      status,
	})
	if err != nil {
		return err
	}

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

func (gb *GoApproverBlock) Update(ctx c.Context, data *script.BlockUpdateData) (interface{}, error) {
	if data == nil {
		return nil, errors.New("empty data")
	}

	switch data.Action {
	case string(entity.TaskUpdateActionApprovement):
		var updateParams approverUpdateParams

		if err := json.Unmarshal(data.Parameters, &updateParams); err != nil {
			return nil, errors.New("can't assert provided data")
		}

		return nil, gb.setApproverDecision(ctx, data.Id, data.ByLogin, updateParams)

	case string(entity.TaskUpdateActionSendEditApp):
		var updateParams updateEditingParams

		if err := json.Unmarshal(data.Parameters, &updateParams); err != nil {
			return nil, errors.New("can't assert provided data")
		}

		return nil, gb.setEditApplication(ctx, &setActionAppDTO{
			stepId:       data.Id,
			approver:     data.ByLogin,
			initiator:    data.Author,
			workNumber:   data.WorkNumber,
			workTitle:    data.WorkTitle,
			updateParams: updateParams,
			action:       data.Action,
		})

	case string(entity.TaskUpdateActionRequestApproveInfo):
		var updateParams requestInfoParams

		if err := json.Unmarshal(data.Parameters, &updateParams); err != nil {
			return nil, errors.New("can't assert provided data")
		}

		return nil, gb.updateRequestApproverInfo(ctx, data.ByLogin, data)

	case string(entity.TaskUpdateActionCancelApp):
		step, err := gb.Pipeline.Storage.GetTaskStepById(ctx, data.Id)
		if err != nil {
			return nil, err
		}

		if step == nil {
			return nil, errors.New("can't get step from database")
		}
		if errUpdate := gb.approverCancelPipeline(ctx, data, step); errUpdate != nil {
			return nil, errUpdate
		}
		return nil, nil
	}

	return nil, errors.New("cant`t update execution block, unknown action: " + data.Action)
}

type setEditingAppLogDTO struct {
	id       uuid.UUID
	runCtx   *store.VariableStore
	workID   uuid.UUID
	stepName string
}

func (gb *GoApproverBlock) setEditingAppLogFromPreviousBlock(ctx c.Context, dto *setEditingAppLogDTO) {
	l := logger.GetLogger(ctx)

	var step *entity.Step
	var parentStep *entity.Step
	var err error

	step, err = gb.Pipeline.Storage.GetTaskStepById(ctx, dto.id)
	if err != nil {
		l.Error(err)
		return
	}

	parentStep, err = gb.Pipeline.Storage.GetParentTaskStepByName(ctx, dto.workID, dto.stepName)
	if err != nil {
		l.Error(err)
		return
	} else if parentStep == nil {
		l.Error("setEditingAppLogFromPreviousBlock: step is nil")
		return
	}

	// get state from step.State
	data, ok := parentStep.State[dto.stepName]
	if !ok {
		l.Error("setEditingAppLogFromPreviousBlock: step state is not found: " + dto.stepName)
		return
	}

	var parentState ApproverData
	err = json.Unmarshal(data, &parentState)
	if err != nil {
		l.Error("setEditingAppLogFromPreviousBlock: invalid format of go-approver-block state")
		return
	}

	if len(parentState.EditingAppLog) > 0 {
		gb.State.EditingAppLog = parentState.EditingAppLog

		step.State[gb.Name], err = json.Marshal(gb.State)
		if err != nil {
			l.Error(err)
			return
		}

		var stateBytes []byte
		stateBytes, err = json.Marshal(store.NewFromStep(step))
		if err != nil {
			l.Error("setEditingAppLogFromPreviousBlock: ", err)
			return
		}

		err = gb.Pipeline.Storage.UpdateStepContext(ctx, &db.UpdateStepRequest{
			Id:          dto.id,
			Content:     stateBytes,
			BreakPoints: step.BreakPoints,
			Status:      step.Status,
		})
		if err != nil {
			l.Error("setEditingAppLogFromPreviousBlock.UpdateStepContext: ", err)
			return
		}

		dto.runCtx.ReplaceState(gb.Name, stateBytes)
	}
}

func (gb *GoApproverBlock) approverCancelPipeline(ctx c.Context, in *script.BlockUpdateData, step *entity.Step) (err error) {
	gb.State.IsRevoked = true

	if step.State[gb.Name], err = json.Marshal(gb.State); err != nil {
		return err
	}
	var content []byte
	if content, err = json.Marshal(store.NewFromStep(step)); err != nil {
		return err
	}
	err = gb.Pipeline.Storage.UpdateStepContext(ctx, &db.UpdateStepRequest{
		Id:          in.Id,
		Content:     content,
		BreakPoints: step.BreakPoints,
		Status:      string(StatusCancel),
	})

	return err
}
