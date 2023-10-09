package pipeline

import (
	c "context"
	"encoding/json"
	"errors"
	"time"

	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/scheduler"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

type signSignatureParams struct {
	Decision    SignDecision        `json:"decision"`
	Comment     string              `json:"comment,omitempty"`
	Attachments []entity.Attachment `json:"attachments"`
}

type changeStatusSignatureParams struct {
	Status string `json:"status"`
}

func (gb *GoSignBlock) handleSignature(login string) error {
	updateParams := &signSignatureParams{}

	err := json.Unmarshal(gb.RunContext.UpdateData.Parameters, updateParams)
	if err != nil {
		return errors.New("can't assert provided update data")
	}

	if gb.State.SignatureType == script.SignatureTypeUKEP && gb.State.IsTakenInWork {
		return errors.New("is already taken in work")
	}

	if updateParams.Decision != SignDecisionRejected && !gb.isValidLogin(login) {
		return NewUserIsNotPartOfProcessErr()
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
		if errUpdate := gb.handleDayBeforeSLANotifications(ctx); errUpdate != nil {
			return nil, errUpdate
		}
	case string(entity.TaskUpdateActionSign):
		if errUpdate := gb.handleSignature(data.ByLogin); errUpdate != nil {
			return nil, errUpdate
		}
	case string(entity.TaskUpdateActionSignChangeWorkStatus):
		if errUpdate := gb.handleChangeWorkStatus(ctx, data.ByLogin); errUpdate != nil {
			return nil, errUpdate
		}
	}

	var stateBytes []byte
	stateBytes, err := json.Marshal(gb.State)
	if err != nil {
		return nil, err
	}

	gb.RunContext.VarStore.ReplaceState(gb.Name, stateBytes)

	if _, ok := gb.expectedEvents[eventEnd]; ok {
		event, eventErr := gb.RunContext.MakeNodeEndEvent(ctx, gb.Name, gb.GetTaskHumanStatus(), gb.GetStatus())
		if eventErr != nil {
			return nil, eventErr
		}
		gb.happenedEvents = append(gb.happenedEvents, event)
	}

	return nil, nil
}

//nolint:dupl,gocyclo //its not duplicate
func (gb *GoSignBlock) handleBreachedSLA(ctx c.Context) error {
	if gb.State.CheckSLA == nil || !*gb.State.CheckSLA {
		gb.State.SLAChecked = true
		return nil
	}

	if gb.State.SLAChecked {
		return nil
	}

	if gb.State.AutoReject != nil && *gb.State.AutoReject {
		gb.RunContext.UpdateData.ByLogin = autoSigner
		gb.State.ActualSigner = &gb.RunContext.UpdateData.ByLogin
		if setErr := gb.setSignerDecision(&signSignatureParams{
			Decision: SignDecisionRejected,
			Comment:  AutoActionComment,
		}); setErr != nil {
			return setErr
		}
	}

	gb.State.SLAChecked = true

	if gb.State.SLA != nil {
		emails := make([]string, 0, len(gb.State.Signers))
		logins := getSliceFromMapOfStrings(gb.State.Signers)

		for i := range logins {
			eml, err := gb.RunContext.Services.People.GetUserEmail(ctx, logins[i])
			if err != nil {
				continue
			}
			emails = append(emails, eml)
		}

		err := gb.RunContext.Services.Sender.SendNotification(ctx, emails, nil,
			mail.NewSignSLAExpiredTemplate(
				gb.RunContext.WorkNumber,
				gb.RunContext.WorkTitle,
				gb.RunContext.Services.Sender.SdAddress,
			),
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (gb *GoSignBlock) handleChangeWorkStatus(ctx c.Context, login string) error {
	log := logger.GetLogger(ctx)

	status := &changeStatusSignatureParams{}

	err := json.Unmarshal(gb.RunContext.UpdateData.Parameters, status)
	if err != nil {
		return errors.New("can't assert provided update data")
	}

	if gb.State.WorkerLogin != login {
		return NewUserIsNotPartOfProcessErr()
	}

	if gb.State.IsTakenInWork && gb.State.WorkerLogin == login && status.Status == "start" {
		gb.State.IsTakenInWork = true
		gb.State.WorkerLogin = login
	} else {
		gb.State.IsTakenInWork = false
		gb.State.WorkerLogin = ""

		return nil
	}

	_, err = gb.RunContext.Services.Scheduler.CreateTask(ctx, &scheduler.CreateTask{
		WorkNumber:  gb.RunContext.WorkNumber,
		WorkID:      gb.RunContext.TaskID.String(),
		ActionName:  string(entity.TaskUpdateActionSignChangeWorkStatus),
		WaitSeconds: int(time.Minute * 5),
	})
	if err != nil {
		log.WithError(err).Error("cannot create signChangeWorkStatus timer")
		return err
	}

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

func (gb *GoSignBlock) isValidLogin(login string) bool {
	if gb.State.WorkerLogin != login &&
		(login != ServiceAccount &&
			login != ServiceAccountStage &&
			login != ServiceAccountDev) {
		return false
	}

	return true
}
