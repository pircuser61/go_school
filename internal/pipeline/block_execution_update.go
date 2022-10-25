package pipeline

import (
	c "context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/google/uuid"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

func (gb *GoExecutionBlock) Update(ctx c.Context, data *script.BlockUpdateData) (interface{}, error) {
	if data == nil {
		return nil, errors.New("update data is empty")
	}

	step, err := gb.Pipeline.Storage.GetTaskStepById(ctx, data.Id)
	if err != nil {
		return nil, err
	}

	if step == nil {
		return nil, errors.New("can't get step from database")
	}

	stepData, ok := step.State[gb.Name]
	if !ok {
		return nil, errors.New("can't get step state")
	}

	var state ExecutionData
	if err = json.Unmarshal(stepData, &state); err != nil {
		return nil, errors.Wrap(err, "invalid format of go-execution-block state")
	}

	gb.State = &state

	if data.Action == string(entity.TaskUpdateActionExecution) {
		if errUpdate := gb.updateExecutionDecision(ctx, data, step); errUpdate != nil {
			return nil, errUpdate
		}
	}

	if data.Action == string(entity.TaskUpdateActionChangeExecutor) {
		if errUpdate := gb.changeExecutor(ctx, data, step); errUpdate != nil {
			return nil, errUpdate
		}
	}

	if data.Action == string(entity.TaskUpdateActionRequestExecutionInfo) {
		if errUpdate := gb.updateRequestExecutionInfo(ctx, &updateRequestExecutionInfoDto{
			data,
			step,
		}); errUpdate != nil {
			return nil, errUpdate
		}
	}

	if data.Action == string(entity.TaskUpdateActionExecutorStartWork) {
		if errUpdate := gb.executorStartWork(ctx, &executorsStartWork{
			stepId:     data.Id,
			step:       step,
			byLogin:    data.ByLogin,
			workNumber: data.WorkNumber,
			author:     data.Author,
		}); errUpdate != nil {
			return nil, errUpdate
		}
	}

	return nil, nil
}

type ExecutorChangeParams struct {
	NewExecutorLogin string   `json:"new_executor_login"`
	Comment          string   `json:"comment"`
	Attachments      []string `json:"attachments,omitempty"`
}

func (gb *GoExecutionBlock) changeExecutor(ctx c.Context, data *script.BlockUpdateData, step *entity.Step) (err error) {
	if _, isExecutor := gb.State.Executors[data.ByLogin]; !isExecutor {
		return fmt.Errorf("can't change executor, user %s in not executor", data.ByLogin)
	}

	var updateParams ExecutorChangeParams
	if err = json.Unmarshal(data.Parameters, &updateParams); err != nil {
		return errors.New("can't assert provided update data")
	}

	if err = gb.State.SetChangeExecutor(data.ByLogin, &updateParams); err != nil {
		return errors.New("can't assert provided change executor data")
	}

	delete(gb.State.Executors, data.ByLogin)
	gb.State.Executors[updateParams.NewExecutorLogin] = struct{}{}
	gb.State.LeftToNotify[updateParams.NewExecutorLogin] = struct{}{}

	if step.State[gb.Name], err = json.Marshal(gb.State); err != nil {
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
		Status:      string(StatusRunning),
	})

	return err
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

func (gb *GoExecutionBlock) updateExecutionDecision(ctx c.Context, in *script.BlockUpdateData, step *entity.Step) error {
	var updateParams ExecutionUpdateParams

	err := json.Unmarshal(in.Parameters, &updateParams)
	if err != nil {
		return errors.New("can't assert provided update data")
	}

	if errSet := gb.State.SetDecision(in.ByLogin, &updateParams); errSet != nil {
		return errSet
	}

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
		Status:      step.Status,
	})

	return err
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

type updateRequestExecutionInfoDto struct {
	data *script.BlockUpdateData
	step *entity.Step
}

type RequestInfoUpdateParams struct {
	Comment       string          `json:"comment"`
	ReqType       RequestInfoType `json:"req_type"`
	Attachments   []string        `json:"attachments"`
	ExecutorLogin string          `json:"executor_login"`
}

type executorsStartWork struct {
	stepId     uuid.UUID
	step       *entity.Step
	byLogin    string
	workNumber string
	author     string
}

//nolint:gocyclo //its ok here
func (gb *GoExecutionBlock) updateRequestExecutionInfo(ctx c.Context, dto *updateRequestExecutionInfoDto) (err error) {
	var updateParams RequestInfoUpdateParams

	err = json.Unmarshal(dto.data.Parameters, &updateParams)
	if err != nil {
		return errors.New("can't assert provided update requestExecutionInfo data")
	}

	if errSet := gb.State.SetRequestExecutionInfo(dto.data.ByLogin, &updateParams); errSet != nil {
		return errSet
	}

	status := string(StatusIdle)
	if updateParams.ReqType == RequestInfoAnswer {
		if _, executorExists := gb.State.Executors[updateParams.ExecutorLogin]; !executorExists {
			return fmt.Errorf("executor: %s is not found in executors", updateParams.ExecutorLogin)
		}

		status = string(StatusRunning)
		if len(gb.State.RequestExecutionInfoLogs) > 0 {
			workHours := getWorkWorkHoursBetweenDates(
				gb.State.RequestExecutionInfoLogs[len(gb.State.RequestExecutionInfoLogs)-1].CreatedAt,
				time.Now(),
			)
			gb.State.IncreaseSLA(workHours)
		}
	}

	dto.step.State[gb.Name], err = json.Marshal(gb.State)
	if err != nil {
		return err
	}

	var content []byte
	content, err = json.Marshal(store.NewFromStep(dto.step))
	if err != nil {
		return err
	}

	err = gb.Pipeline.Storage.UpdateStepContext(ctx, &db.UpdateStepRequest{
		Id:          dto.data.Id,
		Content:     content,
		BreakPoints: dto.step.BreakPoints,
		Status:      status,
	})
	if err != nil {
		return err
	}

	if updateParams.ReqType == RequestInfoQuestion {
		authorEmail, emailErr := gb.Pipeline.People.GetUserEmail(ctx, dto.data.Author)
		if emailErr != nil {
			return emailErr
		}

		tpl := mail.NewRequestExecutionInfoTemplate(dto.data.WorkNumber, dto.data.WorkTitle, gb.Pipeline.Sender.SdAddress)
		err = gb.Pipeline.Sender.SendNotification(ctx, []string{authorEmail}, nil, tpl)
		if err != nil {
			return err
		}
	}

	if updateParams.ReqType == RequestInfoAnswer {
		emails := make([]string, 0, len(gb.State.Executors))
		for executor := range gb.State.Executors {
			email, emailErr := gb.Pipeline.People.GetUserEmail(ctx, executor)
			if emailErr != nil {
				continue
			}

			emails = append(emails, email)
		}

		tpl := mail.NewAnswerExecutionInfoTemplate(dto.data.WorkNumber, dto.data.WorkTitle, gb.Pipeline.Sender.SdAddress)
		err = gb.Pipeline.Sender.SendNotification(ctx, emails, nil, tpl)
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

func (gb *GoExecutionBlock) executorStartWork(ctx c.Context, dto *executorsStartWork) (err error) {
	if _, ok := gb.State.Executors[dto.byLogin]; !ok {
		return fmt.Errorf("login %s is not found in executors", dto.byLogin)
	}
	executorLogins := gb.State.Executors

	gb.State.Executors = map[string]struct{}{
		dto.byLogin: {},
	}

	gb.State.IsTakenInWork = true
	workHours := getWorkWorkHoursBetweenDates(
		dto.step.Time,
		time.Now(),
	)
	gb.State.IncreaseSLA(workHours)

	dto.step.State[gb.Name], err = json.Marshal(gb.State)
	if err != nil {
		return err
	}

	var content []byte
	content, err = json.Marshal(store.NewFromStep(dto.step))
	if err != nil {
		return err
	}

	err = gb.Pipeline.Storage.UpdateStepContext(ctx, &db.UpdateStepRequest{
		Id:          dto.stepId,
		Content:     content,
		BreakPoints: dto.step.BreakPoints,
		Status:      string(StatusRunning),
	})
	if err != nil {
		return err
	}

	if err = gb.emailGroupExecutors(ctx, executorLogins, dto); err != nil {
		return nil
	}

	return nil
}

type description struct {
	Value string `json:"description"`
}

func (gb *GoExecutionBlock) emailGroupExecutors(ctx c.Context, logins map[string]struct{}, dto *executorsStartWork) (err error) {
	var notificationEmails []string
	for login := range logins {
		if login != dto.byLogin {
			email, emailErr := gb.Pipeline.People.GetUserEmail(ctx, login)
			if emailErr != nil {
				return emailErr
			}
			notificationEmails = append(notificationEmails, email)
		}
	}

	descr := description{}
	if errUnmarshal := json.Unmarshal(dto.step.State["servicedesk_application_0"], &descr); errUnmarshal != nil {
		return errUnmarshal
	}

	additionalDescriptions, err := gb.Pipeline.Storage.GetAdditionalForms(dto.workNumber, "")
	if err != nil {
		return err
	}
	for _, item := range additionalDescriptions {
		if item == "" {
			continue
		}
		descr.Value = fmt.Sprintf("%s\n\n%s", descr.Value, item)
	}

	author, err := gb.Pipeline.People.GetUser(ctx, dto.byLogin)
	if err != nil {
		return err
	}

	typedAuthor, err := author.ToSSOUserTyped()
	if err != nil {
		return err
	}

	tpl := mail.NewExecutionTakenInWork(&mail.ExecutorNotifTemplate{
		Id:           dto.workNumber,
		SdUrl:        gb.Pipeline.Sender.SdAddress,
		ExecutorName: typedAuthor.GetFullName(),
		Initiator:    dto.author,
		Description:  descr.Value,
	})

	if err := gb.Pipeline.Sender.SendNotification(ctx, notificationEmails, nil, tpl); err != nil {
		return err
	}

	return nil
}
