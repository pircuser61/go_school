package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/google/uuid"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/user"
)

type eriusTaskResponse struct {
	ID            uuid.UUID              `json:"id"`
	VersionID     uuid.UUID              `json:"version_id"`
	StartedAt     time.Time              `json:"started_at"`
	LastChangedAt time.Time              `json:"last_changed_at"`
	FinishedAt    *time.Time             `json:"finished_at"`
	Name          string                 `json:"name"`
	Description   string                 `json:"description"`
	Status        string                 `json:"status"`
	HumanStatus   string                 `json:"human_status"`
	Author        string                 `json:"author"`
	IsDebugMode   bool                   `json:"debug"`
	Parameters    map[string]interface{} `json:"parameters"`
	Steps         taskSteps              `json:"steps"`
	WorkNumber    string                 `json:"work_number"`
	BlueprintID   string                 `json:"blueprint_id"`
	Rate          int                    `json:"rate"`
	RateComment   string                 `json:"rate_comment"`
}

type step struct {
	Time     time.Time                  `json:"time"`
	Type     string                     `json:"type"`
	Name     string                     `json:"name"`
	State    map[string]json.RawMessage `json:"state" swaggertype:"object"`
	Storage  map[string]interface{}     `json:"storage"`
	Errors   []string                   `json:"errors"`
	Steps    []string                   `json:"steps"`
	HasError bool                       `json:"has_error"`
	Status   pipeline.Status            `json:"status"`
}

type taskSteps []step

func (eriusTaskResponse) toResponse(in *entity.EriusTask) *eriusTaskResponse {
	steps := make([]step, 0, len(in.Steps))
	for i := range in.Steps {
		actionTime := in.Steps[i].Time

		if in.Steps[i].UpdatedAt != nil {
			actionTime = *in.Steps[i].UpdatedAt
		}

		steps = append(steps, step{
			Time:     actionTime,
			Type:     in.Steps[i].Type,
			Name:     in.Steps[i].Name,
			State:    in.Steps[i].State,
			Storage:  in.Steps[i].Storage,
			Errors:   in.Steps[i].Errors,
			Steps:    in.Steps[i].Steps,
			HasError: in.Steps[i].HasError,
			Status:   pipeline.Status(in.Steps[i].Status),
		})
	}

	out := &eriusTaskResponse{
		ID:            in.ID,
		VersionID:     in.VersionID,
		StartedAt:     in.StartedAt,
		LastChangedAt: in.LastChangedAt,
		FinishedAt:    in.FinishedAt,
		Name:          in.Name,
		Description:   in.Description,
		Status:        in.Status,
		HumanStatus:   in.HumanStatus,
		Author:        in.Author,
		IsDebugMode:   in.IsDebugMode,
		Parameters:    in.Parameters,
		Steps:         steps,
		WorkNumber:    in.WorkNumber,
		BlueprintID:   in.BlueprintID,
		Rate:          in.Rate,
		RateComment:   in.RateComment,
	}

	return out
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

	dbTask, err := ae.DB.GetTask(ctx, workNumber)
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

	resp := &eriusTaskResponse{}
	if err = sendResponse(w, http.StatusOK, resp.toResponse(dbTask)); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

//nolint:dupl //its not duplicate
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

	resp, err := ae.DB.GetTasks(ctx, filters)
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
		Name:          p.Name,
		Created:       p.Created.toEntity(),
		Order:         p.Order,
		Limit:         &limit,
		Offset:        &offset,
		TaskIDs:       p.TaskIDs,
		SelectAs:      p.SelectAs,
		Archived:      p.Archived,
		ForCarousel:   p.ForCarousel,
		Status:        statusToEntity(p.Status),
		Receiver:      p.Receiver,
		HasAttacments: p.HasAttachments,
	}

	return filters, nil
}

func (c *Created) toEntity() *entity.TimePeriod {
	var timePeriod *entity.TimePeriod
	if c != nil {
		timePeriod = &entity.TimePeriod{
			Start: c.Start,
			End:   c.End,
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

	resp, err := ae.DB.GetTasksCount(ctx, ui.Username)
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

	if err = updateData.Validate(); err != nil {
		e := UpdateTaskValidationError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	blockTypes := getTaskStepNameByAction(updateData.Action)
	if len(blockTypes) == 0 {
		e := UpdateTaskValidationError
		log.Error(e.errorMessage(nil))
		_ = e.sendError(w)

		return
	}

	dbTask, err := ae.DB.GetTask(ctx, workNumber)
	if err != nil {
		e := GetTaskError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if !dbTask.IsRun() {
		e := UpdateNotRunningTaskError
		log.Error(e.errorMessage(nil))
		_ = e.sendError(w)

		return
	}

	scenario, err := ae.DB.GetPipelineVersion(ctx, dbTask.VersionID)
	if err != nil {
		e := GetVersionError
		log.Error(e.errorMessage(nil))
		_ = e.sendError(w)

		return
	}

	var steps entity.TaskSteps
	for _, blockType := range blockTypes {
		stepsByBlock, er := ae.DB.GetUnfinishedTaskStepsByWorkIdAndStepType(ctx, dbTask.ID, blockType)
		if er != nil {
			e := GetTaskError
			log.Error(e.errorMessage(er))
			_ = e.sendError(w)
			return
		}
		steps = append(steps, stepsByBlock...)
	}

	if len(steps) == 0 {
		e := GetTaskError
		log.Error(e.errorMessage(nil))
		_ = e.sendError(w)

		return
	}
	if updateData.Action == entity.TaskUpdateActionCancelApp {
		steps = steps[:1]
	}

	ep := pipeline.ExecutablePipeline{
		Storage:       ae.DB,
		Remedy:        ae.Remedy,
		FaaS:          ae.FaaS,
		HTTPClient:    ae.HTTPClient,
		PipelineID:    scenario.ID,
		VersionID:     scenario.VersionID,
		EntryPoint:    scenario.Pipeline.Entrypoint,
		Sender:        ae.Mail,
		People:        ae.People,
		ServiceDesc:   ae.ServiceDesc,
		PipelineModel: &entity.EriusScenario{Author: dbTask.Author},
	}

	couldUpdateOne := false
	for _, item := range steps {
		blockFunc, ok := scenario.Pipeline.Blocks[item.Name]
		if !ok {
			e := BlockNotFoundError
			log.Error(e.errorMessage(nil))
			_ = e.sendError(w)

			return
		}

		block, blockErr := ep.CreateBlock(ctx, item.Name, &blockFunc)
		if blockErr != nil {
			e := UpdateBlockError
			log.Error(e.errorMessage(blockErr))
			_ = e.sendError(w)

			return
		}

		_, blockErr = block.Update(ctx, &script.BlockUpdateData{
			Id:         item.ID,
			ByLogin:    ui.Username,
			Action:     string(updateData.Action),
			Parameters: updateData.Parameters,
			WorkNumber: dbTask.WorkNumber,
			WorkTitle:  dbTask.Name,
			Author:     dbTask.Author,
		})
		if blockErr == nil {
			couldUpdateOne = true
		} else {
			log.Error("block.Update: ", blockErr, updateData.Parameters)
		}
	}

	if !couldUpdateOne {
		e := UpdateBlockError
		log.Error(e.errorMessage(errors.New("couldn't update work")))
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

type RateApplication struct {
	Rate    int    `json:"rate"`
	Comment string `json:"comment"`
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

	req := &RateApplication{}
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

	return []string{}
}
