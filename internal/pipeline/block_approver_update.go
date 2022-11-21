package pipeline

import (
	c "context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
)

type approverUpdateEditingParams struct {
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

func (gb *GoApproverBlock) setApproverDecision(u approverUpdateParams) error {
	if errUpdate := gb.State.SetDecision(gb.RunContext.UpdateData.ByLogin, u.Decision, u.Comment, u.Attachments); errUpdate != nil {
		return errUpdate
	}

	if gb.State.Decision != nil {
		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputApprover], &gb.State.ActualApprover)
		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputDecision], gb.State.Decision.String())
		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputComment], gb.State.Comment)
	}
	return nil
}

func (gb *GoApproverBlock) handleBreachedSLA(ctx c.Context) error {
	if gb.State.SLA > 8 {
		emails := make([]string, 0, len(gb.State.Approvers))
		for approver := range gb.State.Approvers {
			email, err := gb.RunContext.People.GetUserEmail(ctx, approver)
			if err != nil {
				continue
			}
			emails = append(emails, email)
		}
		if len(emails) == 0 {
			return nil
		}
		err := gb.RunContext.Sender.SendNotification(ctx, emails, nil,
			mail.NewApprovementSLATemplate(gb.RunContext.WorkNumber, gb.RunContext.WorkTitle, gb.RunContext.Sender.SdAddress))
		if err != nil {
			return err
		}
	}
	if gb.State.AutoAction != nil {
		gb.RunContext.UpdateData.ByLogin = AutoApprover
		if setErr := gb.setApproverDecision(
			approverUpdateParams{
				Decision: decisionFromAutoAction(*gb.State.AutoAction),
				Comment:  AutoActionComment,
			}); setErr != nil {
			return setErr
		}
	}

	gb.State.SLAChecked = true
	return nil
}

//nolint:gocyclo //its ok here
func (gb *GoApproverBlock) setEditApplication(ctx c.Context, updateParams approverUpdateEditingParams) error {
	errSet := gb.State.setEditApp(gb.RunContext.UpdateData.ByLogin, updateParams)
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
func (gb *GoApproverBlock) updateRequestApproverInfo(ctx c.Context) (err error) {
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
		_, ok := gb.State.Approvers[gb.RunContext.UpdateData.ByLogin]
		if !ok && gb.RunContext.UpdateData.ByLogin != AutoApprover {
			return fmt.Errorf("%s not found in approvers", gb.RunContext.UpdateData.ByLogin)
		}

		authorEmail, emailErr := gb.RunContext.People.GetUserEmail(ctx, gb.RunContext.Initiator)
		if emailErr != nil {
			return emailErr
		}

		tpl := mail.NewRequestApproverInfoTemplate(gb.RunContext.WorkNumber, gb.RunContext.WorkTitle, gb.RunContext.Sender.SdAddress)
		if err = gb.RunContext.Sender.SendNotification(ctx, []string{authorEmail}, nil, tpl); err != nil {
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

func (gb *GoApproverBlock) Update(ctx c.Context) (interface{}, error) {
	data := gb.RunContext.UpdateData
	if data == nil {
		return nil, errors.New("empty data")
	}

	switch data.Action {
	case string(entity.TaskUpdateActionSLABreach):
		if errUpdate := gb.handleBreachedSLA(ctx); errUpdate != nil {
			return nil, errUpdate
		}

	case string(entity.TaskUpdateActionApprovement):
		var updateParams approverUpdateParams

		if err := json.Unmarshal(data.Parameters, &updateParams); err != nil {
			return nil, errors.New("can't assert provided data")
		}

		if errUpdate := gb.setApproverDecision(updateParams); errUpdate != nil {
			return nil, errUpdate
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
	gb.State.IsRevoked = true
	if stopErr := gb.RunContext.Storage.StopTaskBlocks(ctx, gb.RunContext.Tx, gb.RunContext.TaskID); stopErr != nil {
		return stopErr
	}
	return nil
}
