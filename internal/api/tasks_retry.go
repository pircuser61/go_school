package api

import (
	"context"
	"errors"
	"math"
	"net/http"

	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/metrics"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
	"go.opencensus.io/trace"
)

func (ae *Env) RetryTasks(w http.ResponseWriter, r *http.Request, params RetryTasksParams) {
	ctx, span := trace.StartSpan(r.Context(), "retry_tasks")
	defer span.End()

	log := script.SetMainFuncLog(ctx,
		"RetryTasks",
		script.MethodGet,
		script.HTTP,
		span.SpanContext().TraceID.String(),
		"v1",
	)

	errorHandler := newHTTPErrorHandler(log, w)

	ctx = logger.WithLogger(ctx, errorHandler.log)

	limit := math.MaxInt
	if params.Limit != nil && *params.Limit > 0 {
		limit = *params.Limit
	}

	err := ae.retryEmptyTasks(ctx, limit)
	if err != nil {
		httpErr := getErr(err)

		errorHandler.handleError(httpErr, err)

		return
	}

	err = sendResponse(w, http.StatusOK, nil)
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

func (ae *Env) retryEmptyTasks(ctx context.Context, limit int) error {
	ctx, span := trace.StartSpan(ctx, "retry_empty_tasks")
	defer span.End()

	log := logger.GetLogger(ctx).WithField(script.FuncName, "retryEmptyTasks")

	emptyTasks, filledTasks, err := ae.DB.GetTasksToRetry(ctx, ae.TaskRetry.MinLifetime, ae.TaskRetry.MaxLifetime, limit)
	if err != nil {
		log.Error(err)

		return errors.Join(GetTaskError, err)
	}

	ctx = logger.WithLogger(ctx, log)

	ae.launchEmptyTasks(ctx, emptyTasks)
	ae.launchTasks(ctx, filledTasks)

	return nil
}

func (ae *Env) launchEmptyTasks(ctx context.Context, emptyTasks []*db.Task) {
	log := logger.GetLogger(ctx).WithField(script.FuncName, "launchEmptyTasks")

	for _, emptyTask := range emptyTasks {
		processErr := ae.launchEmptyTask(ctx, ae.DB, emptyTask, "", &metrics.RequestInfo{})
		if processErr != nil {
			log.WithError(processErr).
				WithFields(
					logger.Fields{
						"workId":     emptyTask.WorkID,
						"workNumber": emptyTask.WorkNumber,
					},
				).
				Error("process empty task error")
		}
	}
}

func (ae *Env) launchTasks(ctx context.Context, tasks []*db.Task) {
	log := logger.GetLogger(ctx).WithField(script.FuncName, "launchTasks")

	for _, emptyTask := range tasks {
		processErr := ae.launchTask(ctx, emptyTask)
		if processErr != nil {
			log.WithError(processErr).
				WithFields(
					logger.Fields{
						"workId":     emptyTask.WorkID,
						"workNumber": emptyTask.WorkNumber,
					},
				).
				Error("process empty task error")
		}
	}
}

var ErrStepNotInPipelineBlocks = errors.New("step not in pipeline block")

func (ae *Env) launchTask(ctx context.Context, task *db.Task) error {
	err := ae.processTask(ctx, task)

	return handleLaunchTaskError(ctx, ae.DB, task.WorkID, err)
}

func (ae *Env) processTask(ctx context.Context, task *db.Task) error {
	log := logger.GetLogger(ctx).WithField(script.FuncName, "processTask")

	step, err := ae.DB.GetTaskStepToRetry(ctx, task.WorkID)
	if err != nil {
		return errors.Join(GetTaskStepError, err)
	}

	version, err := ae.DB.GetVersionByPipelineID(ctx, task.RunContext.PipelineID)
	if err != nil {
		return errors.Join(GetVersionsByBlueprintIDError, err)
	}

	runCtx := &pipeline.BlockRunContext{
		TaskID:     task.WorkID,
		WorkNumber: task.WorkNumber,
		ClientID:   task.RunContext.ClientID,
		PipelineID: version.PipelineID,
		VersionID:  version.VersionID,
		WorkTitle:  version.Name,
		Initiator:  task.Author,
		VarStore:   store.NewFromStep(step),

		Services: pipeline.RunContextServices{
			HTTPClient:    ae.HTTPClient,
			Sender:        ae.Mail,
			Kafka:         ae.Kafka,
			People:        ae.People,
			ServiceDesc:   ae.ServiceDesc,
			FunctionStore: ae.FunctionStore,
			HumanTasks:    ae.HumanTasks,
			Integrations:  ae.Integrations,
			FileRegistry:  ae.FileRegistry,
			FaaS:          ae.FaaS,
			HrGate:        ae.HrGate,
			Scheduler:     ae.Scheduler,
			SLAService:    ae.SLAService,
			Storage:       ae.DB,
			JocastaURL:    ae.HostURL,
		},
		BlockRunResults: &pipeline.BlockRunResults{},

		UpdateData: nil,
		IsTest:     task.RunContext.InitialApplication.IsTestApplication,
		NotifName: utils.MakeTaskTitle(
			version.Name,
			task.RunContext.InitialApplication.CustomTitle,
			task.RunContext.InitialApplication.IsTestApplication,
		),
		Productive: true,
	}

	blockData, ok := version.Pipeline.Blocks[step.Name]
	if !ok {
		return ErrStepNotInPipelineBlocks
	}

	_, workFinished, err := pipeline.ProcessBlockWithEndMapping(ctx, step.Name, blockData, runCtx, false)
	if err != nil {
		runCtx.NotifyEvents(ctx) // events for successfully processed nodes

		return err
	}

	if workFinished {
		err = ae.Scheduler.DeleteAllTasksByWorkID(ctx, task.WorkID)
		if err != nil {
			log.WithField("funcName", "DeleteAllTasksByWorkID").
				WithError(err).Error("failed delete all tasks by work id in scheduler")
		}
	}

	runCtx.NotifyEvents(ctx)

	return nil
}
