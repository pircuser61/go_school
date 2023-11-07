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

const (
	changeWorkStatusTimeout = 20 * time.Minute
)

type signSignatureParams struct {
	Decision    SignDecision        `json:"decision"`
	Comment     string              `json:"comment,omitempty"`
	Attachments []entity.Attachment `json:"attachments"`
	Username    string              `json:"username"`
}

type changeStatusSignatureParams struct {
	Status string `json:"status"`
}

func (gb *GoSignBlock) handleSignature(ctx c.Context, login string) error {
	log := logger.GetLogger(ctx)

	updateParams := &signSignatureParams{}

	err := json.Unmarshal(gb.RunContext.UpdateData.Parameters, updateParams)
	if err != nil {
		return errors.New("can't assert provided update data")
	}

	if gb.State.SignatureType == script.SignatureTypeUKEP && updateParams.Decision != SignDecisionRejected {
		if !gb.State.IsTakenInWork {
			if updateParams.Username == "" {
				return errors.New("is not taken in work")
			}
			log.Info("setting signature with no 'taken in work'")
		}
		if !gb.isValidLogin(login) {
			return NewUserIsNotPartOfProcessErr()
		}
	} else {
		if !gb.isValidSigner(login) {
			return NewUserIsNotPartOfProcessErr()
		}
	}

	if setErr := gb.setSignerDecision(updateParams); setErr != nil {
		return setErr
	}

	if updateParams.Decision == SignDecisionError {
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
			mail.NewSignErrorTemplate(
				gb.RunContext.WorkNumber,
				gb.RunContext.Services.Sender.SdAddress,
			),
		)
		if err != nil {
			return err
		}
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
		if errUpdate := gb.handleSignature(ctx, data.ByLogin); errUpdate != nil {
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

	status := &changeStatusSignatureParams{Status: "end"}

	if gb.RunContext.UpdateData.Parameters != nil {
		err := json.Unmarshal(gb.RunContext.UpdateData.Parameters, status)
		if err != nil {
			return errors.New("can't assert provided update data")
		}
	}

	if gb.State.IsTakenInWork && !gb.isValidLogin(login) {
		return NewUserIsNotPartOfProcessErr()
	}

	switch {
	case !gb.State.IsTakenInWork && status.Status == "start":
		if !gb.isValidSigner(login) {
			return NewUserIsNotPartOfProcessErr()
		}
		gb.State.IsTakenInWork = true
		gb.State.WorkerLogin = login

	case gb.State.IsTakenInWork && status.Status == "end":
		gb.State.IsTakenInWork = false
		gb.State.WorkerLogin = ""

		// delete those that may exist
		err := gb.RunContext.Services.Scheduler.DeleteTask(ctx,
			&scheduler.DeleteTask{
				WorkID:   gb.RunContext.TaskID.String(),
				StepName: gb.Name,
			})
		if err != nil {
			log.WithError(err).Error("cannot delete signChangeWorkStatus timer")
			return err
		}

		return nil
	default:
		return nil
	}

	_, err := gb.RunContext.Services.Scheduler.CreateTask(ctx, &scheduler.CreateTask{
		WorkNumber:  gb.RunContext.WorkNumber,
		WorkID:      gb.RunContext.TaskID.String(),
		ActionName:  string(entity.TaskUpdateActionSignChangeWorkStatus),
		StepName:    gb.Name,
		WaitSeconds: int(changeWorkStatusTimeout.Seconds()),
	})
	if err != nil {
		log.WithError(err).Error("cannot create signChangeWorkStatus timer")
		return err
	}

	return nil
}

func (gb *GoSignBlock) setSignerDecision(u *signSignatureParams) error {
	login := gb.RunContext.UpdateData.ByLogin
	if u.Username != "" {
		login = u.Username
	}
	if errUpdate := gb.State.SetDecision(login, u); errUpdate != nil {
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

func (gb *GoSignBlock) isValidSigner(login string) bool {
	if _, ok := gb.State.Signers[login]; !ok &&
		(login != ServiceAccount &&
			login != ServiceAccountStage &&
			login != ServiceAccountDev) {
		return false
	}

	return true
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
