package api

import (
	"bytes"
	c "context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/google/uuid"

	"golang.org/x/exp/slices"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	ht "gitlab.services.mts.ru/jocasta/pipeliner/internal/human-tasks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/user"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

type eriusTaskResponse struct {
	ID               uuid.UUID              `json:"id"`
	VersionID        uuid.UUID              `json:"version_id"`
	StartedAt        time.Time              `json:"started_at"`
	LastChangedAt    time.Time              `json:"last_changed_at"`
	FinishedAt       *time.Time             `json:"finished_at"`
	Name             string                 `json:"name"`
	Description      string                 `json:"description"`
	Status           string                 `json:"status"`
	HumanStatus      string                 `json:"human_status"`
	Author           string                 `json:"author"`
	IsDebugMode      bool                   `json:"debug"`
	Parameters       map[string]interface{} `json:"parameters"`
	Steps            taskSteps              `json:"steps"`
	WorkNumber       string                 `json:"work_number"`
	BlueprintID      string                 `json:"blueprint_id"`
	Rate             *int                   `json:"rate"`
	RateComment      *string                `json:"rate_comment"`
	AvailableActions taskActions            `json:"available_actions"`
}

type step struct {
	Time                      time.Time                  `json:"time"`
	Type                      string                     `json:"type"`
	Name                      string                     `json:"name"`
	IsDelegateOfAnyStepMember bool                       `json:"is_delegate_of_any_step_member"`
	State                     map[string]json.RawMessage `json:"state" swaggertype:"object"`
	Storage                   map[string]interface{}     `json:"storage"`
	Errors                    []string                   `json:"errors"`
	Steps                     []string                   `json:"steps"`
	HasError                  bool                       `json:"has_error"`
	Status                    pipeline.Status            `json:"status"`
}

type action struct {
	Id                 string `json:"id"`
	ButtonType         string `json:"button_type"`
	Title              string `json:"title"`
	CommentEnabled     bool   `json:"comment_enabled"`
	AttachmentsEnabled bool   `json:"attachments_enabled"`
}

type taskActions []action
type taskSteps []step

func (eriusTaskResponse) toResponse(in *entity.EriusTask,
	currentUserDelegateSteps map[string]bool) *eriusTaskResponse {
	steps := make([]step, 0, len(in.Steps))
	actions := make([]action, 0, len(in.Actions))
	for i := range in.Steps {
		actionTime := in.Steps[i].Time

		if in.Steps[i].UpdatedAt != nil {
			actionTime = *in.Steps[i].UpdatedAt
		}

		steps = append(steps, step{
			Time:                      actionTime,
			Type:                      in.Steps[i].Type,
			Name:                      in.Steps[i].Name,
			State:                     in.Steps[i].State,
			Storage:                   in.Steps[i].Storage,
			Errors:                    in.Steps[i].Errors,
			Steps:                     in.Steps[i].Steps,
			HasError:                  in.Steps[i].HasError,
			Status:                    pipeline.Status(in.Steps[i].Status),
			IsDelegateOfAnyStepMember: currentUserDelegateSteps[in.Steps[i].Name],
		})
	}

	for _, a := range in.Actions {
		actions = append(actions, action{
			Id:                 a.Id,
			ButtonType:         a.ButtonType,
			Title:              a.Title,
			CommentEnabled:     a.CommentEnabled,
			AttachmentsEnabled: a.AttachmentsEnabled,
		})
	}

	out := &eriusTaskResponse{
		ID:               in.ID,
		VersionID:        in.VersionID,
		StartedAt:        in.StartedAt,
		LastChangedAt:    in.LastChangedAt,
		FinishedAt:       in.FinishedAt,
		Name:             in.Name,
		Description:      in.Description,
		Status:           in.Status,
		HumanStatus:      in.HumanStatus,
		Author:           in.Author,
		IsDebugMode:      in.IsDebugMode,
		Parameters:       in.Parameters,
		Steps:            steps,
		WorkNumber:       in.WorkNumber,
		BlueprintID:      in.BlueprintID,
		Rate:             in.Rate,
		RateComment:      in.RateComment,
		AvailableActions: actions,
	}

	return out
}

func (ae *APIEnv) GetTaskFormSchema(w http.ResponseWriter, req *http.Request, workNumber, formID string) {
	ctx, s := trace.StartSpan(req.Context(), "get_task_form_schema")
	defer s.End()

	log := logger.GetLogger(ctx)

	id, err := ae.DB.GetTaskFormSchemaID(workNumber, formID)
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}
	if err = sendResponse(w, http.StatusOK, id); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

func (ae *APIEnv) GetTask(w http.ResponseWriter, req *http.Request, workNumber string) {
	ctx, s := trace.StartSpan(req.Context(), "get_task")
	defer s.End()

	log := logger.GetLogger(ctx)

	if workNumber == "" {
		e := UUIDParsingError
		log.Error(e.errorMessage(errors.New("workNumber is empty")))
		_ = e.sendError(w)

		return
	}

	ui, err := user.GetEffectiveUserInfoFromCtx(ctx)
	if err != nil {
		e := NoUserInContextError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}

	delegations, err := ae.HumanTasks.GetDelegationsToLogin(ctx, ui.Username)
	if err != nil {
		e := GetDelegationsError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}

	delegationsByApprovement := delegations.FilterByType("approvement")
	delegationsByExecution := delegations.FilterByType("execution")

	dbTask, err := ae.DB.GetTask(ctx,
		delegationsByApprovement.GetUserInArrayWithDelegators([]string{ui.Username}),
		delegationsByExecution.GetUserInArrayWithDelegators([]string{ui.Username}),
		ui.Username,
		workNumber)
	if err != nil {
		e := GetTaskError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}

	steps, err := ae.DB.GetTaskSteps(ctx, dbTask.ID)
	if err != nil {
		e := GetTaskError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	dbTask.Steps = steps

	currentUserDelegateSteps, tErr := ae.getCurrentUserInDelegatesForSteps(ui.Username, &steps, &delegations)
	if tErr != nil {
		e := GetDelegationsError
		log.Error(e.errorMessage(tErr))
		_ = e.sendError(w)

		return
	}

	resp := &eriusTaskResponse{}
	if err = sendResponse(w, http.StatusOK, resp.toResponse(dbTask, currentUserDelegateSteps)); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

type approverBlock struct {
	Approvers           map[string]struct{}  `json:"approvers"`
	AdditionalApprovers []additionalApprover `json:"additional_approvers"`
}

type executionBlock struct {
	Executors map[string]struct{}
}

type additionalApprover struct {
	ApproverLogin string `json:"approver_login"`
}

func (ae *APIEnv) getCurrentUserInDelegatesForSteps(currentUser string, steps *entity.TaskSteps, delegates *ht.Delegations) (
	res map[string]bool, err error) {
	const (
		ApproverBlockType  = "approver"
		ExecutionBlockType = "execution"
		FormBlockType      = "form"
	)

	res = make(map[string]bool, 0)
	for _, s := range *steps {
		var isDelegateAnyPersonOfStep = false

		if s.State == nil {
			continue
		}

		switch s.Type {
		case ApproverBlockType:
			var approver approverBlock
			unmarshalErr := json.Unmarshal(s.State[s.Name], &approver)
			if unmarshalErr != nil {
				return nil, unmarshalErr
			}

			for member := range approver.Approvers {
				if isDelegate(currentUser, member, delegates) {
					isDelegateAnyPersonOfStep = true
					break
				}
			}

			for _, member := range approver.AdditionalApprovers {
				if isDelegate(currentUser, member.ApproverLogin, delegates) {
					isDelegateAnyPersonOfStep = true
					break
				}
			}
		case ExecutionBlockType, FormBlockType:
			var execution executionBlock
			unmarshalErr := json.Unmarshal(s.State[s.Name], &execution)
			if unmarshalErr != nil {
				return nil, unmarshalErr
			}

			for member := range execution.Executors {
				if isDelegate(currentUser, member, delegates) {
					isDelegateAnyPersonOfStep = true
					break
				}
			}
		}

		res[s.Name] = isDelegateAnyPersonOfStep
	}

	return res, nil
}

func isDelegate(currentUser, login string, delegations *ht.Delegations) bool {
	var delegates = delegations.GetDelegates(login)
	return slices.Contains(delegates, currentUser)
}

//nolint:dupl,gocritic //its not duplicate
func (ae *APIEnv) GetTasks(w http.ResponseWriter, req *http.Request, params GetTasksParams) {
	ctx, s := trace.StartSpan(req.Context(), "get_tasks")
	defer s.End()

	log := logger.GetLogger(ctx)

	filters, err := params.toEntity(req)
	if err != nil {
		e := BadFiltersError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	/*delegations, err := ae.HumanTasks.GetDelegationsToLogin(ctx, filters.CurrentUser)
	if err != nil {
		e := GetDelegationsError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}

	if filters.SelectAs != nil {
		switch *filters.SelectAs {
		case "approver", "finished_approver":
			delegations = delegations.FilterByType("approvement")
		case "executor", "finished_executor":
			delegations = delegations.FilterByType("execution")
		default:
			delegations = delegations[:0]
		}
	} else {
		delegations = delegations[:0]
	}*/

	/*getDelegatesFor := []string{filters.CurrentUser}
	if filters.ProcessingLogins != nil {
		getDelegatesFor = append(getDelegatesFor, *filters.ProcessingLogins...)
	}

	users := delegations.GetUserInArrayWithDelegators(getDelegatesFor)*/

	resp, err := ae.DB.GetTasks(ctx, filters, []string{"rapetrin1"})
	if err != nil {
		e := GetTasksError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if err = sendResponse(w, http.StatusOK, resp); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

func (p *GetTasksParams) toEntity(req *http.Request) (entity.TaskFilter, error) {
	var filters entity.TaskFilter

	ui, err := user.GetEffectiveUserInfoFromCtx(req.Context())
	if err != nil {
		return filters, err
	}

	filters.CurrentUser = ui.Username
	limit, offset := parseLimitOffsetWithDefault(p.Limit, p.Offset)

	filters.GetTaskParams = entity.GetTaskParams{
		Name:               p.Name,
		Created:            p.Created.toEntity(),
		Order:              p.Order,
		Limit:              &limit,
		Offset:             &offset,
		TaskIDs:            p.TaskIDs,
		SelectAs:           p.SelectAs,
		Archived:           p.Archived,
		ForCarousel:        p.ForCarousel,
		Status:             statusToEntity(p.Status),
		Receiver:           p.Receiver,
		HasAttachments:     p.HasAttachments,
		InitiatorLogins:    p.InitiatorLogins,
		ProcessingLogins:   p.ProcessingLogins,
		ProcessingGroupIds: p.ProcessingGroupIds,
	}

	if p.ExecutorTypeAssigned != nil {
		typeAssigned := string(*p.ExecutorTypeAssigned)
		filters.GetTaskParams.ExecutorTypeAssigned = &typeAssigned
	}

	return filters, nil
}

func (cr *Created) toEntity() *entity.TimePeriod {
	var timePeriod *entity.TimePeriod
	if cr != nil {
		timePeriod = &entity.TimePeriod{
			Start: cr.Start,
			End:   cr.End,
		}
	}
	return timePeriod
}

func statusToEntity(status *[]string) *string {
	if status == nil {
		return nil
	}
	for i := range *status {
		(*status)[i] = "'" + (*status)[i] + "'"
	}
	qStatus := strings.Join(*status, ",")
	return &qStatus
}

func (ae *APIEnv) GetTasksCount(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "get_tasks_count")
	defer s.End()

	log := logger.GetLogger(ctx)

	ui, err := user.GetEffectiveUserInfoFromCtx(req.Context())
	if err != nil {
		e := GetUserinfoErr
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}

	delegations, err := ae.HumanTasks.GetDelegationsToLogin(ctx, ui.Username)
	if err != nil {
		e := GetDelegationsError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}

	delegationsByApprovement := delegations.FilterByType("approvement")
	delegationsByExecution := delegations.FilterByType("execution")

	resp, err := ae.DB.GetTasksCount(
		ctx,
		ui.Username,
		delegationsByApprovement.GetUserInArrayWithDelegators([]string{ui.Username}),
		delegationsByExecution.GetUserInArrayWithDelegators([]string{ui.Username}))

	if err != nil {
		e := GetTasksCountError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if err = sendResponse(w, http.StatusOK, resp); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

//nolint:dupl //its not duplicate
func (ae *APIEnv) GetPipelineTasks(w http.ResponseWriter, req *http.Request, pipelineID string) {
	ctx, s := trace.StartSpan(req.Context(), "get_pipeline_tasks")
	defer s.End()

	log := logger.GetLogger(ctx)

	id, err := uuid.Parse(pipelineID)
	if err != nil {
		e := UUIDParsingError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	resp, err := ae.DB.GetPipelineTasks(ctx, id)
	if err != nil {
		e := GetTasksError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if err := sendResponse(w, http.StatusOK, resp); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

//nolint:dupl //its not duplicate
func (ae *APIEnv) GetVersionTasks(w http.ResponseWriter, req *http.Request, versionID string) {
	ctx, s := trace.StartSpan(req.Context(), "get_version_logs")
	defer s.End()

	log := logger.GetLogger(ctx)

	id, err := uuid.Parse(versionID)
	if err != nil {
		e := UUIDParsingError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	resp, err := ae.DB.GetVersionTasks(ctx, id)
	if err != nil {
		e := GetTasksError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if err = sendResponse(w, http.StatusOK, resp); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

func (ae *APIEnv) UpdateTasksByMails(w http.ResponseWriter, req *http.Request) {
	const funcName = "update_tasks_by_mails"
	ctx, s := trace.StartSpan(req.Context(), funcName)
	defer s.End()

	log := logger.GetLogger(ctx)
	log.Info(funcName, ", started")

	emails, err := ae.MailFetcher.FetchEmails(ctx)
	if err != nil {
		e := ParseMailsError
		log.WithField(funcName, "parse parsedEmails failed").Error(err)
		_ = e.sendError(w)
		return
	}

	if emails == nil {
		return
	}

	for i := range emails {
		usr, errGetUser := ae.People.GetUser(ctx, emails[i].Action.Login)
		if errGetUser != nil {
			log.WithField("workNumber", emails[i].Action.WorkNumber).
				WithField("login", emails[i].Action.Login).Error(errGetUser)
			continue
		}

		useInfo, errToUserinfo := usr.ToUserinfo()
		if errToUserinfo != nil {
			log.Error(errToUserinfo)
			continue
		}

		if !strings.EqualFold(useInfo.Email, emails[i].From) && !utils.IsContainsInSlice(emails[i].From, useInfo.ProxyEmails) {
			log.WithField("userEmailByLogin", useInfo.Email).
				WithField("emailFromEmail", emails[i].From).
				WithField("proxyEmails", useInfo.ProxyEmails).
				Error(errors.New("login from email not eq or not in proxyAddresses"))
			continue
		}

		for fileName := range emails[i].Action.Attachments {
			r := bytes.NewReader(emails[i].Action.Attachments[fileName].Raw)
			ext := emails[i].Action.Attachments[fileName].Ext
			id, errSave := ae.Minio.SaveFile(ctx, ext, fileName, r, r.Size())
			if errSave != nil {
				log.WithField("workNumber", emails[i].Action.WorkNumber).
					WithField("fileName", fileName).
					Error(errSave)
				continue
			}

			emails[i].Action.AttachmentsIds = append(emails[i].Action.AttachmentsIds, id)
		}

		jsonBody, errParse := json.Marshal(emails[i].Action)
		if errParse != nil {
			log.WithField("workNumber", emails[i].Action.WorkNumber).Error(errParse)
			continue
		}

		updateData := entity.TaskUpdate{
			Action:     entity.TaskUpdateAction(emails[i].Action.ActionName),
			Parameters: jsonBody,
		}

		errUpdate := ae.updateTaskInternal(ctx, emails[i].Action.WorkNumber, emails[i].Action.Login, &updateData)
		if errUpdate != nil {
			log.WithField("action", *emails[i].Action).
				WithField("workNumber", emails[i].Action.WorkNumber).
				Error(errUpdate)
			continue
		}
	}

	if err = sendResponse(w, http.StatusOK, nil); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

//nolint:gocyclo //its ok here
func (ae *APIEnv) UpdateTask(w http.ResponseWriter, req *http.Request, workNumber string) {
	ctx, s := trace.StartSpan(req.Context(), "update_task")
	defer s.End()

	log := logger.GetLogger(ctx)

	if workNumber == "" {
		e := WorkNumberParsingError
		log.Error(e.errorMessage(errors.New("workNumber is empty")))
		_ = e.sendError(w)

		return
	}

	b, err := io.ReadAll(req.Body)
	defer req.Body.Close()

	if err != nil {
		e := RequestReadError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	var updateData entity.TaskUpdate
	if err = json.Unmarshal(b, &updateData); err != nil {
		e := UpdateTaskParsingError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	ui, err := user.GetUserInfoFromCtx(ctx)
	if err != nil {
		e := NoUserInContextError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}

	err = ae.updateTaskInternal(ctx, workNumber, ui.Username, &updateData)

	if err != nil {
		e := UpdateTaskParsingError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if err = sendResponse(w, http.StatusOK, nil); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

//nolint:gocyclo // ok here
func (ae *APIEnv) updateTaskInternal(ctx c.Context, workNumber, userLogin string, in *entity.TaskUpdate) (err error) {
	ctxLocal, span := trace.StartSpan(ctx, "update_task_internal")
	defer span.End()

	log := logger.GetLogger(ctx).WithField("mainFuncName", "updateTaskInternal")

	delegations, getDelegationsErr := ae.HumanTasks.GetDelegationsToLogin(ctxLocal, userLogin)
	if getDelegationsErr != nil {
		return getDelegationsErr
	}

	if validateErr := in.Validate(); validateErr != nil {
		return validateErr
	}

	blockTypes := getTaskStepNameByAction(in.Action)
	if len(blockTypes) == 0 {
		return errors.New("blockTypes is empty")
	}

	delegationsByApprovement := delegations.FilterByType("approvement")
	delegationsByExecution := delegations.FilterByType("execution")

	dbTask, err := ae.DB.GetTask(ctxLocal,
		delegationsByApprovement.GetUserInArrayWithDelegators([]string{userLogin}),
		delegationsByExecution.GetUserInArrayWithDelegators([]string{userLogin}),
		userLogin,
		workNumber)

	if err != nil {
		e := GetTaskError
		return errors.New(e.errorMessage(nil))
	}

	if !dbTask.IsRun() {
		e := UpdateNotRunningTaskError
		return errors.New(e.errorMessage(nil))
	}

	scenario, err := ae.DB.GetPipelineVersion(ctxLocal, dbTask.VersionID, false)
	if err != nil {
		e := GetVersionError
		return errors.New(e.errorMessage(nil))
	}

	var steps entity.TaskSteps
	for _, blockType := range blockTypes {
		stepsByBlock, stepErr := ae.DB.GetUnfinishedTaskStepsByWorkIdAndStepType(ctxLocal, dbTask.ID, blockType, in.Action)
		if stepErr != nil {
			e := GetTaskError
			return errors.New(e.errorMessage(nil))
		}
		steps = append(steps, stepsByBlock...)
	}

	if len(steps) == 0 {
		e := GetTaskError
		return errors.New(e.errorMessage(nil))
	}

	if in.Action == entity.TaskUpdateActionCancelApp {
		steps = steps[:1]
	}

	couldUpdateOne := false
	spCtx := span.SpanContext()
	for _, item := range steps {
		// nolint:staticcheck // fix later
		routineCtx := c.WithValue(c.Background(), XRequestIDHeader, ctx.Value(XRequestIDHeader))
		routineCtx = logger.WithLogger(routineCtx, log)
		processCtx, fakeSpan := trace.StartSpanWithRemoteParent(routineCtx, "start_task_step_update", spCtx)
		fakeSpan.End()

		txStorage, transactionErr := ae.DB.StartTransaction(processCtx)
		if transactionErr != nil {
			continue
		}

		storage, getErr := txStorage.GetVariableStorageForStep(processCtx, dbTask.ID, item.Name)
		if getErr != nil {
			if txErr := txStorage.RollbackTransaction(processCtx); txErr != nil {
				log.WithField("funcName", "GetVariableStorageForStep").
					WithError(errors.New("couldn't rollback tx")).
					Error(txErr)
			}
			log.WithError(getErr).Error("couldn't get block to update")
			continue
		}
		runCtx := &pipeline.BlockRunContext{
			TaskID:     dbTask.ID,
			WorkNumber: workNumber,
			WorkTitle:  dbTask.Name,
			Initiator:  dbTask.Author,
			Storage:    txStorage,
			VarStore:   storage,

			Sender:        ae.Mail,
			Kafka:         ae.Kafka,
			People:        ae.People,
			ServiceDesc:   ae.ServiceDesc,
			FunctionStore: ae.FunctionStore,
			HumanTasks:    ae.HumanTasks,
			Integrations:  ae.Integrations,
			FaaS:          ae.FaaS,

			UpdateData: &script.BlockUpdateData{
				ByLogin:    userLogin,
				Action:     string(in.Action),
				Parameters: in.Parameters,
			},
			Delegations: delegations,
		}

		blockFunc, ok := scenario.Pipeline.Blocks[item.Name]
		if !ok {
			if txErr := txStorage.RollbackTransaction(processCtx); txErr != nil {
				log.WithField("funcName", "get block by name").
					WithError(errors.New("couldn't rollback tx")).
					Error(txErr)
			}
			log.WithError(errors.New("couldn't get block from pipeline")).
				Error("couldn't get block to update")
			continue
		}

		blockErr := pipeline.ProcessBlock(processCtx, item.Name, &blockFunc, runCtx, true)
		if blockErr != nil {
			if txErr := txStorage.RollbackTransaction(processCtx); txErr != nil {
				log.WithField("funcName", "ProcessBlock").
					WithError(errors.New("couldn't rollback tx")).
					Error(txErr)
			}
			log.WithError(blockErr).Error("couldn't update block")
			continue
		}

		if err = txStorage.CommitTransaction(processCtx); err != nil {
			log.WithError(err).Error("couldn't update block")
			continue
		}

		couldUpdateOne = true
	}

	if !couldUpdateOne {
		e := UpdateBlockError
		return errors.New(e.errorMessage(errors.New("couldn't update work")))
	}

	return
}

//nolint:gocyclo //its ok here
func (ae *APIEnv) RateApplication(w http.ResponseWriter, r *http.Request, workNumber string) {
	ctx, s := trace.StartSpan(r.Context(), "rate_application")
	defer s.End()

	log := logger.GetLogger(ctx)

	b, err := io.ReadAll(r.Body)
	defer r.Body.Close()

	if err != nil {
		e := RequestReadError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	req := &RateApplicationRequest{}
	if err = json.Unmarshal(b, req); err != nil {
		e := UpdateTaskParsingError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	ui, err := user.GetUserInfoFromCtx(ctx)
	if err != nil {
		e := NoUserInContextError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}

	err = ae.DB.UpdateTaskRate(ctx, &db.UpdateTaskRate{
		ByLogin:    ui.Username,
		WorkNumber: workNumber,
		Comment:    req.Comment,
		Rate:       req.Rate,
	})
	if err != nil {
		e := UpdateTaskRateError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if err = sendResponse(w, http.StatusOK, nil); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

func getTaskStepNameByAction(action entity.TaskUpdateAction) []string {
	if action == entity.TaskUpdateActionAdditionalApprovement {
		return []string{pipeline.BlockGoApproverID}
	}

	if action == entity.TaskUpdateActionApprovement {
		return []string{pipeline.BlockGoApproverID}
	}

	if action == entity.TaskUpdateActionApproverSendEditApp {
		return []string{pipeline.BlockGoApproverID}
	}

	if action == entity.TaskUpdateActionRequestApproveInfo {
		return []string{pipeline.BlockGoApproverID}
	}

	if action == entity.TaskUpdateActionExecution {
		return []string{pipeline.BlockGoExecutionID}
	}

	if action == entity.TaskUpdateActionChangeExecutor {
		return []string{pipeline.BlockGoExecutionID}
	}

	if action == entity.TaskUpdateActionRequestExecutionInfo {
		return []string{pipeline.BlockGoExecutionID}
	}

	if action == entity.TaskUpdateActionCancelApp {
		return []string{pipeline.BlockGoApproverID, pipeline.BlockGoExecutionID, pipeline.BlockGoFormID, pipeline.BlockGoWaitForAllInputsTitle}
	}

	if action == entity.TaskUpdateActionExecutorStartWork {
		return []string{pipeline.BlockGoExecutionID}
	}

	if action == entity.TaskUpdateActionRequestFillForm {
		return []string{pipeline.BlockGoFormID}
	}

	if action == entity.TaskUpdateActionExecutorSendEditApp {
		return []string{pipeline.BlockGoExecutionID}
	}

	if action == entity.TaskUpdateActionAddApprovers {
		return []string{pipeline.BlockGoApproverID}
	}

	return []string{}
}

func (ae *APIEnv) GetTaskMeanSolveTime(w http.ResponseWriter, req *http.Request, pipelineId string) {
	ctx, s := trace.StartSpan(req.Context(), "get_task_mean_solve_time")
	defer s.End()

	log := logger.GetLogger(ctx).WithField("pipelineId", pipelineId)

	taskTimeIntervals, intervalsErr := ae.DB.GetMeanTaskSolveTime(ctx, pipelineId)
	if intervalsErr != nil {
		e := GetTaskError
		log.Error(e.errorMessage(intervalsErr))
		_ = e.sendError(w)
		return
	}

	var mean = pipeline.ComputeMeanTaskCompletionTime(taskTimeIntervals)

	if err := sendResponse(w, http.StatusOK, script.TaskSolveTime{MeanWorkHours: mean.MeanWorkHours}); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}
