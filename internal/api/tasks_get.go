package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/pkg/errors"

	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	ht "gitlab.services.mts.ru/jocasta/pipeliner/internal/human-tasks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sla"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/user"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
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
	Id                 string                 `json:"id"`
	ButtonType         string                 `json:"button_type"`
	Title              string                 `json:"title"`
	CommentEnabled     bool                   `json:"comment_enabled"`
	AttachmentsEnabled bool                   `json:"attachments_enabled"`
	Params             map[string]interface{} `json:"params,omitempty"`
}

type taskActions []action
type taskSteps []step

func (taskResp) toResponse(in *entity.EriusTask, usrDegSteps map[string]bool, sNames map[string]string, dln time.Time) *taskResp {
	steps := make([]step, 0, len(in.Steps))
	actions := make([]action, 0, len(in.Actions))
	for i := range in.Steps {
		steps = append(steps, step{
			Time:                      in.Steps[i].Time,
			UpdateTime:                in.Steps[i].UpdatedAt,
			Type:                      in.Steps[i].Type,
			Name:                      in.Steps[i].Name,
			State:                     in.Steps[i].State,
			Storage:                   in.Steps[i].Storage,
			Errors:                    in.Steps[i].Errors,
			Steps:                     in.Steps[i].Steps,
			HasError:                  in.Steps[i].HasError,
			Status:                    pipeline.Status(in.Steps[i].Status),
			IsDelegateOfAnyStepMember: usrDegSteps[in.Steps[i].Name],
			ShortTitle:                sNames[in.Steps[i].Name],
		})
	}

	for _, a := range in.Actions {
		actions = append(actions, action{
			Id:                 a.Id,
			ButtonType:         a.ButtonType,
			Title:              a.Title,
			CommentEnabled:     a.CommentEnabled,
			AttachmentsEnabled: a.AttachmentsEnabled,
			Params:             a.Params,
		})
	}

	out := &taskResp{
		ID:                 in.ID,
		VersionID:          in.VersionID,
		StartedAt:          in.StartedAt,
		LastChangedAt:      in.LastChangedAt,
		FinishedAt:         in.FinishedAt,
		Name:               in.Name,
		Description:        in.Description,
		Status:             in.Status,
		HumanStatus:        in.HumanStatus,
		HumanStatusComment: in.HumanStatusComment,
		Author:             in.Author,
		IsDebugMode:        in.IsDebugMode,
		Parameters:         in.Parameters,
		Steps:              steps,
		WorkNumber:         in.WorkNumber,
		BlueprintID:        in.BlueprintID,
		Rate:               in.Rate,
		RateComment:        in.RateComment,
		AvailableActions:   actions,
		StatusComment:      in.StatusComment,
		StatusAuthor:       in.StatusAuthor,
		ProcessDeadline:    dln,
		NodeGroup:          groupsToResponce(in.NodeGroup),
	}

	return out
}

func groupsToResponce(groups []*entity.NodeGroup) []NodeGroup {
	if groups == nil {
		return nil
	}
	var resp []NodeGroup
	for i := range groups {
		insideNodes := groupsToResponce(groups[i].Nodes)
		resp = append(resp, NodeGroup{
			EndNode:   groups[i].EndNode,
			Nodes:     &insideNodes,
			Prev:      &groups[i].Prev,
			StartNode: groups[i].StartNode,
		})
	}
	return resp
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

	var parsedContent EriusScenario
	err = json.Unmarshal([]byte(dbTask.VersionContent), &parsedContent)
	if err != nil {
		e := PipelineParseError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}

	if len(dbTask.NodeGroup) == 0 {
		scenario, getVersionErr := ae.DB.GetVersionByWorkNumber(ctx, dbTask.WorkNumber)
		if getVersionErr != nil {
			e := UnknownError
			log.Error(e.errorMessage(getVersionErr))
			_ = e.sendError(w)

			return
		}
		groups := scenario.Pipeline.Blocks.GetGroups()
		updateGroupsErr := ae.DB.UpdateGroupsForEmptyVersions(ctx, scenario.VersionID.String(), groups)
		if updateGroupsErr != nil {
			e := UnknownError
			log.Error(e.errorMessage(updateGroupsErr))
			_ = e.sendError(w)

			return
		}

		dbTask.NodeGroup = groups
	}

	steps, err := ae.DB.GetTaskSteps(ctx, dbTask.ID)
	if err != nil {
		e := GetTaskError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	shortNameMap := map[string]string{}
	for key, val := range parsedContent.Pipeline.Blocks.AdditionalProperties {
		if val.ShortTitle != nil {
			shortNameMap[key] = *val.ShortTitle
		} else {
			shortNameMap[key] = ""
		}
	}
	dbTask.Steps = steps

	currentUserDelegateSteps, tErr := ae.getCurrentUserInDelegatesForSteps(ui.Username, &steps, &delegations)
	if tErr != nil {
		e := GetDelegationsError
		log.Error(e.errorMessage(tErr))
		_ = e.sendError(w)

		return
	}
	scenario, getVersionErr := ae.DB.GetVersionByWorkNumber(ctx, dbTask.WorkNumber)
	if getVersionErr != nil {
		e := UnknownError
		log.Error(e.errorMessage(getVersionErr))
		_ = e.sendError(w)

		return
	}
	if ui.Username != scenario.Author {
		hideErr := ae.hideExecutorsFromInitiator(dbTask.Steps, ui.Username, dbTask.Author, currentUserDelegateSteps)
		if hideErr != nil {
			e := UnknownError
			log.Error(e.errorMessage(hideErr))
			_ = e.sendError(w)

			return
		}
	}
	versionSettings, errSla := ae.DB.GetSlaVersionSettings(ctx, dbTask.VersionID.String())
	if errSla != nil {
		e := GetProcessSlaSettingsError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	slaInfoPtr, getSlaInfoErr := ae.SLAService.GetSLAInfoPtr(ctx, sla.InfoDto{
		TaskCompletionIntervals: []entity.TaskCompletionInterval{
			{
				StartedAt:  dbTask.StartedAt,
				FinishedAt: dbTask.StartedAt.Add(time.Hour * 24 * 100),
			},
		},
		WorkType: sla.WorkHourType(versionSettings.WorkType),
	})
	if getSlaInfoErr != nil {
		e := UnknownError
		log.Error(e.errorMessage(getSlaInfoErr))
		_ = e.sendError(w)

		return
	}

	deadline := ae.SLAService.ComputeMaxDate(dbTask.StartedAt, float32(versionSettings.Sla), slaInfoPtr)

	resp := &taskResp{}
	if err = sendResponse(w, http.StatusOK,
		resp.toResponse(dbTask, currentUserDelegateSteps, shortNameMap, deadline)); err != nil {
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

	delegations, err := ae.HumanTasks.GetDelegationsToLogin(ctx, filters.CurrentUser)
	if err != nil {
		e := GetDelegationsError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}

	if filters.SelectAs != nil {
		switch *filters.SelectAs {
		case entity.SelectAsValApprover:
			delegations = delegations.FilterByType("approvement")
		case entity.SelectAsValExecutor:
			delegations = delegations.FilterByType("execution")
		default:
			delegations = delegations[:0]
		}
	} else {
		delegations = delegations[:0]
	}

	users := delegations.GetUserInArrayWithDelegators([]string{filters.CurrentUser})

	if filters.ProcessingLogins != nil && len(*filters.ProcessingLogins) > 0 {
		delegations, err = ae.HumanTasks.GetDelegationsToLogins(ctx, *filters.ProcessingLogins)
		if err != nil {
			e := GetDelegationsError
			log.Error(e.errorMessage(err))
			_ = e.sendError(w)
			return
		}

		users = delegations.GetUserInArrayWithDelegators(*filters.ProcessingLogins)
	}

	if filters.Status != nil {
		ss := strings.Split(*filters.Status, ",")
		uniqueS := make(map[pipeline.TaskHumanStatus]struct{})
		for _, status := range ss {
			uniqueS[pipeline.TaskHumanStatus(strings.Trim(status, "'"))] = struct{}{}
		}
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

	resp, err := ae.DB.GetTasks(ctx, filters, users)
	if err != nil {
		e := GetTasksError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	versionsSLA := make(map[string]*entity.SlaVersionSettings)

	for i := range resp.Tasks {
		if _, exists := versionsSLA[resp.Tasks[i].VersionID.String()]; !exists {
			versionSettings, errSla := ae.DB.GetSlaVersionSettings(ctx, resp.Tasks[i].VersionID.String())
			if errSla != nil {
				e := GetProcessSlaSettingsError
				log.Error(e.errorMessage(err))
				_ = e.sendError(w)

				return
			}

			versionsSLA[resp.Tasks[i].VersionID.String()] = &versionSettings
		}

		slaInfoPtr, getSlaInfoErr := ae.SLAService.GetSLAInfoPtr(ctx, sla.InfoDto{
			TaskCompletionIntervals: []entity.TaskCompletionInterval{
				{
					StartedAt:  resp.Tasks[i].StartedAt,
					FinishedAt: resp.Tasks[i].StartedAt.Add(time.Hour * 24 * 100),
				},
			},
			WorkType: sla.WorkHourType(versionsSLA[resp.Tasks[i].VersionID.String()].WorkType),
		})
		if getSlaInfoErr != nil {
			e := UnknownError
			log.Error(e.errorMessage(getSlaInfoErr))
			_ = e.sendError(w)

			return
		}

		deadline := ae.SLAService.ComputeMaxDate(resp.Tasks[i].StartedAt, float32(versionsSLA[resp.Tasks[i].VersionID.String()].Sla), slaInfoPtr)
		resp.Tasks[i].ProcessDeadline = deadline
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

	if p.ProcessingLogins != nil && p.ProcessedLogins != nil {
		return filters, errors.New("can't filter by processingLogins and processedLogins at the same time")
	}
	var selectAs *string
	if p.SelectAs != nil {
		at := string(*p.SelectAs)
		selectAs = &at
		if *selectAs != entity.SelectAsValApprover &&
			*selectAs != entity.SelectAsValFinishedApprover &&
			*selectAs != entity.SelectAsValExecutor &&
			*selectAs != entity.SelectAsValFinishedExecutor &&
			*selectAs != entity.SelectAsValFormExecutor &&
			*selectAs != entity.SelectAsValFinishedFormExecutor &&
			*selectAs != entity.SelectAsValSignerPhys &&
			*selectAs != entity.SelectAsValFinishedSignerPhys &&
			*selectAs != entity.SelectAsValSignerJur &&
			*selectAs != entity.SelectAsValFinishedSignerJur &&
			*selectAs != entity.SelectAsValInitiators &&
			*selectAs != entity.SelectAsValGroupExecutor &&
			*selectAs != entity.SelectAsValFinishedGroupExecutor {
			return filters, errors.New("invalid value in SelectAs filter")
		}
	}

	if p.ProcessingGroupIds != nil && p.ProcessedGroupIds != nil {
		return filters, errors.New("can't filter by processingGroupIds and processedGroupIds at the same time")
	}

	ui, err := user.GetEffectiveUserInfoFromCtx(req.Context())
	if err != nil {
		return filters, err
	}

	filters.CurrentUser = ui.Username
	limit, offset := parseLimitOffsetWithDefault(p.Limit, p.Offset)

	filters.GetTaskParams = entity.GetTaskParams{
		Name:                 p.Name,
		Created:              p.Created.toEntity(),
		Order:                p.Order,
		Limit:                &limit,
		Offset:               &offset,
		TaskIDs:              p.TaskIDs,
		SelectAs:             selectAs,
		Archived:             p.Archived,
		ForCarousel:          p.ForCarousel,
		Status:               statusToEntity(p.Status),
		Receiver:             p.Receiver,
		HasAttachments:       p.HasAttachments,
		SelectFor:            p.SelectFor,
		InitiatorLogins:      p.InitiatorLogins,
		ProcessingLogins:     p.ProcessingLogins,
		ProcessingGroupIds:   p.ProcessingGroupIds,
		ProcessedLogins:      p.ProcessedLogins,
		ProcessedGroupIds:    p.ProcessedGroupIds,
		ExecutorTypeAssigned: typeAssigned,
		SignatureCarrier:     signatureCarrier,
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

	return []string{}
}

func (ae *APIEnv) GetTaskMeanSolveTime(w http.ResponseWriter, req *http.Request, pipelineId string) {
	ctx, s := trace.StartSpan(req.Context(), "get_task_mean_solve_time")
	defer s.End()

	log := logger.GetLogger(ctx).WithField("pipelineId", pipelineId)

	taskTimeIntervals, intervalsErr := ae.DB.GetMeanTaskSolveTime(ctx, pipelineId) // it returns ordered by created_at
	if intervalsErr != nil {
		e := GetTaskError
		log.Error(e.errorMessage(intervalsErr))
		_ = e.sendError(w)
		return
	}
	if len(taskTimeIntervals) == 0 {
		if err := sendResponse(w, http.StatusOK, script.TaskSolveTime{MeanWorkHours: 0}); err != nil {
			e := UnknownError
			log.Error(e.errorMessage(err))
			_ = e.sendError(w)
		}
		return
	}

	calendarDays, err := ae.HrGate.GetDefaultCalendarDaysForGivenTimeIntervals(ctx, taskTimeIntervals)
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}

	var mean = ae.SLAService.ComputeMeanTaskCompletionTime(taskTimeIntervals, *calendarDays)

	if err = sendResponse(w, http.StatusOK, script.TaskSolveTime{MeanWorkHours: mean.MeanWorkHours}); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

func (ae *APIEnv) hideExecutorsFromInitiator(steps entity.TaskSteps, requesterLogin, author string, delegates map[string]bool) error {
	for stepIndex := range steps {
		currentStep := steps[stepIndex]
		if currentStep.State == nil {
			continue
		}
		switch currentStep.Type {
		case pipeline.BlockGoFormID:
			{
				if author == requesterLogin {
					var formBlock pipeline.FormData
					unmarshalErr := json.Unmarshal(currentStep.State[currentStep.Name], &formBlock)
					if unmarshalErr != nil {
						return unmarshalErr
					}

					if !formBlock.HideExecutorFromInitiator || slices.Contains(maps.Keys(formBlock.Executors), requesterLogin) {
						continue
					}
					formBlock.Executors = map[string]struct{}{
						hiddenUserLogin: {},
					}
					formBlock.ActualExecutor = utils.GetAddressOfValue(hiddenUserLogin)

					for historyIdx := range formBlock.ChangesLog {
						formBlock.ChangesLog[historyIdx].Executor = hiddenUserLogin
						formBlock.ChangesLog[historyIdx].DelegateFor = hiddenUserLogin
					}
					data, marshalErr := json.Marshal(formBlock)
					if marshalErr != nil {
						return marshalErr
					}
					currentStep.State[currentStep.Name] = data
				}
			}
		case pipeline.BlockGoExecutionID:
			{
				if delegates[currentStep.Name] {
					continue
				}
				var execBlock pipeline.ExecutionData
				unmarshalErr := json.Unmarshal(currentStep.State[currentStep.Name], &execBlock)
				if unmarshalErr != nil {
					return unmarshalErr
				}
				if execBlock.ShowExecutor || slices.Contains(maps.Keys(execBlock.Executors), requesterLogin) {
					continue
				}
				execBlock.Executors = map[string]struct{}{
					hiddenUserLogin: {},
				}
				execBlock.ActualExecutor = utils.GetAddressOfValue(hiddenUserLogin)

				for i := range execBlock.ChangedExecutorsLogs {
					execBlock.ChangedExecutorsLogs[i].OldLogin = hiddenUserLogin
					execBlock.ChangedExecutorsLogs[i].NewLogin = hiddenUserLogin
				}

				for i := range execBlock.RequestExecutionInfoLogs {
					execBlock.RequestExecutionInfoLogs[i].Login = hiddenUserLogin
					execBlock.RequestExecutionInfoLogs[i].DelegateFor = hiddenUserLogin
				}

				for i := range execBlock.EditingAppLog {
					execBlock.EditingAppLog[i].Executor = hiddenUserLogin
					execBlock.EditingAppLog[i].DelegateFor = hiddenUserLogin
				}

				data, marshalErr := json.Marshal(execBlock)
				if marshalErr != nil {
					return marshalErr
				}
				currentStep.State[currentStep.Name] = data
			}
		}
	}

	return nil
}
