package handlers

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/erius/admin/pkg/auth"
	"gitlab.services.mts.ru/erius/admin/pkg/vars"

	"gitlab.services.mts.ru/erius/pipeliner/internal/entity"
	"gitlab.services.mts.ru/erius/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/erius/pipeliner/internal/store"
)

const (
	actionStepOver = "step_over"
	actionResume   = "resume"
)

var errCantGetNextBlock = errors.New("can't get next block")

type DebugRunRequest struct {
	TaskID      uuid.UUID `json:"task_id"`
	BreakPoints []string  `json:"break_points"`
	Action      string    `json:"action" example:"step_over,resume"`
}

func (d DebugRunRequest) Bind(r *http.Request) error {
	return nil
}

type CreateTaskRequest struct {
	VersionID  uuid.UUID              `json:"version_id"`
	Parameters map[string]interface{} `json:"parameters"`
}

// @Summary Start debug task
// @Description Начать отладку
// @Tags debug
// @ID debug-task-run
// @Accept json
// @Produce json
// @Param variables body DebugRunRequest false "debug request"
// @Success 200 {object} httpResponse{data=entity.EriusTask}
// @Failure 400 {object} httpError
// @Failure 401 {object} httpError
// @Failure 500 {object} httpError
// @Router /debug/run [post]
func (ae *APIEnv) StartDebugTask(w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.StartSpan(r.Context(), "start debug task")
	defer span.End()

	log := logger.GetLogger(ctx)

	debugRequest := &DebugRunRequest{}
	if err := render.Bind(r, debugRequest); err != nil {
		e := RequestReadError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	task, err := ae.DB.GetTask(ctx, debugRequest.TaskID)
	if err != nil {
		e := GetTaskError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if !task.IsStopped() && !task.IsCreated() {
		if task.IsRun() {
			e := RunDebugTaskAlreadyRunError
			log.Error(e.errorMessage(err))
			_ = e.sendError(w)

			return
		}

		if task.IsError() {
			e := RunDebugTaskAlreadyError
			log.Error(e.errorMessage(err))
			_ = e.sendError(w)

			return
		}

		if task.IsFinished() {
			e := RunDebugTaskFinishedError
			log.Error(e.errorMessage(err))
			_ = e.sendError(w)

			return
		}

		e := RunDebugInvalidStatusError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	go func() {
		routineCtx := context.Background()

		_, err := ae.runDebugTask(routineCtx, task, debugRequest.BreakPoints, debugRequest.Action)
		if err != nil {
			e := RunDebugError
			log.Error(e.errorMessage(err))

			return
		}
	}()

	if err := sendResponse(w, http.StatusOK, task); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

// @Summary Create debug task
// @Description Создать сессию отладки
// @Tags debug
// @ID create-debug-task
// @Accept json
// @Produce json
// @Param debug body CreateTaskRequest true "New debug task"
// @Success 200 {object} httpResponse{data=entity.EriusTask}
// @Failure 400 {object} httpError
// @Failure 401 {object} httpError
// @Failure 500 {object} httpError
// @Router /debug/ [post]
func (ae *APIEnv) CreateDebugTask(w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.StartSpan(r.Context(), "create debug task")
	defer span.End()

	log := logger.GetLogger(ctx)

	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		e := RequestReadError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
	defer r.Body.Close()

	d := CreateTaskRequest{}

	err = json.Unmarshal(b, &d)
	if err != nil {
		e := CreateDebugParseError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	version, err := ae.DB.GetPipelineVersion(ctx, d.VersionID)
	if err != nil {
		e := GetVersionError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	// права на создание дебаг сессии проверяем относительно запуска сценария
	grants, err := ae.AuthClient.CheckGrants(ctx, vars.Pipeline, vars.Run)
	if err != nil {
		e := AuthServiceError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if !grants.Allow && grants.Contains(version.ID.String()) {
		e := UnauthError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	user, err := auth.UserFromContext(ctx)
	if err != nil {
		log.Error("user failed: ", err.Error())
	}

	parameters, err := json.Marshal(d.Parameters)
	if err != nil {
		e := CreateDebugInputsError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	task, err := ae.DB.CreateTask(ctx, uuid.New(), version.VersionID, user.UserName(), true, parameters)
	if err != nil {
		e := CreateWorkError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = sendResponse(w, http.StatusOK, task)
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

func (ae *APIEnv) executablePipeline(
	ctx context.Context,
	task *entity.EriusTask,
	version *entity.EriusScenario,
) (*pipeline.ExecutablePipeline, error) {
	ep := pipeline.ExecutablePipeline{
		TaskID:        task.ID,
		PipelineID:    version.ID,
		VersionID:     version.VersionID,
		Storage:       ae.DB,
		EntryPoint:    version.Pipeline.Entrypoint,
		FaaS:          ae.FaaS,
		PipelineModel: version,
		HTTPClient:    ae.HTTPClient,
		Remedy:        ae.Remedy,
	}

	err := ep.CreateBlocks(ctx, version.Pipeline.Blocks)
	if err != nil {
		return nil, errors.Wrap(err, "unable to create pipeline blocks")
	}

	return &ep, nil
}

func variableStoreFromSteps(
	task *entity.EriusTask,
	version *entity.EriusScenario,
	steps entity.TaskSteps,
) *store.VariableStore {
	vs := store.NewStore()
	isFirstRun := len(steps) == 0

	if isFirstRun {
		for key, value := range task.Parameters {
			vs.SetValue(version.Name+pipeline.KeyDelimiter+key, value)
		}
	}

	if !isFirstRun {
		lastStep := steps[0]
		vs = store.NewFromStep(lastStep)
	}

	return vs
}

func currentStepName(
	ep *pipeline.ExecutablePipeline,
	steps entity.TaskSteps,
	task *entity.EriusTask,
	vs *store.VariableStore,
) (string, error) {
	if steps.IsEmpty() {
		return ep.EntryPoint, nil
	}

	if task.IsRun() {
		currentStep, ok := ep.Blocks[steps[0].Name].Next(vs)
		if !ok {
			return "", pipeline.ErrCantGetNextStep
		}

		return currentStep, nil
	}

	return steps[0].Name, nil
}

func currentBlockStatus(
	task *entity.EriusTask,
	steps entity.TaskSteps,
) (blockStatus string) {
	if !steps.IsEmpty() {
		blockStatus = stepStatus(task, steps[0])

		return
	}

	return
}

func stepStatus(task *entity.EriusTask, step *entity.Step) (stepStatus string) {
	if _, ok := step.Storage[step.Name+pipeline.KeyDelimiter+pipeline.ErrorKey]; ok || step.HasError {
		return "error"
	}

	return task.Status
}

// todo monitoring
func (ae *APIEnv) runDebugTask(
	ctx context.Context,
	task *entity.EriusTask,
	breakPoints []string,
	action string,
) (*entity.DebugResult, error) {
	ctx, s := trace.StartSpan(ctx, "run debug task")
	defer s.End()

	log := logger.GetLogger(ctx)

	_ = action

	version, err := ae.DB.GetPipelineVersion(ctx, task.VersionID)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get version")
	}

	ep, err := ae.executablePipeline(ctx, task, version)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get executable pipeline")
	}

	steps, err := ae.DB.GetTaskSteps(ctx, task.ID)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get task steps")
	}

	vs := variableStoreFromSteps(task, version, steps)

	if steps.IsEmpty() {
		ep.NowOnPoint = ep.EntryPoint
	} else {
		ep.NowOnPoint, _ = ep.Blocks[steps[0].Name].Next(vs)
	}

	stopPoints := store.NewStopPoints(ep.NowOnPoint)
	nextBlock := ep.Blocks[ep.NowOnPoint]

	if nextBlock == nil {
		log.Error(errCantGetNextBlock)

		return nil, errors.Wrap(errCantGetNextBlock, "can't get next block")
	}

	nextSteps := nextBlock.NextSteps()

	vs.SetStopPoints(*stopPoints)
	vs.StopPoints.SetBreakPoints(breakPoints...)

	if action == actionStepOver {
		vs.StopPoints.SetStepOvers(nextSteps...)
	}

	// игнорируем точки останова на блоках, следующих за тем с которого выполняется resume
	// это не касается случая когда task был только создан и точка останова стоит на блоках следующих за стартовым
	if action == actionResume && !task.IsCreated() {
		vs.StopPoints.SetExcludedPoints(nextSteps...)
	}

	err = ep.DebugRun(ctx, vs)
	if err != nil {
		log.Error(err)

		return nil, errors.Wrap(err, "unable to run debug")
	}

	return &entity.DebugResult{}, nil
}

// DebugTask
// @Summary Debug task
// @Description Получить debug-задачу
// @Tags tasks
// @ID      debug-task
// @Produce json
// @Param taskID path string true "Task ID"
// @success 200 {object} httpResponse{data=entity.DebugResult}
// @Failure 400 {object} httpError
// @Failure 401 {object} httpError
// @Failure 500 {object} httpError
// @Router /debug/{taskID} [get]
// nolint:dupl //its unique
func (ae *APIEnv) DebugTask(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "get_debug_task")
	defer s.End()

	log := logger.GetLogger(ctx)

	idParam := chi.URLParam(req, "taskID")

	taskID, err := uuid.Parse(idParam)
	if err != nil {
		e := UUIDParsingError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	task, err := ae.DB.GetTask(ctx, taskID)
	if err != nil {
		e := GetTaskError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	version, err := ae.DB.GetPipelineVersion(ctx, task.VersionID)
	if err != nil {
		e := GetVersionError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	ep, err := ae.executablePipeline(ctx, task, version)
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	steps, err := ae.DB.GetTaskSteps(ctx, task.ID)
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	task.Steps = steps

	vs := variableStoreFromSteps(task, version, steps)

	nowOnPoint, err := currentStepName(ep, steps, task, vs)
	if err != nil {
		e := RunDebugError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	nowOnPointStatus := currentBlockStatus(task, steps)
	stopPoints := vs.StopPoints.BreakPointsList()

	result := entity.DebugResult{
		BlockName:   nowOnPoint,
		BlockStatus: nowOnPointStatus,
		BreakPoints: stopPoints,
		Task:        task,
	}

	if err := sendResponse(w, http.StatusOK, result); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

// LastVersionDebugTask
// @Summary Get last debug task for version
// @Description Получить последнюю debug-задачу версии сценария
// @Tags tasks
// @ID      get-version-last-debug-task
// @Produce json
// @Param versionID path string true "Version ID"
// @success 200 {object} httpResponse{data=entity.EriusTask}
// @Failure 400 {object} httpError
// @Failure 401 {object} httpError
// @Failure 500 {object} httpError
// @Router /tasks/last-by-version/{versionID} [get]
// nolint:dupl //its unique
func (ae *APIEnv) LastVersionDebugTask(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "get_last_version_tasks")
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

	user, err := auth.UserFromContext(ctx)
	if err != nil {
		if err != nil {
			e := NoUserInContextError
			log.Error(e.errorMessage(err))
			_ = e.sendError(w)

			return
		}
	}

	task, err := ae.DB.GetLastDebugTask(ctx, id, user.UserName())
	if err != nil {
		e := GetTaskError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	steps, err := ae.DB.GetTaskSteps(ctx, task.ID)
	if err != nil {
		e := GetTaskError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	task.Steps = steps

	if err := sendResponse(w, http.StatusOK, task); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}
