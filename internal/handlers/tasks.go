package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/go-chi/chi/v5"

	"github.com/google/uuid"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

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
	Name          string                 `json:"name"`
	Status        string                 `json:"status"`
	HumanStatus   string                 `json:"human_status"`
	Author        string                 `json:"author"`
	IsDebugMode   bool                   `json:"debug"`
	Parameters    map[string]interface{} `json:"parameters"`
	Steps         taskSteps              `json:"steps"`
	WorkNumber    string                 `json:"work_number"`
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
		steps = append(steps, step{
			Time:     in.Steps[i].Time,
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
		Name:          in.Name,
		Status:        in.Status,
		HumanStatus:   in.HumanStatus,
		Author:        in.Author,
		IsDebugMode:   in.IsDebugMode,
		Parameters:    in.Parameters,
		Steps:         steps,
		WorkNumber:    in.WorkNumber,
	}

	return out
}

// GetTask
// @Summary Get Task
// @Description Получить экземпляр задачи
// @Tags tasks
// @ID      get-task-entity
// @Produce json
// @Param workNumber path string true "work number"
// @success 200 {object} httpResponse{data=eriusTaskResponse}
// @Failure 400 {object} httpError
// @Failure 401 {object} httpError
// @Failure 500 {object} httpError
// @Router /tasks/{taskID} [get]
func (ae *APIEnv) GetTask(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "get_task")
	defer s.End()

	log := logger.GetLogger(ctx)

	workNumber := chi.URLParam(req, "workNumber")
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

func compileGetTasksFilters(req *http.Request) (filters entity.TaskFilter, err error) {
	ui, err := user.GetEffectiveUserInfoFromCtx(req.Context())
	if err != nil {
		return filters, err
	}
	filters.CurrentUser = ui.Username

	taskIDs := req.URL.Query().Get("taskIDs")
	if taskIDs != "" {
		ids := strings.Split(taskIDs, ",")
		filters.TaskIDs = &ids
	}

	name := req.URL.Query().Get("name")
	if name != "" {
		filters.Name = &name
	}

	type created struct {
		Start int `json:"start"`
		End   int `json:"end"`
	}

	createdTime := req.URL.Query().Get("created")
	if createdTime != "" {
		var cr created
		if unmErr := json.Unmarshal([]byte(createdTime), &cr); unmErr != nil {
			return filters, unmErr
		}

		filters.Created = &entity.TimePeriod{
			Start: cr.Start,
			End:   cr.End,
		}
	}

	order := req.URL.Query().Get("order")
	if order != "" {
		filters.Order = &order
	}

	archived := req.URL.Query().Get("archived")
	if archived != "" {
		a, convErr := strconv.ParseBool(archived)
		if convErr != nil {
			return filters, convErr
		}
		filters.Archived = &a
	} else {
		a := false
		filters.Archived = &a
	}

	selectAs := req.URL.Query().Get("selectAs")
	if selectAs != "" {
		filters.SelectAs = &selectAs
	}

	lim := 10
	limit := req.URL.Query().Get("limit")
	if limit != "" {
		lim, err = strconv.Atoi(limit)
		if err != nil {
			return
		}
	}
	filters.Limit = &lim

	off := 0
	offset := req.URL.Query().Get("offset")
	if offset != "" {
		off, err = strconv.Atoi(offset)
		if err != nil {
			return
		}
	}
	filters.Offset = &off

	return
}

// GetTasks
// @Summary Get Tasks
// @Description Получить задачи
// @Tags pipeline, tasks
// @ID      get-tasks
// @Produce json
// @Param name query string false "Pipeline name"
// @Param taskIDs query []string false "Task IDs"
// @Param order query string false "Order"
// @Param limit query string false "Limit"
// @Param offset query string false "Offset"
// @success 200 {object} httpResponse{data=entity.EriusTasksPage}
// @Failure 400 {object} httpError
// @Failure 401 {object} httpError
// @Failure 500 {object} httpError
// @Router /tasks [get]
//nolint:dupl //diff logic
func (ae *APIEnv) GetTasks(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "get_tasks")
	defer s.End()

	log := logger.GetLogger(ctx)

	filters, err := compileGetTasksFilters(req)
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

	if err := sendResponse(w, http.StatusOK, resp); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

// GetPipelineTasks
// @Summary Get Pipeline Tasks
// @Description Получить задачи по сценарию
// @Tags pipeline, tasks
// @ID      get-pipeline-tasks
// @Produce json
// @Param pipelineID path string true "Pipeline ID"
// @success 200 {object} httpResponse{data=entity.EriusTasks}
// @Failure 400 {object} httpError
// @Failure 401 {object} httpError
// @Failure 500 {object} httpError
// @Router /tasks/pipeline/{pipelineID} [get]
//nolint:dupl //diff logic
func (ae *APIEnv) GetPipelineTasks(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "get_pipeline_tasks")
	defer s.End()

	log := logger.GetLogger(ctx)

	idParam := chi.URLParam(req, "pipelineID")

	id, err := uuid.Parse(idParam)
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

// GetVersionTasks
// @Summary Get Version Tasks
// @Description Получить задачи по версии сценарию
// @Tags version, tasks
// @ID      get-version-tasks
// @Produce json
// @Param versionID path string true "Version ID"
// @success 200 {object} httpResponse{data=entity.EriusTasks}
// @Failure 400 {object} httpError
// @Failure 401 {object} httpError
// @Failure 500 {object} httpError
// @Router /tasks/version/{versionID} [get]
//nolint:dupl //diff logic
func (ae *APIEnv) GetVersionTasks(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "get_version_logs")
	defer s.End()

	log := logger.GetLogger(ctx)

	idParam := chi.URLParam(req, "versionID")

	id, err := uuid.Parse(idParam)
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

	if err := sendResponse(w, http.StatusOK, resp); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

// UpdateTask
// @Summary Update Task
// @Description Update task
// @Tags tasks
// @ID update-task-entity
// @Accept json
// @Produce json
// @Param workNumber path string true "work number"
// @Param data body entity.TaskUpdate true "Task update data"
// @success 200 {object} httpResponse
// @Failure 400 {object} httpError
// @Failure 401 {object} httpError
// @Failure 500 {object} httpError
// @Router /tasks/{taskID} [post]
//nolint:gocyclo //its ok here
func (ae *APIEnv) UpdateTask(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "update_task")
	defer s.End()

	log := logger.GetLogger(ctx)

	workNumber := chi.URLParam(req, "workNumber")
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
	err = json.Unmarshal(b, &updateData)
	if err != nil {
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

	err = updateData.Validate()
	if err != nil {
		e := UpdateTaskValidationError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	blockType := getTaskStepNameByAction(updateData.Action)
	if blockType == "" {
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

	// can update only running tasks
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

	steps, err := ae.DB.GetUnfinishedTaskStepsByWorkIdAndStepType(ctx, dbTask.ID, blockType)
	if err != nil {
		e := GetTaskError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if len(steps) == 0 {
		e := GetTaskError
		log.Error(e.errorMessage(nil))
		_ = e.sendError(w)

		return
	}

	ep := pipeline.ExecutablePipeline{
		Storage:    ae.DB,
		Remedy:     ae.Remedy,
		FaaS:       ae.FaaS,
		HTTPClient: ae.HTTPClient,
		PipelineID: scenario.ID,
		VersionID:  scenario.VersionID,
		EntryPoint: scenario.Pipeline.Entrypoint,
		Sender:     ae.Mail,
		People:     ae.People,
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
		})
		if blockErr == nil {
			couldUpdateOne = true
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

func getTaskStepNameByAction(action entity.TaskUpdateAction) string {
	if action == entity.TaskUpdateActionApprovement {
		return pipeline.BlockGoApproverID
	}

	if action == entity.TaskUpdateActionExecution {
		return pipeline.BlockGoExecutionID
	}

	if action == entity.TaskUpdateActionChangeExecutor {
		return pipeline.BlockGoExecutionID
	}

	return ""
}
