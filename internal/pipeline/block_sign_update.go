package pipeline

import (
	c "context"
	"encoding/json"
	"errors"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
)

type signSignatureParams struct {
	Decision    SignDecision        `json:"decision"`
	Comment     string              `json:"comment,omitempty"`
	Attachments []entity.Attachment `json:"attachments"`
}

func (gb *GoSignBlock) handleSignature() error {
	updateParams := &signSignatureParams{}

	err := json.Unmarshal(gb.RunContext.UpdateData.Parameters, updateParams)
	if err != nil {
		return errors.New("can't assert provided update data")
	}

	if setErr := gb.setSignerDecision(updateParams); setErr != nil {
		return setErr
	}
	return nil
}

func (gb *GoSignBlock) Update(ctx c.Context) (interface{}, error) {
	data := gb.RunContext.UpdateData
	if data == nil {
		return nil, errors.New("empty data")
	}

	//nolint:gocritic //for future actions
	switch data.Action {
	case string(entity.TaskUpdateActionSLABreach):
		if errUpdate := gb.handleBreachedSLA(ctx); errUpdate != nil {
			return nil, errUpdate
		}
	case string(entity.TaskUpdateActionDayBeforeSLABreach):
		if errUpdate := gb.handleNotifications(ctx); errUpdate != nil {
			return nil, errUpdate
		}
	case string(entity.TaskUpdateActionSign):
		if errUpdate := gb.handleSignature(); errUpdate != nil {
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

//nolint:dupl,gocyclo //its not duplicate
func (gb *GoSignBlock) handleBreachedSLA(ctx c.Context) error {
	if gb.State.CheckSLA == nil || !*gb.State.CheckSLA {
		gb.State.SLAChecked = true
		return nil
	}

	if gb.State.SLA != nil {
		emails := make([]string, 0, len(gb.State.Signers))
		logins := getSliceFromMapOfStrings(gb.State.Signers)

		for i := range logins {
			eml, err := gb.RunContext.People.GetUserEmail(ctx, logins[i])
			if err != nil {
				continue
			}
			emails = append(emails, eml)
		}

		if len(emails) == 0 {
			return nil
		}
		err := gb.RunContext.Sender.SendNotification(ctx, emails, nil,
			mail.NewSignSLAExpiredTemplate(
				gb.RunContext.WorkNumber,
				gb.RunContext.WorkTitle,
				gb.RunContext.Sender.SdAddress,
			),
		)
		if err != nil {
			return err
		}
	}

	if gb.State.AutoReject != nil && *gb.State.AutoReject {
		gb.RunContext.UpdateData.ByLogin = autoSigner
		if setErr := gb.setSignerDecision(
			&signSignatureParams{
				Decision: SignDecisionRejected,
				Comment:  AutoActionComment,
			}); setErr != nil {
			return setErr
		}
	}

	gb.State.SLAChecked = true

	return nil
}

func (gb *GoSignBlock) setSignerDecision(u *signSignatureParams) error {
	if errUpdate := gb.State.SetDecision(gb.RunContext.UpdateData.ByLogin, u); errUpdate != nil {
		return errUpdate
	}

	if gb.State.Decision != nil {
		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputSigner], gb.State.ActualSigner)
		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputSignDecision], gb.State.Decision)
		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputSignComment], gb.State.Comment)
		resAttachments := make([]entity.Attachment, 0)
		for _, l := range gb.State.SignLog {
			resAttachments = append(resAttachments, l.Attachments...)
		}
		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputSignAttachments], resAttachments)
	}

	return nil
}
