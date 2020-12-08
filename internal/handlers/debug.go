package handlers

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"gitlab.services.mts.ru/erius/pipeliner/internal/store"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/erius/pipeliner/internal/pipeline"

	"github.com/google/uuid"
	"gitlab.services.mts.ru/erius/admin/pkg/auth"
	"gitlab.services.mts.ru/erius/admin/pkg/vars"
	"gitlab.services.mts.ru/erius/pipeliner/internal/entity"

	"github.com/go-chi/render"

	"go.opencensus.io/trace"
)

type DebugRunRequest struct {
	TaskID      uuid.UUID `json:"task_id"`
	BreakPoints []string  `json:"break_points"`
	Action      string    `json:"action" example:"step_over,resume"`
}

func (d DebugRunRequest) Bind(r *http.Request) error {
	return nil
}

type CreateTaskRequest struct {
	VersionID  uuid.UUID         `json:"version_id"`
	Parameters map[string]string `json:"parameters"`
}

// @Summary Start debug task
// @Description Начать отладку
// @Tags debug
// @ID debug-task
// @Accept json
// @Produce json
// @Param variables body DebugRunRequest false "debug request"
// @Success 200 {object} httpResponse{data=entity.DebugRunResult}
// @Failure 400 {object} httpError
// @Failure 401 {object} httpError
// @Failure 500 {object} httpError
// @Router /debug/run [post]
func (ae *APIEnv) StartDebugTask(w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.StartSpan(r.Context(), "start debug task")
	defer span.End()

	debugRequest := &DebugRunRequest{}
	if err := render.Bind(r, debugRequest); err != nil {
		e := RequestReadError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	task, err := ae.DB.GetTask(ctx, debugRequest.TaskID)
	if err != nil {
		e := GetTaskError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	mappedBreakPoints := sliceToMap(debugRequest.BreakPoints)

	result, err := ae.runDebugTask(ctx, task, mappedBreakPoints, debugRequest.Action)
	if err != nil {
		e := PipelineRunError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if err := sendResponse(w, http.StatusOK, result); err != nil {
		e := UnknownError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

func sliceToMap(items []string) map[string]struct{} {
	res := make(map[string]struct{})

	for i := range items {
		res[items[i]] = struct{}{}
	}

	return res
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

	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		e := RequestReadError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
	defer r.Body.Close()

	d := CreateTaskRequest{}

	err = json.Unmarshal(b, &d)
	if err != nil {
		e := CreateDebugParseError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	version, err := ae.DB.GetPipelineVersion(ctx, d.VersionID)
	if err != nil {
		e := GetVersionError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	// права на создание дебаг сессии проверяем относительно запуска сценария
	grants, err := ae.AuthClient.CheckGrants(ctx, vars.Pipeline, vars.Run)
	if err != nil {
		e := AuthServiceError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if !grants.Allow && grants.Contains(version.ID.String()) {
		e := UnauthError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	user, err := auth.UserFromContext(ctx)
	if err != nil {
		ae.Logger.Error("user failed: ", err.Error())
	}

	parameters, err := json.Marshal(d.Parameters)
	if err != nil {
		e := CreateDebugInputsError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	task, err := ae.DB.CreateTask(ctx, uuid.New(), version.VersionID, user.UserName(), true, parameters)
	if err != nil {
		e := CreateWorkError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = sendResponse(w, http.StatusOK, task)
	if err != nil {
		e := UnknownError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

// todo monitoring
func (ae *APIEnv) runDebugTask(
	ctx context.Context,
	task *entity.EriusTask,
	breakPoints map[string]struct{},
	action string,
) (*entity.DebugRunResult, error) {
	ctx, s := trace.StartSpan(ctx, "run debug task")
	defer s.End()

	version, err := ae.DB.GetPipelineVersion(ctx, task.VersionID)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get version")
	}

	ep := pipeline.ExecutablePipeline{
		PipelineID:    version.ID,
		VersionID:     version.VersionID,
		Storage:       ae.DB,
		EntryPoint:    version.Pipeline.Entrypoint,
		Logger:        ae.Logger,
		FaaS:          ae.FaaS,
		PipelineModel: version,
		HTTPClient:    ae.HTTPClient,
		Remedy:        ae.Remedy,
	}

	err = ep.CreateBlocks(ctx, version.Pipeline.Blocks)
	if err != nil {
		return nil, errors.Wrap(err, "unable to create pipeline blocks")
	}

	steps, err := ae.DB.GetTaskSteps(ctx, task.ID)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get task steps")
	}

	vs := store.NewStore()

	if len(steps) == 0 {
		vs.SetBreakPoints(breakPoints)

		for key, value := range task.Parameters {
			vs.SetValue(version.Name+"."+key, value)
		}
	} else {
		lastStep := steps[len(steps)-1]
		vs = store.NewFromStep(&lastStep, breakPoints)
	}

	err = ep.DebugRun(ctx, vs)
	if err != nil {
		ae.Logger.Error(err)
	}

	return &entity.DebugRunResult{}, nil
}
