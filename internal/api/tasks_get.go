package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/pkg/errors"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	ht "gitlab.services.mts.ru/jocasta/pipeliner/internal/humantasks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/metrics"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sla"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/stephandlers"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/user"
)

const (
	hiddenUserLogin = "hidden_user"
)

type taskResp struct {
	ID                 uuid.UUID              `json:"id"`
	VersionID          uuid.UUID              `json:"version_id"`
	StartedAt          time.Time              `json:"started_at"`
	LastChangedAt      time.Time              `json:"last_changed_at"`
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
	task         *entity.EriusTask
	usrDegSteps  map[string]bool
	sNames       map[string]string
	dln          time.Time
	approvalList []entity.ApprovalListSettings
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

func groupsToResponse(groups []*entity.NodeGroup) []NodeGroup {
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

	log := logger.GetLogger(ctx)
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

const taskPath = "/tasks/{workNumber}"

func (ae *Env) GetTask(w http.ResponseWriter, req *http.Request, workNumber string) {
	start := time.Now()
	ctx, s := trace.StartSpan(req.Context(), "get_task")

	requestInfo := metrics.NewGetRequestInfo(taskPath)

	defer func() {
		s.End()

		requestInfo.Duration = time.Since(start)

		ae.Metrics.RequestsIncrease(requestInfo)
	}()

	log := logger.GetLogger(ctx)
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

	delegationsByApprovement := delegations.FilterByType("approvement")
	delegationsByExecution := delegations.FilterByType("execution")

	dbTask, err := ae.DB.GetTask(
		ctx,
		delegationsByApprovement.GetUserInArrayWithDelegators([]string{ui.Username}),
		delegationsByExecution.GetUserInArrayWithDelegators([]string{ui.Username}),
		ui.Username,
		workNumber,
	)
	if err != nil {
		errorHandler.handleError(GetTaskError, err)

		return
	}

	var parsedContent EriusScenario

	err = json.Unmarshal([]byte(dbTask.VersionContent), &parsedContent)
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

	shortNameMap := shortNameMap(parsedContent.Pipeline.Blocks.AdditionalProperties)

	dbTask.Steps = steps

	if ui.Username != dbTask.Author {
		accessibleForms, ttErr := ae.getAccessibleForms(ui.Username, &steps, &delegations)
		if ttErr != nil {
			errorHandler.handleError(GetDelegationsError, ttErr)

			return
		}

		ae.removeForms(dbTask, accessibleForms)
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

	if ui.Username != scenario.Author {
		hideErr := ae.hideExecutors(ctx, dbTask, ui.Username, currentUserDelegateSteps, ui.Username == dbTask.Author)
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
		TaskCompletionIntervals: []entity.TaskCompletionInterval{
			{
				StartedAt:  dbTask.StartedAt,
				FinishedAt: dbTask.StartedAt.Add(time.Hour * 24 * 100),
			},
		},
		WorkType: sla.WorkHourType(versionSettings.WorkType),
	}

	slaInfoPtr, getSLAInfoErr := ae.SLAService.GetSLAInfoPtr(
		ctx,
		slaInfoDTO,
	)
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

	resp := &taskResp{}

	toResponse := &taskToResponseDTO{
		task:         dbTask,
		usrDegSteps:  currentUserDelegateSteps,
		sNames:       shortNameMap,
		dln:          deadline,
		approvalList: approvalLists,
	}

	err = sendResponse(w, http.StatusOK, resp.toResponse(toResponse))
	if err != nil {
		errorHandler.handleError(UnknownError, err)
	}
}

func shortNameMap(additionalProperties map[string]EriusFunc) map[string]string {
	shortNameMap := make(map[string]string, len(additionalProperties))

	for key, val := range additionalProperties {
		if val.ShortTitle != nil {
			shortNameMap[key] = *val.ShortTitle
		} else {
			shortNameMap[key] = ""
		}
	}

	return shortNameMap
}

func (ae *Env) handleZeroTaskNodeGroup(ctx context.Context, dbTask *entity.EriusTask) error {
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

func (ae *Env) getAccessibleForms(
	currentUser string,
	steps *entity.TaskSteps,
	delegates *ht.Delegations,
) (accessibleForms map[string]struct{}, err error) {
	const (
		ApproverBlockType  = "approver"
		ExecutionBlockType = "execution"
		FormBlockType      = "form"
		SignBlockType      = "sign"
	)

	accessibleForms = make(map[string]struct{}, 0)

	// это костыль но он вынужденный потому что в тестах подразумевается что функцию можно вызвать с nil delegates
	if delegates == nil {
		delegates = &ht.Delegations{}
	}

	stepHandler := stephandlers.NewMultipleTypesStepHandler()

	stepHandler.RegisterStepTypeHandler(
		ApproverBlockType,
		stephandlers.NewAccessibleFormsApproverBlockStepHandler(currentUser, accessibleForms, *delegates),
	)
	stepHandler.RegisterStepTypeHandler(
		FormBlockType,
		stephandlers.NewAccessibleFormsFormBlockStepHandler(currentUser, accessibleForms),
	)
	stepHandler.RegisterStepTypeHandler(
		ExecutionBlockType,
		stephandlers.NewAccessibleFormsExecutionBlockStepHandler(currentUser, accessibleForms, *delegates),
	)
	stepHandler.RegisterStepTypeHandler(
		SignBlockType,
		stephandlers.NewAccessibleFormsSignBlockStepHandler(currentUser, accessibleForms),
	)

	err = stepHandler.HandleSteps(*steps)
	if err != nil {
		return nil, err
	}

	return accessibleForms, nil
}

func (ae *Env) getCurrentUserInDelegatesForSteps(
	currentUser string,
	steps *entity.TaskSteps,
	delegates *ht.Delegations,
) (userInDelegates map[string]bool, err error) {
	const (
		ApproverBlockType  = "approver"
		ExecutionBlockType = "execution"
		FormBlockType      = "form"
	)

	userInDelegates = make(map[string]bool, 0)

	stepHandler := stephandlers.NewMultipleTypesStepHandler()

	stepHandler.RegisterStepTypeHandler(
		ApproverBlockType,
		stephandlers.NewUserInDelegatesApproverBlockTypeStepHandler(currentUser, *delegates, userInDelegates),
	)

	executionFormBlockTypeStepHandler := stephandlers.NewUserInDelegatesExecutionFromBlockTypesStepHandler(
		currentUser,
		*delegates,
		userInDelegates,
	)

	stepHandler.RegisterStepTypeHandler(ExecutionBlockType, executionFormBlockTypeStepHandler)
	stepHandler.RegisterStepTypeHandler(FormBlockType, executionFormBlockTypeStepHandler)

	err = stepHandler.HandleSteps(*steps)
	if err != nil {
		return nil, err
	}

	return userInDelegates, nil
}

const getTasksPath = "/tasks"

//nolint:dupl,gocritic //its not duplicate // params без поинтера нужен для интерфейса
func (ae *Env) GetTasks(w http.ResponseWriter, req *http.Request, params GetTasksParams) {
	start := time.Now()
	ctx, s := trace.StartSpan(req.Context(), "get_tasks")

	requestInfo := metrics.NewGetRequestInfo(getTasksPath)

	defer func() {
		s.End()

		requestInfo.Duration = time.Since(start)

		ae.Metrics.RequestsIncrease(requestInfo)
	}()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)
	errorHandler.setMetricsRequestInfo(requestInfo)

	filters, err := params.toEntity(req)
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
		case entity.SelectAsValApprover:
			delegations = delegations.FilterByType("approvement")
		case entity.SelectAsValExecutor, entity.SelectAsValQueueExecutor, entity.SelectAsValInWorkExecutor:
			delegations = delegations.FilterByType("execution")
		default:
			delegations = delegations[:0]
		}
	} else {
		delegations = delegations[:0]
	}

	users := delegations.GetUserInArrayWithDelegators([]string{filters.CurrentUser})

	if filters.Status != nil {
		handleFilterStatus(&filters)
	}

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
	}

	if err = sendResponse(w, http.StatusOK, resp); err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

func (p *GetTasksParams) toEntity(req *http.Request) (entity.TaskFilter, error) {
	var filters entity.TaskFilter

	var typeAssigned *string

	if p.ExecutorTypeAssigned != nil {
		at := string(*p.ExecutorTypeAssigned)

		typeAssigned = &at
		if *typeAssigned != entity.AssignedToMe && *typeAssigned != entity.AssignedByMe {
			return filters, errors.New("invalid value in typeAssigned filter")
		}
	}

	var signatureCarrier *string

	if p.SignatureCarrier != nil {
		at := string(*p.SignatureCarrier)

		signatureCarrier = &at
		if *signatureCarrier != entity.SignatureCarrierCloud &&
			*signatureCarrier != entity.SignatureCarrierToken &&
			*signatureCarrier != entity.SignatureCarrierAll {
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

	filters.GetTaskParams = entity.GetTaskParams{
		Name:                 p.Name,
		Created:              p.Created.toEntity(),
		Order:                p.Order,
		Limit:                &limit,
		Offset:               &offset,
		TaskIDs:              p.TaskIDs,
		SelectAs:             &selectAs,
		Archived:             p.Archived,
		ForCarousel:          p.ForCarousel,
		Status:               statusToEntity(p.Status),
		Receiver:             p.Receiver,
		HasAttachments:       p.HasAttachments,
		Initiator:            p.Initiator,
		InitiatorLogins:      p.InitiatorLogins,
		ProcessingLogins:     p.ProcessingLogins,
		ProcessingGroupIds:   p.ProcessingGroupIds,
		ExecutorTypeAssigned: typeAssigned,
		SignatureCarrier:     signatureCarrier,
	}

	return filters, nil
}

func handleFilterStatus(filters *entity.TaskFilter) {
	ss := strings.Split(*filters.Status, ",")

	uniqueS := make(map[pipeline.TaskHumanStatus]struct{})
	for _, status := range ss {
		uniqueS[pipeline.TaskHumanStatus(strings.Trim(status, "'"))] = struct{}{}
	}

	//nolint:exhaustive // раз не надо было обрабатывать остальные случаи значит не надо // правильно, не уважаю этот линтер
	for status := range uniqueS {
		switch status {
		case pipeline.StatusRejected:
			uniqueS[pipeline.StatusApprovementRejected] = struct{}{}
		case pipeline.StatusApprovementRejected:
			uniqueS[pipeline.StatusRejected] = struct{}{}
		default:
			continue
		}
	}

	newSS := make([]string, 0, len(uniqueS))

	for status := range uniqueS {
		newSS = append(newSS, "'"+string(status)+"'")
	}

	newStatuses := strings.Join(newSS, ",")
	filters.Status = &newStatuses
}

func selectAsValid(selectAs string) bool {
	switch selectAs {
	case entity.SelectAsValApprover,
		entity.SelectAsValFinishedApprover,
		entity.SelectAsValExecutor,
		entity.SelectAsValFinishedExecutor,
		entity.SelectAsValFormExecutor,
		entity.SelectAsValFinishedFormExecutor,
		entity.SelectAsValSignerPhys,
		entity.SelectAsValFinishedSignerPhys,
		entity.SelectAsValSignerJur,
		entity.SelectAsValFinishedSignerJur,
		entity.SelectAsValInitiators,
		entity.SelectAsValGroupExecutor,
		entity.SelectAsValFinishedGroupExecutor,
		entity.SelectAsValQueueExecutor,
		entity.SelectAsValInWorkExecutor:
		return true
	}

	return false
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

func (ae *Env) GetTasksCount(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "get_tasks_count")
	defer s.End()

	log := logger.GetLogger(ctx)
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

func getTaskStepNameByAction(action entity.TaskUpdateAction) []string {
	if action == entity.TaskUpdateActionAdditionalApprovement {
		return []string{pipeline.BlockGoApproverID, pipeline.BlockGoSignID}
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

	if action == entity.TaskUpdateActionReplyApproverInfo {
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

	if action == entity.TaskUpdateActionReplyExecutionInfo {
		return []string{pipeline.BlockGoExecutionID}
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
		return []string{pipeline.BlockGoApproverID, pipeline.BlockGoSignID}
	}

	if action == entity.TaskUpdateActionFormExecutorStartWork {
		return []string{pipeline.BlockGoFormID}
	}

	if action == entity.TaskUpdateActionSign {
		return []string{pipeline.BlockGoSignID}
	}

	if action == entity.TaskUpdateActionSignChangeWorkStatus {
		return []string{pipeline.BlockGoSignID}
	}

	if action == entity.TaskUpdateActionFinishTimer {
		return []string{pipeline.BlockTimerID}
	}

	if action == entity.TaskUpdateActionFuncSLAExpired {
		return []string{pipeline.BlockExecutableFunctionID}
	}

	if action == entity.TaskUpdateActionRetry {
		return []string{pipeline.BlockExecutableFunctionID}
	}

	return []string{}
}

func (ae *Env) GetTaskMeanSolveTime(w http.ResponseWriter, req *http.Request, pipelineID string) {
	ctx, s := trace.StartSpan(req.Context(), "get_task_mean_solve_time")
	defer s.End()

	log := logger.GetLogger(ctx).WithField("pipelineId", pipelineID)
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

func (ae *Env) removeForms(dbTask *entity.EriusTask, accessibleForms map[string]struct{}) {
	actualSteps := make([]*entity.Step, 0, len(dbTask.Steps))

	for _, step := range dbTask.Steps {
		if _, ok := accessibleForms[step.Name]; !ok && step.Type == "form" {
			continue
		}

		newSteps := make([]string, 0, len(step.Steps))

		for _, s := range step.Steps {
			if _, ok := accessibleForms[s]; !ok && strings.HasPrefix(s, "form") {
				continue
			}

			newSteps = append(newSteps, s)
		}

		step.Steps = newSteps

		newStates := make(map[string]json.RawMessage, len(step.State))

		for k, v := range step.State {
			if _, ok := accessibleForms[k]; !ok && strings.HasPrefix(k, "form") {
				continue
			}

			newStates[k] = v
		}

		for k := range step.Storage {
			key := strings.Split(k, ".")

			if _, ok := accessibleForms[key[0]]; !ok && strings.HasPrefix(k, "form") {
				delete(step.Storage, k)
			}
		}

		step.State = newStates
		actualSteps = append(actualSteps, step)
	}

	dbTask.Steps = actualSteps
}

func (ae *Env) hideExecutors(
	ctx context.Context, dbTask *entity.EriusTask, requesterLogin string, stepDelegates map[string]bool, isInitiator bool,
) error {
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
		stephandlers.NewHideExecutorsFormBlockStepHandler(stepDelegates, members, requesterLogin, isInitiator),
	)

	stepHandler.RegisterStepTypeHandler(
		pipeline.BlockGoExecutionID,
		stephandlers.NewHideExecutorsExecutionBlockStepHandler(stepDelegates, members, requesterLogin, isInitiator),
	)

	err := stepHandler.HandleSteps(dbTask.Steps)
	if err != nil {
		return err
	}

	return nil
}
