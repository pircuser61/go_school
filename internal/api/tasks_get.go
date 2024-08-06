package api

import (
	c "context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/pkg/errors"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	e "gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	ht "gitlab.services.mts.ru/jocasta/pipeliner/internal/humantasks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/metrics"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sla"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/stephandlers"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/user"
)

const (
	hiddenUserLogin        = "hidden_user"
	executionBaseGroupName = "группа исполнителей"
	executionNotInGroup    = "Не групповая заявка"
	taskPath               = "/tasks/{workNumber}"
)

type taskResp struct {
	ID                 uuid.UUID              `json:"id"`
	VersionID          uuid.UUID              `json:"version_id"`
	StartedAt          time.Time              `json:"started_at"`
	LastChangedAt      *time.Time             `json:"last_changed_at"`
	FinishedAt         *time.Time             `json:"finished_at"`
	Name               string                 `json:"name"`
	Description        string                 `json:"description"`
	Status             string                 `json:"status"`
	HumanStatus        string                 `json:"human_status"`
	HumanStatusComment string                 `json:"human_status_comment"`
	Author             string                 `json:"author"`
	IsDebugMode        bool                   `json:"debug"`
	Parameters         map[string]interface{} `json:"parameters"`
	Steps              taskSteps              `json:"steps"`
	WorkNumber         string                 `json:"work_number"`
	BlueprintID        string                 `json:"blueprint_id"`
	Rate               *int                   `json:"rate"`
	RateComment        *string                `json:"rate_comment"`
	AvailableActions   taskActions            `json:"available_actions"`
	StatusComment      string                 `json:"status_comment"`
	StatusAuthor       string                 `json:"status_author"`
	ProcessDeadline    time.Time              `json:"process_deadline"`
	NodeGroup          []NodeGroup            `json:"node_group"`
	ApprovalList       map[string]string      `json:"approval_list"`
	IsPaused           bool                   `json:"is_paused"`
	ParentWorkNumber   *string                `json:"parent_work_number"`
	ChildWorkNumbers   []string               `json:"child_work_numbers"`
}

type step struct {
	Time                      time.Time                  `json:"time"`
	UpdateTime                *time.Time                 `json:"update_time"`
	Type                      string                     `json:"type"`
	Name                      string                     `json:"name"`
	IsDelegateOfAnyStepMember bool                       `json:"is_delegate_of_any_step_member"`
	State                     map[string]json.RawMessage `json:"state" swaggertype:"object"`
	Storage                   map[string]interface{}     `json:"storage"`
	Errors                    []string                   `json:"errors"`
	Steps                     []string                   `json:"steps"`
	HasError                  bool                       `json:"has_error"`
	Status                    pipeline.Status            `json:"status"`
	ShortTitle                string                     `json:"short_title"`
}

type action struct {
	ID                 string                 `json:"id"`
	ButtonType         string                 `json:"button_type"`
	NodeType           string                 `json:"node_type"`
	Title              string                 `json:"title"`
	CommentEnabled     bool                   `json:"comment_enabled"`
	AttachmentsEnabled bool                   `json:"attachments_enabled"`
	Params             map[string]interface{} `json:"params,omitempty"`
}

type (
	taskActions []action
	taskSteps   []step
)

type taskToResponseDTO struct {
	task         *e.EriusTask
	usrDegSteps  map[string]bool
	sNames       map[string]string
	dln          time.Time
	approvalList []e.ApprovalListSettings
}

func (taskResp) toResponse(in *taskToResponseDTO) *taskResp {
	steps := make([]step, 0, len(in.task.Steps))
	actions := make([]action, 0, len(in.task.Actions))

	for i := range in.task.Steps {
		steps = append(steps, step{
			Time:                      in.task.Steps[i].Time,
			UpdateTime:                in.task.Steps[i].UpdatedAt,
			Type:                      in.task.Steps[i].Type,
			Name:                      in.task.Steps[i].Name,
			State:                     in.task.Steps[i].State,
			Storage:                   in.task.Steps[i].Storage,
			Errors:                    in.task.Steps[i].Errors,
			Steps:                     in.task.Steps[i].Steps,
			HasError:                  in.task.Steps[i].HasError,
			Status:                    pipeline.Status(in.task.Steps[i].Status),
			IsDelegateOfAnyStepMember: in.usrDegSteps[in.task.Steps[i].Name],
			ShortTitle:                in.sNames[in.task.Steps[i].Name],
		})
	}

	for _, a := range in.task.Actions {
		actions = append(actions, action{
			ID:                 a.ID,
			ButtonType:         a.ButtonType,
			NodeType:           a.NodeType,
			Title:              a.Title,
			CommentEnabled:     a.CommentEnabled,
			AttachmentsEnabled: a.AttachmentsEnabled,
			Params:             a.Params,
		})
	}

	out := &taskResp{
		ID:                 in.task.ID,
		VersionID:          in.task.VersionID,
		StartedAt:          in.task.StartedAt,
		LastChangedAt:      in.task.LastChangedAt,
		FinishedAt:         in.task.FinishedAt,
		Name:               in.task.Name,
		Description:        in.task.Description,
		Status:             in.task.Status,
		HumanStatus:        in.task.HumanStatus,
		HumanStatusComment: in.task.HumanStatusComment,
		Author:             in.task.Author,
		IsDebugMode:        in.task.IsDebugMode,
		Parameters:         in.task.Parameters,
		Steps:              steps,
		WorkNumber:         in.task.WorkNumber,
		BlueprintID:        in.task.BlueprintID,
		Rate:               in.task.Rate,
		RateComment:        in.task.RateComment,
		AvailableActions:   actions,
		StatusComment:      in.task.StatusComment,
		StatusAuthor:       in.task.StatusAuthor,
		ProcessDeadline:    in.dln,
		NodeGroup:          groupsToResponse(in.task.NodeGroup),
		ParentWorkNumber:   in.task.ParentWorkNumber,
		ChildWorkNumbers:   in.task.ChildWorkNumbers,
	}

	approvalList := map[string]string{}
	for i := range in.approvalList {
		approvalList[in.approvalList[i].ID] = in.approvalList[i].Name
	}

	if len(approvalList) > 0 {
		out.ApprovalList = approvalList
	}

	return out
}

func groupsToResponse(groups []*e.NodeGroup) []NodeGroup {
	if groups == nil {
		return nil
	}

	resp := make([]NodeGroup, 0, len(groups))

	for i := range groups {
		insideNodes := groupsToResponse(groups[i].Nodes)

		resp = append(resp, NodeGroup{
			EndNode:   groups[i].EndNode,
			Nodes:     &insideNodes,
			Prev:      &groups[i].Prev,
			StartNode: groups[i].StartNode,
		})
	}

	return resp
}

func (ae *Env) GetTaskFormSchema(w http.ResponseWriter, req *http.Request, workNumber, formID string) {
	ctx, s := trace.StartSpan(req.Context(), "get_task_form_schema")
	defer s.End()

	log := script.SetMainFuncLog(ctx,
		"GetTaskFormSchema",
		script.MethodGet,
		script.HTTP,
		s.SpanContext().TraceID.String(),
		"v1").WithField(script.WorkNumber, workNumber)
	errorHandler := newHTTPErrorHandler(log, w)

	id, err := ae.DB.GetTaskFormSchemaID(workNumber, formID)
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}

	if err = sendResponse(w, http.StatusOK, id); err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

//nolint:gocyclo,gocognit //its ok here
func (ae *Env) GetTask(w http.ResponseWriter, req *http.Request, workNumber string) {
	start := time.Now()
	ctx, s := trace.StartSpan(req.Context(), "get_task")

	requestInfo := metrics.NewGetRequestInfo(taskPath)

	defer func() {
		s.End()

		requestInfo.Duration = time.Since(start)

		ae.Metrics.RequestsIncrease(requestInfo)
	}()

	log := script.SetMainFuncLog(ctx,
		"GetTask",
		script.MethodGet,
		script.HTTP,
		s.SpanContext().TraceID.String(),
		"v1").WithField(script.WorkNumber, workNumber)
	errorHandler := newHTTPErrorHandler(log, w)
	errorHandler.setMetricsRequestInfo(requestInfo)

	if workNumber == "" {
		errorHandler.handleError(UUIDParsingError, errors.New("workNumber is empty"))

		return
	}

	requestInfo.WorkNumber = workNumber

	ui, err := user.GetEffectiveUserInfoFromCtx(ctx)
	if err != nil {
		errorHandler.handleError(NoUserInContextError, err)

		return
	}

	delegations, err := ae.HumanTasks.GetDelegationsToLogin(ctx, ui.Username)
	if err != nil {
		errorHandler.handleError(GetDelegationsError, err)

		return
	}

	dApprove := delegations.FilterByType("approvement")
	dExecution := delegations.FilterByType("execution")

	dApprovers := dApprove.GetUserInArrayWithDelegators([]string{ui.Username})
	dExecutors := dExecution.GetUserInArrayWithDelegators([]string{ui.Username})

	dbTask, err := ae.DB.GetTask(ctx, dApprovers, dExecutors, ui.Username, workNumber)
	if err != nil {
		if errors.Is(err, e.ErrNoRecords) {
			errorHandler.handleError(TaskNotFoundError, err)
			requestInfo.Status = TaskNotFoundError.Status()

			return
		}

		errorHandler.handleError(GetTaskError, err)

		return
	}

	var version EriusScenario

	err = json.Unmarshal([]byte(dbTask.VersionContent), &version)
	if err != nil {
		errorHandler.handleError(PipelineParseError, err)

		return
	}

	if len(dbTask.NodeGroup) == 0 {
		err = ae.handleZeroTaskNodeGroup(ctx, dbTask)
		if err != nil {
			errorHandler.handleError(UnknownError, err)

			return
		}
	}

	steps, err := ae.DB.GetTaskSteps(ctx, dbTask.ID)
	if err != nil {
		errorHandler.handleError(GetTaskError, err)

		return
	}

	shortNames := shortNameMap(version.Pipeline.Blocks.AdditionalProperties)

	dbTask.Steps = steps
	isInitiator := ui.Username == dbTask.Author

	accessibleForms, err := ae.getAccessibleForms(ui.Username, &steps, &delegations)
	if err != nil {
		errorHandler.handleError(GetDelegationsError, err)

		return
	}

	if !isInitiator {
		ae.removeFormsFromUser(dbTask, accessibleForms)
	}

	if isInitiator {
		ae.removeHiddenFormsFromInitiator(dbTask, accessibleForms)
	}

	currentUserDelegateSteps, tErr := ae.getCurrentUserInDelegatesForSteps(ui.Username, &steps, &delegations)
	if tErr != nil {
		errorHandler.handleError(GetDelegationsError, tErr)

		return
	}

	scenario, getVersionErr := ae.DB.GetVersionByWorkNumber(ctx, dbTask.WorkNumber)
	if getVersionErr != nil {
		errorHandler.handleError(UnknownError, getVersionErr)

		return
	}

	requestInfo.PipelineID = scenario.PipelineID.String()
	requestInfo.VersionID = scenario.VersionID.String()

	if !isInitiator {
		hideErr := ae.hideExecutors(ctx, dbTask, ui.Username, currentUserDelegateSteps)
		if hideErr != nil {
			errorHandler.handleError(UnknownError, hideErr)

			return
		}
	}

	versionSettings, errSLA := ae.DB.GetSLAVersionSettings(ctx, dbTask.VersionID.String())
	if errSLA != nil {
		errorHandler.handleError(GetProcessSLASettingsError, errSLA)

		return
	}

	slaInfoDTO := sla.InfoDTO{
		TaskCompletionIntervals: []e.TaskCompletionInterval{
			{
				StartedAt:  dbTask.StartedAt,
				FinishedAt: dbTask.StartedAt.Add(time.Hour * 24 * 100),
			},
		},
		WorkType: sla.WorkHourType(versionSettings.WorkType),
	}

	slaInfoPtr, getSLAInfoErr := ae.SLAService.GetSLAInfoPtr(ctx, slaInfoDTO)
	if getSLAInfoErr != nil {
		errorHandler.handleError(UnknownError, getSLAInfoErr)

		return
	}

	deadline := ae.SLAService.ComputeMaxDate(dbTask.StartedAt, float32(versionSettings.SLA), slaInfoPtr)

	approvalLists, err := ae.DB.GetApprovalListsSettings(ctx, dbTask.VersionID.String())
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}

	rel, err := ae.DB.GetTaskRelations(ctx, dbTask.WorkNumber)
	if err != nil {
		return
	}

	if rel != nil && rel.WorkNumber != "" {
		dbTask.ParentWorkNumber = rel.ParentWorkNumber
		dbTask.ChildWorkNumbers = rel.ChildWorkNumbers
	}

	resp := &taskResp{}

	toResponse := &taskToResponseDTO{
		task:         dbTask,
		usrDegSteps:  currentUserDelegateSteps,
		sNames:       shortNames,
		dln:          deadline,
		approvalList: approvalLists,
	}

	err = sendResponse(w, http.StatusOK, resp.toResponse(toResponse))
	if err != nil {
		errorHandler.handleError(UnknownError, err)
	}
}

func shortNameMap(additionalProperties map[string]EriusFunc) map[string]string {
	shortNames := make(map[string]string, len(additionalProperties))

	for key, val := range additionalProperties {
		if val.ShortTitle != nil {
			shortNames[key] = *val.ShortTitle
		} else {
			shortNames[key] = ""
		}
	}

	return shortNames
}

func (ae *Env) handleZeroTaskNodeGroup(ctx c.Context, dbTask *e.EriusTask) error {
	scenario, getVersionErr := ae.DB.GetVersionByWorkNumber(ctx, dbTask.WorkNumber)
	if getVersionErr != nil {
		return getVersionErr
	}

	groups, groupsErr := scenario.Pipeline.Blocks.GetGroups()
	if groupsErr != nil {
		return groupsErr
	}

	updateGroupsErr := ae.DB.UpdateGroupsForEmptyVersions(ctx, scenario.VersionID.String(), groups)
	if updateGroupsErr != nil {
		return updateGroupsErr
	}

	dbTask.NodeGroup = groups

	return nil
}

const (
	approverBlockType  = "approver"
	executionBlockType = "execution"
	formBlockType      = "form"
	signBlockType      = "sign"
)

func (ae *Env) getAccessibleForms(login string, steps *e.TaskSteps, ds *ht.Delegations) (map[string]struct{}, error) {
	accessibleForms := make(map[string]struct{}, 0)

	if ds == nil {
		ds = &ht.Delegations{}
	}

	stepHandler := stephandlers.NewMultipleTypesStepHandler()

	stepHandler.RegisterStepTypeHandler(
		approverBlockType,
		stephandlers.NewAccessibleFormsApproverBlockStepHandler(login, accessibleForms, *ds),
	)
	stepHandler.RegisterStepTypeHandler(
		formBlockType,
		stephandlers.NewAccessibleFormsFormBlockStepHandler(login, accessibleForms),
	)
	stepHandler.RegisterStepTypeHandler(
		executionBlockType,
		stephandlers.NewAccessibleFormsExecutionBlockStepHandler(login, accessibleForms, *ds),
	)
	stepHandler.RegisterStepTypeHandler(
		signBlockType,
		stephandlers.NewAccessibleFormsSignBlockStepHandler(login, accessibleForms),
	)

	err := stepHandler.HandleSteps(*steps)
	if err != nil {
		return nil, err
	}

	return accessibleForms, nil
}

func (ae *Env) getCurrentUserInDelegatesForSteps(
	currentUser string,
	steps *e.TaskSteps,
	delegates *ht.Delegations,
) (userInDelegates map[string]bool, err error) {
	userInDelegates = make(map[string]bool, 0)

	stepHandler := stephandlers.NewMultipleTypesStepHandler()

	stepHandler.RegisterStepTypeHandler(
		approverBlockType,
		stephandlers.NewUserInDelegatesApproverBlockTypeStepHandler(currentUser, *delegates, userInDelegates),
	)

	executionFormBlockTypeStepHandler := stephandlers.NewUserInDelegatesExecutionFromBlockTypesStepHandler(
		currentUser,
		*delegates,
		userInDelegates,
	)

	stepHandler.RegisterStepTypeHandler(executionBlockType, executionFormBlockTypeStepHandler)
	stepHandler.RegisterStepTypeHandler(formBlockType, executionFormBlockTypeStepHandler)

	err = stepHandler.HandleSteps(*steps)
	if err != nil {
		return nil, err
	}

	return userInDelegates, nil
}

const (
	getTasksPath      = "/tasks"
	getTasksUsersPath = "/tasks/users"
)

//nolint:dupl,gocritic,gocognit //its not duplicate // params без поинтера нужен для интерфейса
func (ae *Env) GetTasks(w http.ResponseWriter, req *http.Request, params GetTasksParams) {
	start := time.Now()
	ctx, s := trace.StartSpan(req.Context(), "get_tasks")

	requestInfo := metrics.NewGetRequestInfo(getTasksPath)

	defer func() {
		s.End()

		requestInfo.Duration = time.Since(start)

		ae.Metrics.RequestsIncrease(requestInfo)
	}()

	log := script.SetMainFuncLog(ctx,
		"GetTasks",
		script.MethodGet,
		script.HTTP,
		s.SpanContext().TraceID.String(),
		"v1")
	errorHandler := newHTTPErrorHandler(log, w)
	errorHandler.setMetricsRequestInfo(requestInfo)

	filters, err := params.toEntity(req)
	if err != nil {
		errorHandler.handleError(BadFiltersError, err)

		return
	}

	// we need it in order to fix freezing query ahead
	if filters.Order == nil {
		ascOrder := db.AscOrder
		filters.Order = &ascOrder
	}

	if *filters.Order != db.SkipOrderKey {
		if filters.OrderBy == nil {
			orderByStartedAt := fmt.Sprintf("started_at:%s", *filters.Order)
			filters.OrderBy = &[]string{orderByStartedAt, "id"}
		} else {
			*filters.OrderBy = append(*filters.OrderBy, "id")
		}
	}

	delegations, err := ae.HumanTasks.GetDelegationsToLogin(ctx, filters.CurrentUser)
	if err != nil {
		errorHandler.handleError(GetDelegationsError, err)

		return
	}

	if filters.SelectAs != nil {
		switch *filters.SelectAs {
		case e.SelectAsValApprover:
			delegations = delegations.FilterByType("approvement")
		case e.SelectAsValExecutor, e.SelectAsValQueueExecutor, e.SelectAsValInWorkExecutor:
			delegations = delegations.FilterByType("execution")
		default:
			delegations = delegations[:0]
		}
	} else {
		delegations = delegations[:0]
	}

	users := delegations.GetUserInArrayWithDelegators([]string{filters.CurrentUser})

	resp, err := ae.DB.GetTasks(ctx, filters, users)
	if err != nil {
		errorHandler.handleError(GetTasksError, err)

		return
	}

	for i := range resp.Tasks {
		approvalLists, errGetSettings := ae.DB.GetApprovalListsSettings(ctx, resp.Tasks[i].VersionID.String())
		if errGetSettings != nil {
			errorHandler.handleError(UnknownError, err)

			return
		}

		mapApprovalLists := map[string]string{}
		for j := range approvalLists {
			mapApprovalLists[approvalLists[j].ID] = approvalLists[j].Name
		}

		if len(mapApprovalLists) > 0 {
			resp.Tasks[i].ApprovalList = mapApprovalLists
		}

		rel, taskErr := ae.DB.GetTaskRelations(ctx, resp.Tasks[i].WorkNumber)
		if taskErr != nil {
			errorHandler.handleError(UnknownError, taskErr)

			return
		}

		if rel != nil && rel.WorkNumber != "" {
			resp.Tasks[i].ParentWorkNumber = rel.ParentWorkNumber
			resp.Tasks[i].ChildWorkNumbers = rel.ChildWorkNumbers
		}

		if resp.Tasks[i].CurrentExecutor.ExecutionGroupName == "" {
			if resp.Tasks[i].CurrentExecutor.ExecutionGroupID == "" {
				resp.Tasks[i].CurrentExecutor.ExecutionGroupName = executionNotInGroup

				continue
			}

			resp.Tasks[i].CurrentExecutor.ExecutionGroupName = executionBaseGroupName
		}
	}

	if err = sendResponse(w, http.StatusOK, resp); err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

//nolint:dupl,gocritic //its not duplicate // params без поинтера нужен для интерфейса
func (ae *Env) GetTasksSchemas(w http.ResponseWriter, req *http.Request, params GetTasksSchemasParams) {
	start := time.Now()
	ctx, s := trace.StartSpan(req.Context(), "get_tasks")

	requestInfo := metrics.NewGetRequestInfo(getTasksPath)

	defer func() {
		s.End()

		requestInfo.Duration = time.Since(start)

		ae.Metrics.RequestsIncrease(requestInfo)
	}()

	log := script.SetMainFuncLog(ctx,
		"GetTasksSchemas",
		script.MethodGet,
		script.HTTP,
		s.SpanContext().TraceID.String(),
		"v1")
	errorHandler := newHTTPErrorHandler(log, w)
	errorHandler.setMetricsRequestInfo(requestInfo)

	newParams, err := convertParamsTaskToSchema(params)
	if err != nil {
		errorHandler.handleError(BadFiltersError, err)

		return
	}

	filters, err := newParams.toEntity(req)
	if err != nil {
		errorHandler.handleError(BadFiltersError, err)

		return
	}

	delegations, err := ae.HumanTasks.GetDelegationsToLogin(ctx, filters.CurrentUser)
	if err != nil {
		errorHandler.handleError(GetDelegationsError, err)

		return
	}

	if filters.SelectAs != nil {
		switch *filters.SelectAs {
		case e.SelectAsValApprover:
			delegations = delegations.FilterByType("approvement")
		case e.SelectAsValExecutor, e.SelectAsValQueueExecutor, e.SelectAsValInWorkExecutor:
			delegations = delegations.FilterByType("execution")
		default:
			delegations = delegations[:0]
		}
	} else {
		delegations = delegations[:0]
	}

	users := delegations.GetUserInArrayWithDelegators([]string{filters.CurrentUser})

	resp, err := ae.DB.GetTasksSchemas(ctx, filters, users)
	if err != nil {
		errorHandler.handleError(GetTasksError, err)

		return
	}

	if err = sendResponse(w, http.StatusOK, resp); err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

//nolint:dupl,gocritic //its not duplicate // params без поинтера нужен для интерфейса
func (ae *Env) GetTasksUsers(w http.ResponseWriter, req *http.Request, params GetTasksUsersParams) {
	start := time.Now()
	ctx, s := trace.StartSpan(req.Context(), "get_tasks_users")

	requestInfo := metrics.NewGetRequestInfo(getTasksUsersPath)

	defer func() {
		s.End()

		requestInfo.Duration = time.Since(start)

		ae.Metrics.RequestsIncrease(requestInfo)
	}()

	log := script.SetMainFuncLog(ctx,
		"GetTasksUsers",
		script.MethodGet,
		script.HTTP,
		s.SpanContext().TraceID.String(),
		"v1")
	errorHandler := newHTTPErrorHandler(log, w)
	errorHandler.setMetricsRequestInfo(requestInfo)

	newParams, err := convertTaskUserParams(params)
	if err != nil {
		errorHandler.handleError(BadFiltersError, err)

		return
	}

	filters, err := newParams.toEntity(req)
	if err != nil {
		errorHandler.handleError(BadFiltersError, err)

		return
	}

	delegations, err := ae.HumanTasks.GetDelegationsToLogin(ctx, filters.CurrentUser)
	if err != nil {
		errorHandler.handleError(GetDelegationsError, err)

		return
	}

	if filters.SelectAs != nil {
		switch *filters.SelectAs {
		case e.SelectAsValApprover:
			delegations = delegations.FilterByType("approvement")
		case e.SelectAsValExecutor, e.SelectAsValQueueExecutor, e.SelectAsValInWorkExecutor:
			delegations = delegations.FilterByType("execution")
		default:
			delegations = delegations[:0]
		}
	} else {
		delegations = delegations[:0]
	}

	users := delegations.GetUserInArrayWithDelegators([]string{filters.CurrentUser})

	dbResp, err := ae.DB.GetTasksUsers(ctx, filters, users)
	if err != nil {
		errorHandler.handleError(GetTasksError, err)

		return
	}

	groups := &UniquePersons_Groups{dbResp.Groups}

	resp := UniquePersons{Groups: groups, Logins: &dbResp.Logins}

	respUsers := make([]UniqueUser, 0)

	for i := range dbResp.Logins {
		ssoUser, errSso := ae.People.GetUser(ctx, dbResp.Logins[i], false)
		if errSso != nil {
			errorHandler.handleError(GetUserinfoErr, errSso)

			return
		}

		person, errConv := ssoUser.ToPerson()
		if errConv != nil {
			errorHandler.handleError(GetUserinfoErr, errConv)

			return
		}

		if ssoUser != nil {
			respUsers = append(respUsers, UniqueUser{
				FullName: person.Fullname,
				TabNum:   person.Tabnum,
				Username: person.Username,
			})
		}
	}

	resp.Users = &respUsers

	if filters.InitiatorReq != nil && *filters.InitiatorReq {
		resp.InitLogins = &dbResp.InitLogins
	}

	if err = sendResponse(w, http.StatusOK, resp); err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

//nolint:dupl //Нужно для /tasks
func (p *GetTasksParams) toEntity(req *http.Request) (e.TaskFilter, error) {
	var filters e.TaskFilter

	var typeAssigned *string

	if p.ExecutorTypeAssigned != nil {
		at := string(*p.ExecutorTypeAssigned)

		typeAssigned = &at
		if *typeAssigned != e.AssignedToMe && *typeAssigned != e.AssignedByMe {
			return filters, errors.New("invalid value in typeAssigned filter")
		}
	}

	var signatureCarrier *string

	if p.SignatureCarrier != nil {
		at := string(*p.SignatureCarrier)

		signatureCarrier = &at
		if *signatureCarrier != e.SignatureCarrierCloud &&
			*signatureCarrier != e.SignatureCarrierToken &&
			*signatureCarrier != e.SignatureCarrierAll {
			return filters, errors.New("invalid value in SignatureCarrier filter")
		}
	}

	var selectAs string

	if p.SelectAs != nil {
		selectAs = string(*p.SelectAs)

		valid := selectAsValid(selectAs)
		if !valid {
			return filters, errors.New("invalid value in SelectAs filter")
		}
	}

	ui, err := user.GetEffectiveUserInfoFromCtx(req.Context())
	if err != nil {
		return filters, err
	}

	filters.CurrentUser = ui.Username

	limit, offset := parseLimitOffsetWithDefault(p.Limit, p.Offset)
	if limit > 1000 {
		limit = 1000
	}

	filters.GetTaskParams = e.GetTaskParams{
		Name:                 p.Name,
		Created:              p.Created.toEntity(),
		Order:                p.Order,
		OrderBy:              p.OrderBy,
		Expired:              p.Expired,
		Limit:                &limit,
		Fields:               p.Fields,
		Offset:               &offset,
		TaskIDs:              p.TaskIDs,
		SelectAs:             &selectAs,
		Archived:             p.Archived,
		ForCarousel:          p.ForCarousel,
		Status:               statusToEntity(p.Status),
		Receiver:             p.Receiver,
		ProcessDeadline:      p.ProcessDeadline.toEntity(),
		HasAttachments:       p.HasAttachments,
		Initiator:            p.Initiator,
		InitiatorLogins:      p.InitiatorLogins,
		InitiatorReq:         p.InitiatorReq,
		ProcessingLogins:     p.ProcessingLogins,
		ProcessingGroupIds:   p.ProcessingGroupIds,
		ExecutorLogins:       p.ExecutorLogins,
		ExecutorGroupIds:     p.ExecutorGroupIds,
		ExecutorTypeAssigned: typeAssigned,
		SignatureCarrier:     signatureCarrier,
	}

	return filters, nil
}

func selectAsValid(selectAs string) bool {
	switch selectAs {
	case e.SelectAsValApprover,
		e.SelectAsValFinishedApprover,
		e.SelectAsValExecutor,
		e.SelectAsValFinishedExecutor,
		e.SelectAsValFormExecutor,
		e.SelectAsValFinishedFormExecutor,
		e.SelectAsValSignerPhys,
		e.SelectAsValFinishedSignerPhys,
		e.SelectAsValSignerJur,
		e.SelectAsValFinishedSignerJur,
		e.SelectAsValInitiators,
		e.SelectAsValGroupExecutor,
		e.SelectAsValFinishedGroupExecutor,
		e.SelectAsValQueueExecutor,
		e.SelectAsValInWorkExecutor,
		e.SelectAsValFinishedExecutorV2:
		return true
	}

	return false
}

func (cr *TimePeriod) toEntity() *e.TimePeriod {
	var timePeriod *e.TimePeriod

	if cr != nil {
		timePeriod = &e.TimePeriod{
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

func (ae *Env) GetTasksCount(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "get_tasks_count")
	defer s.End()

	log := script.SetMainFuncLog(ctx,
		"GetTasksCount",
		script.MethodGet,
		script.HTTP,
		s.SpanContext().TraceID.String(),
		"v1")
	errorHandler := newHTTPErrorHandler(log, w)

	ui, err := user.GetEffectiveUserInfoFromCtx(req.Context())
	if err != nil {
		errorHandler.handleError(GetUserinfoErr, err)

		return
	}

	delegations, err := ae.HumanTasks.GetDelegationsToLogin(ctx, ui.Username)
	if err != nil {
		errorHandler.handleError(GetDelegationsError, err)

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
		errorHandler.handleError(GetTasksCountError, err)

		return
	}

	err = sendResponse(w, http.StatusOK, resp)
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

// nolint:gocyclo //number of actions to reduce the limit
func getTaskStepNameByAction(action e.TaskUpdateAction) []string {
	if action == e.TaskUpdateActionAdditionalApprovement {
		return []string{pipeline.BlockGoApproverID, pipeline.BlockGoSignID}
	}

	if action == e.TaskUpdateActionApprovement {
		return []string{pipeline.BlockGoApproverID}
	}

	if action == e.TaskUpdateActionApproverSendEditApp {
		return []string{pipeline.BlockGoApproverID}
	}

	if action == e.TaskUpdateActionRequestApproveInfo {
		return []string{pipeline.BlockGoApproverID}
	}

	if action == e.TaskUpdateActionReplyApproverInfo {
		return []string{pipeline.BlockGoApproverID}
	}

	if action == e.TaskUpdateActionExecution {
		return []string{pipeline.BlockGoExecutionID}
	}

	if action == e.TaskUpdateActionChangeExecutor {
		return []string{pipeline.BlockGoExecutionID}
	}

	if action == e.TaskUpdateActionRequestExecutionInfo {
		return []string{pipeline.BlockGoExecutionID}
	}

	if action == e.TaskUpdateActionReplyExecutionInfo {
		return []string{pipeline.BlockGoExecutionID}
	}

	if action == e.TaskUpdateActionExecutorStartWork {
		return []string{pipeline.BlockGoExecutionID}
	}

	if action == e.TaskUpdateActionBackToGroup {
		return []string{pipeline.BlockGoExecutionID}
	}

	if action == e.TaskUpdateActionRequestFillForm {
		return []string{pipeline.BlockGoFormID}
	}

	if action == e.TaskUpdateActionExecutorSendEditApp {
		return []string{pipeline.BlockGoExecutionID}
	}

	if action == e.TaskUpdateActionAddApprovers {
		return []string{pipeline.BlockGoApproverID, pipeline.BlockGoSignID}
	}

	if action == e.TaskUpdateActionFormExecutorStartWork {
		return []string{pipeline.BlockGoFormID}
	}

	if action == e.TaskUpdateActionSign {
		return []string{pipeline.BlockGoSignID}
	}

	if action == e.TaskUpdateActionSignChangeWorkStatus {
		return []string{pipeline.BlockGoSignID}
	}

	if action == e.TaskUpdateActionFinishTimer {
		return []string{pipeline.BlockTimerID}
	}

	if action == e.TaskUpdateActionFuncSLAExpired {
		return []string{pipeline.BlockExecutableFunctionID}
	}

	if action == e.TaskUpdateActionRetry {
		return []string{pipeline.BlockExecutableFunctionID}
	}

	return []string{}
}

func (ae *Env) GetTaskMeanSolveTime(w http.ResponseWriter, req *http.Request, pipelineID string) {
	ctx, s := trace.StartSpan(req.Context(), "get_task_mean_solve_time")
	defer s.End()

	log := script.SetMainFuncLog(ctx,
		"GetTaskMeanSolveTime",
		script.MethodGet,
		script.HTTP,
		s.SpanContext().TraceID.String(),
		"v1").WithField("pipelineID", pipelineID)
	errorHandler := newHTTPErrorHandler(log, w)

	taskTimeIntervals, intervalsErr := ae.DB.GetMeanTaskSolveTime(ctx, pipelineID) // it returns ordered by created_at
	if intervalsErr != nil {
		errorHandler.handleError(GetTaskError, intervalsErr)

		return
	}

	if len(taskTimeIntervals) == 0 {
		err := sendResponse(w, http.StatusOK, script.TaskSolveTime{MeanWorkHours: 0})
		if err != nil {
			errorHandler.handleError(UnknownError, err)
		}

		return
	}

	calendarDays, err := ae.HrGate.GetDefaultCalendarDaysForGivenTimeIntervals(ctx, taskTimeIntervals)
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}

	mean := ae.SLAService.ComputeMeanTaskCompletionTime(taskTimeIntervals, *calendarDays)

	err = sendResponse(w, http.StatusOK, script.TaskSolveTime{MeanWorkHours: mean.MeanWorkHours})
	if err != nil {
		errorHandler.handleError(UnknownError, err)
	}
}

func (ae *Env) removeHiddenFormsFromInitiator(dbTask *e.EriusTask, accessibleForms map[string]struct{}) {
	res := make([]*e.Step, 0, len(dbTask.Steps))

	for _, st := range dbTask.Steps {
		if st.Type != formBlockType {
			res = append(res, st)

			continue
		}

		if _, ok := accessibleForms[st.Name]; ok {
			res = append(res, st)

			continue
		}

		var formBlock pipeline.FormData

		err := json.Unmarshal(st.State[st.Name], &formBlock)
		if err != nil {
			ae.Log.WithField("func", "removeHiddenFormsForInitiator").Warning(errors.Wrap(err, st.Name))

			continue
		}

		_, isInitiatorExecutor := formBlock.Executors[dbTask.Author]
		if !formBlock.HideFormFromInitiator || isInitiatorExecutor {
			res = append(res, st)
		}
	}

	dbTask.Steps = res
}

func (ae *Env) removeFormsFromUser(dbTask *e.EriusTask, accessibleForms map[string]struct{}) {
	actualSteps := make([]*e.Step, 0, len(dbTask.Steps))

	for _, st := range dbTask.Steps {
		if _, ok := accessibleForms[st.Name]; !ok && st.Type == formBlockType {
			continue
		}

		newSteps := make([]string, 0, len(st.Steps))

		for _, s := range st.Steps {
			if _, ok := accessibleForms[s]; !ok && strings.HasPrefix(s, formBlockType) {
				continue
			}

			newSteps = append(newSteps, s)
		}

		st.Steps = newSteps

		newStates := make(map[string]json.RawMessage, len(st.State))

		for k, v := range st.State {
			if _, ok := accessibleForms[k]; !ok && strings.HasPrefix(k, formBlockType) {
				continue
			}

			newStates[k] = v
		}

		for k := range st.Storage {
			key := strings.Split(k, ".")

			if _, ok := accessibleForms[key[0]]; !ok && strings.HasPrefix(k, formBlockType) {
				delete(st.Storage, k)
			}
		}

		st.State = newStates
		actualSteps = append(actualSteps, st)
	}

	dbTask.Steps = actualSteps
}

func (ae *Env) hideExecutors(ctx c.Context, dbTask *e.EriusTask, login string, stepDelegates map[string]bool) error {
	dbMembers, membErr := ae.DB.GetTaskMembers(ctx, dbTask.WorkNumber, false)
	if membErr != nil {
		return membErr
	}

	members := make([]string, 0)

	for i := range dbMembers {
		members = append(members, dbMembers[i].Login)
	}

	stepHandler := stephandlers.NewMultipleTypesStepHandler()

	stepHandler.RegisterStepTypeHandler(
		pipeline.BlockGoFormID,
		stephandlers.NewHideExecutorsFormBlockStepHandler(stepDelegates, members, login),
	)

	stepHandler.RegisterStepTypeHandler(
		pipeline.BlockGoExecutionID,
		stephandlers.NewHideExecutorsExecutionBlockStepHandler(stepDelegates, members, login),
	)

	err := stepHandler.HandleSteps(dbTask.Steps)
	if err != nil {
		return err
	}

	return nil
}

func (ae *Env) GetTaskForUpdate(ctx c.Context, workNumber string) (task *e.EriusTask, err error) {
	dbTask, taskErr := ae.DB.GetTask(
		ctx,
		[]string{""},
		[]string{""},
		"",
		workNumber,
	)
	if taskErr != nil {
		return nil, taskErr
	}

	workID, idErr := ae.DB.GetWorkIDByWorkNumber(ctx, workNumber)
	if idErr != nil {
		return nil, idErr
	}

	dbSteps, dbStepErr := ae.DB.GetTaskSteps(ctx, workID)
	if dbStepErr != nil {
		return nil, dbStepErr
	}

	dbTask.Steps = dbSteps

	return dbTask, nil
}

func (ae *Env) GetTaskForRestart(ctx c.Context, workNumber string) (task *e.EriusTask, err error) {
	dbTask, taskErr := ae.DB.GetTask(
		ctx,
		[]string{""},
		[]string{""},
		"",
		workNumber,
	)
	if taskErr != nil {
		return nil, taskErr
	}

	workID, idErr := ae.DB.GetWorkIDByWorkNumber(ctx, workNumber)
	if idErr != nil {
		return nil, idErr
	}

	dbSteps, dbStepErr := ae.DB.GetNotSkippedTaskSteps(ctx, workID)
	if dbStepErr != nil {
		return nil, dbStepErr
	}

	dbTask.Steps = dbSteps

	return dbTask, nil
}
