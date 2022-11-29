package pipeline

import (
	c "context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
)

//nolint:gocyclo //its ok here
func (gb *GoExecutionBlock) Update(ctx c.Context) (interface{}, error) {
	switch gb.RunContext.UpdateData.Action {
	case string(entity.TaskUpdateActionSLABreach):
		if errUpdate := gb.handleBreachedSLA(ctx); errUpdate != nil {
			return nil, errUpdate
		}
	case string(entity.TaskUpdateActionHalfSLABreach):
		if errUpdate := gb.handleHalfSLABreached(ctx); errUpdate != nil {
			return nil, errUpdate
		}
	case string(entity.TaskUpdateActionExecution):
		if errUpdate := gb.updateDecision(); errUpdate != nil {
			return nil, errUpdate
		}
	case string(entity.TaskUpdateActionChangeExecutor):
		if errUpdate := gb.changeExecutor(ctx); errUpdate != nil {
			return nil, errUpdate
		}
	case string(entity.TaskUpdateActionCancelApp):
		if errUpdate := gb.cancelPipeline(ctx); errUpdate != nil {
			return nil, errUpdate
		}
	case string(entity.TaskUpdateActionRequestExecutionInfo):
		if errUpdate := gb.updateRequestInfo(ctx); errUpdate != nil {
			return nil, errUpdate
		}
	case string(entity.TaskUpdateActionExecutorStartWork):
		if errUpdate := gb.executorStartWork(ctx); errUpdate != nil {
			return nil, errUpdate
		}
	case string(entity.TaskUpdateActionExecutorSendEditApp):
		if errUpdate := gb.toEditApplication(ctx); errUpdate != nil {
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

type ExecutorChangeParams struct {
	NewExecutorLogin string   `json:"new_executor_login"`
	Comment          string   `json:"comment"`
	Attachments      []string `json:"attachments,omitempty"`
}

func (gb *GoExecutionBlock) changeExecutor(ctx c.Context) (err error) {
	if _, isExecutor := gb.State.Executors[gb.RunContext.UpdateData.ByLogin]; !isExecutor {
		return fmt.Errorf("can't change executor, user %s in not executor", gb.RunContext.UpdateData.ByLogin)
	}

	var updateParams ExecutorChangeParams
	if err = json.Unmarshal(gb.RunContext.UpdateData.Parameters, &updateParams); err != nil {
		return errors.New("can't assert provided update data")
	}

	if err = gb.State.SetChangeExecutor(gb.RunContext.UpdateData.ByLogin, &updateParams); err != nil {
		return errors.New("can't assert provided change executor data")
	}

	delete(gb.State.Executors, gb.RunContext.UpdateData.ByLogin)
	oldExecutors := gb.State.Executors

	// add new person to exec anyway
	defer func() {
		oldExecutors[updateParams.NewExecutorLogin] = struct{}{}
		gb.State.Executors = oldExecutors
	}()

	gb.State.Executors = map[string]struct{}{
		updateParams.NewExecutorLogin: {},
	}
	// do notif only for the new person
	if notifErr := gb.handleNotifications(ctx); notifErr != nil {
		return notifErr
	}

	return nil
}

func (a *ExecutionData) SetChangeExecutor(oldLogin string, in *ExecutorChangeParams) error {
	_, ok := a.Executors[oldLogin]
	if !ok {
		return fmt.Errorf("%s not found in executors", oldLogin)
	}

	a.ChangedExecutorsLogs = append(a.ChangedExecutorsLogs, ChangeExecutorLog{
		OldLogin:    oldLogin,
		NewLogin:    in.NewExecutorLogin,
		Comment:     in.Comment,
		Attachments: in.Attachments,
		CreatedAt:   time.Now(),
	})

	return nil
}

type ExecutionUpdateParams struct {
	Decision    ExecutionDecision `json:"decision"`
	Comment     string            `json:"comment"`
	Attachments []string          `json:"attachments"`
}

func (gb *GoExecutionBlock) handleBreachedSLA(ctx c.Context) error {
	if gb.State.SLA > 8 {
		emails := make([]string, 0, len(gb.State.Executors))
		for executor := range gb.State.Executors {
			email, err := gb.RunContext.People.GetUserEmail(ctx, executor)
			if err != nil {
				continue
			}
			emails = append(emails, email)
		}
		if len(emails) == 0 {
			return nil
		}
		err := gb.RunContext.Sender.SendNotification(ctx, emails, nil,
			mail.NewExecutionSLATemplate(gb.RunContext.WorkNumber, gb.RunContext.WorkTitle, gb.RunContext.Sender.SdAddress))
		if err != nil {
			return err
		}
	}
	gb.State.SLAChecked = true
	gb.State.HalfSLAChecked = true
	return nil
}

func (gb *GoExecutionBlock) handleHalfSLABreached(ctx c.Context) error {
	if gb.State.SLA > 8 {
		emails := make([]string, 0, len(gb.State.Executors))
		for executor := range gb.State.Executors {
			email, err := gb.RunContext.People.GetUserEmail(ctx, executor)
			if err != nil {
				continue
			}
			emails = append(emails, email)
		}
		if len(emails) == 0 {
			return nil
		}
		err := gb.RunContext.Sender.SendNotification(ctx, emails, nil,
			mail.NewExecutiontHalfSLATemplate(gb.RunContext.WorkNumber, gb.RunContext.WorkTitle, gb.RunContext.Sender.SdAddress))
		if err != nil {
			return err
		}
	}
	gb.State.HalfSLAChecked = true
	return nil
}

func (gb *GoExecutionBlock) updateDecision() error {
	var updateParams ExecutionUpdateParams

	err := json.Unmarshal(gb.RunContext.UpdateData.Parameters, &updateParams)
	if err != nil {
		return errors.New("can't assert provided update data")
	}

	if errSet := gb.State.SetDecision(gb.RunContext.UpdateData.ByLogin, &updateParams); errSet != nil {
		return errSet
	}

	if gb.State.Decision != nil {
		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputExecutionLogin], &gb.State.ActualExecutor)
		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputDecision], &gb.State.Decision)
		gb.RunContext.VarStore.SetValue(gb.Output[keyOutputComment], &gb.State.DecisionComment)
	}

	return nil
}

func (a *ExecutionData) SetDecision(login string, in *ExecutionUpdateParams) error {
	_, ok := a.Executors[login]
	if !ok {
		return fmt.Errorf("%s not found in executors", login)
	}

	if a.Decision != nil {
		return errors.New("decision already set")
	}

	if in.Decision != ExecutionDecisionExecuted && in.Decision != ExecutionDecisionRejected {
		return fmt.Errorf("unknown decision %s", in.Decision)
	}

	a.Decision = &in.Decision
	a.DecisionComment = &in.Comment
	a.DecisionAttachments = in.Attachments
	a.ActualExecutor = &login

	return nil
}

type RequestInfoUpdateParams struct {
	Comment       string          `json:"comment"`
	ReqType       RequestInfoType `json:"req_type"`
	Attachments   []string        `json:"attachments"`
	ExecutorLogin string          `json:"executor_login"`
}

//nolint:gocyclo //its ok here
func (gb *GoExecutionBlock) updateRequestInfo(ctx c.Context) (err error) {
	var updateParams RequestInfoUpdateParams

	err = json.Unmarshal(gb.RunContext.UpdateData.Parameters, &updateParams)
	if err != nil {
		return errors.New("can't assert provided update requestExecutionInfo data")
	}

	if errSet := gb.State.SetRequestExecutionInfo(gb.RunContext.UpdateData.ByLogin, &updateParams); errSet != nil {
		return errSet
	}

	if updateParams.ReqType == RequestInfoAnswer {
		if _, executorExists := gb.State.Executors[updateParams.ExecutorLogin]; !executorExists {
			return fmt.Errorf("executor: %s is not found in executors", updateParams.ExecutorLogin)
		}
		if len(gb.State.RequestExecutionInfoLogs) > 0 {
			workHours := getWorkWorkHoursBetweenDates(
				gb.State.RequestExecutionInfoLogs[len(gb.State.RequestExecutionInfoLogs)-1].CreatedAt,
				time.Now(),
			)
			gb.State.IncreaseSLA(workHours)
		}
	}

	if updateParams.ReqType == RequestInfoQuestion {
		authorEmail, emailErr := gb.RunContext.People.GetUserEmail(ctx, gb.RunContext.Initiator)
		if emailErr != nil {
			return emailErr
		}

		tpl := mail.NewRequestExecutionInfoTemplate(gb.RunContext.WorkNumber,
			gb.RunContext.WorkTitle, gb.RunContext.Sender.SdAddress)
		err = gb.RunContext.Sender.SendNotification(ctx, []string{authorEmail}, nil, tpl)
		if err != nil {
			return err
		}
	}

	if updateParams.ReqType == RequestInfoAnswer {
		emails := make([]string, 0, len(gb.State.Executors))
		for executor := range gb.State.Executors {
			email, emailErr := gb.RunContext.People.GetUserEmail(ctx, executor)
			if emailErr != nil {
				continue
			}

			emails = append(emails, email)
		}

		tpl := mail.NewAnswerExecutionInfoTemplate(gb.RunContext.WorkNumber,
			gb.RunContext.WorkTitle, gb.RunContext.Sender.SdAddress)
		err = gb.RunContext.Sender.SendNotification(ctx, emails, nil, tpl)
		if err != nil {
			return err
		}
	}

	return err
}

func (a *ExecutionData) SetRequestExecutionInfo(login string, in *RequestInfoUpdateParams) error {
	_, ok := a.Executors[login]
	if !ok && in.ReqType == RequestInfoQuestion {
		return fmt.Errorf("%s not found in executors", login)
	}

	if in.ReqType != RequestInfoAnswer && in.ReqType != RequestInfoQuestion {
		return fmt.Errorf("request info type is not valid")
	}

	a.RequestExecutionInfoLogs = append(a.RequestExecutionInfoLogs, RequestExecutionInfoLog{
		Login:       login,
		Comment:     in.Comment,
		CreatedAt:   time.Now(),
		ReqType:     in.ReqType,
		Attachments: in.Attachments,
	})

	return nil
}

func (gb *GoExecutionBlock) executorStartWork(ctx c.Context) (err error) {
	if _, ok := gb.State.Executors[gb.RunContext.UpdateData.ByLogin]; !ok {
		return fmt.Errorf("login %s is not found in executors", gb.RunContext.UpdateData.ByLogin)
	}
	executorLogins := gb.State.Executors

	gb.State.Executors = map[string]struct{}{
		gb.RunContext.UpdateData.ByLogin: {},
	}

	gb.State.IsTakenInWork = true
	workHours := getWorkWorkHoursBetweenDates(
		gb.RunContext.currBlockStartTime,
		time.Now(),
	)
	gb.State.IncreaseSLA(workHours)

	if err = gb.emailGroupExecutors(ctx, executorLogins); err != nil {
		return nil
	}

	return nil
}

func (gb *GoExecutionBlock) emailGroupExecutors(ctx c.Context, logins map[string]struct{}) (err error) {
	var notificationEmails []string
	for login := range logins {
		if login != gb.RunContext.UpdateData.ByLogin {
			email, emailErr := gb.RunContext.People.GetUserEmail(ctx, login)
			if emailErr != nil {
				return emailErr
			}
			notificationEmails = append(notificationEmails, email)
		}
	}

	descr, err := gb.RunContext.makeNotificationDescription(gb.Name)
	if err != nil {
		return err
	}

	author, err := gb.RunContext.People.GetUser(ctx, gb.RunContext.UpdateData.ByLogin)
	if err != nil {
		return err
	}

	typedAuthor, err := author.ToSSOUserTyped()
	if err != nil {
		return err
	}

	tpl := mail.NewExecutionTakenInWork(&mail.ExecutorNotifTemplate{
		Id:           gb.RunContext.WorkNumber,
		SdUrl:        gb.RunContext.Sender.SdAddress,
		ExecutorName: typedAuthor.GetFullName(),
		Initiator:    gb.RunContext.Initiator,
		Description:  descr,
	})

	if err := gb.RunContext.Sender.SendNotification(ctx, notificationEmails, nil, tpl); err != nil {
		return err
	}

	return nil
}

// nolint:dupl // another action
func (gb *GoExecutionBlock) cancelPipeline(ctx c.Context) error {
	gb.State.IsRevoked = true
	if stopErr := gb.RunContext.Storage.StopTaskBlocks(ctx, gb.RunContext.TaskID); stopErr != nil {
		return stopErr
	}
	if stopErr := gb.RunContext.updateTaskStatus(ctx, db.RunStatusFinished); stopErr != nil {
		return stopErr
	}
	return nil
}

type executorUpdateEditParams struct {
	Comment     string   `json:"comment"`
	Attachments []string `json:"attachments"`
}

//nolint:gocyclo //its ok here
func (gb *GoExecutionBlock) toEditApplication(ctx c.Context) (err error) {
	var updateParams executorUpdateEditParams
	if err = json.Unmarshal(gb.RunContext.UpdateData.Parameters, &updateParams); err != nil {
		return errors.New("can't assert provided update data")
	}

	if err = gb.State.setEditApp(gb.RunContext.UpdateData.ByLogin, updateParams); err != nil {
		return err
	}

	initiatorEmail, emailErr := gb.RunContext.People.GetUserEmail(ctx, gb.RunContext.Initiator)
	if emailErr != nil {
		return emailErr
	}

	tpl := mail.NewAnswerSendToEditTemplate(gb.RunContext.WorkNumber,
		gb.RunContext.WorkTitle, gb.RunContext.Sender.SdAddress)
	err = gb.RunContext.Sender.SendNotification(ctx, []string{initiatorEmail}, nil, tpl)
	if err != nil {
		return err
	}

	return nil
}
