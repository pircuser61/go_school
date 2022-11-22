package api

import (
	"context"
	"net/http"

	"github.com/go-chi/render"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

func (d DebugRunRequest) Bind(_ *http.Request) error {
	return nil
}

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

	task, err := ae.DB.GetTask(ctx, nil, debugRequest.WorkNumber)
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
		Sender:        ae.Mail,
		People:        ae.People,
		ServiceDesc:   ae.ServiceDesc,
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

		if len(currentStep) > 1 {
			return currentStep[0], nil // todo: must переделать
		}
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
	_ context.Context,
	_ *entity.EriusTask,
	_ []string,
	_ string,
) (*entity.DebugResult, error) {
	return &entity.DebugResult{}, nil
}

func (ae *APIEnv) DebugTask(w http.ResponseWriter, req *http.Request, workNumber string) {
	ctx, s := trace.StartSpan(req.Context(), "get_debug_task")
	defer s.End()

	log := logger.GetLogger(ctx)

	if workNumber == "" {
		e := UUIDParsingError
		log.Error(e.errorMessage(errors.New("workNumber is empty")))
		_ = e.sendError(w)

		return
	}

	task, err := ae.DB.GetTask(ctx, nil, workNumber)
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

func (ae *APIEnv) LastVersionDebugTask(w http.ResponseWriter, req *http.Request, versionID string) {
	ctx, s := trace.StartSpan(req.Context(), "get_last_version_tasks")
	defer s.End()

	log := logger.GetLogger(ctx)

	id, err := uuid.Parse(versionID)
	if err != nil {
		e := UUIDParsingError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	task, err := ae.DB.GetLastDebugTask(ctx, id, "")
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
