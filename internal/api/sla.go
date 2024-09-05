package api

import (
	c "context"
	"net/http"

	"github.com/pkg/errors"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

func (ae *Env) handleBreachSlA(ctx c.Context, item *db.StepBreachedSLA) {
	log := logger.GetLogger(ctx).
		WithField(script.FuncName, "handleBreachSlA").
		WithField(script.WorkID, item.TaskID).
		WithField(script.WorkNumber, item.WorkNumber).
		WithField(script.StepName, item.StepName)
	ctx = logger.WithLogger(ctx, log)

	runCtx := &pipeline.BlockRunContext{
		TaskID:     item.TaskID,
		WorkNumber: item.WorkNumber,
		WorkTitle:  item.WorkTitle,
		Initiator:  item.Initiator,
		VarStore:   item.VarStore,
		PipelineID: item.PipelineID,
		VersionID:  item.VersionID,

		Services: pipeline.RunContextServices{
			HTTPClient:     ae.HTTPClient,
			Storage:        ae.DB,
			StorageFactory: ae.DB,
			Sender:         ae.Mail,
			Kafka:          ae.Kafka,
			People:         ae.People,
			ServiceDesc:    ae.ServiceDesc,
			FunctionStore:  ae.FunctionStore,
			HumanTasks:     ae.HumanTasks,
			Integrations:   ae.Integrations,
			FileRegistry:   ae.FileRegistry,
			FaaS:           ae.FaaS,
			HrGate:         ae.HrGate,
			Scheduler:      ae.Scheduler,
			SLAService:     ae.SLAService,
			JocastaURL:     ae.HostURL,
		},
		BlockRunResults: &pipeline.BlockRunResults{},

		UpdateData: &script.BlockUpdateData{
			Action: string(item.Action),
		},
		IsTest:      item.IsTest,
		CustomTitle: item.CustomTitle,
		NotifName:   utils.MakeTaskTitle(item.WorkTitle, item.CustomTitle, item.IsTest),
		Productive:  true,
		BreachedSLA: true,
	}

	runCtx.SetTaskEvents(ctx)

	_, workFinished, blockErr := pipeline.ProcessBlockWithEndMapping(ctx, item.StepName, item.BlockData, runCtx, true)
	if blockErr != nil {
		log.WithError(blockErr)
		runCtx.NotifyEvents(ctx) // events for successfully processed nodes

		return
	}

	if workFinished {
		err := ae.Scheduler.DeleteAllTasksByWorkID(ctx, item.TaskID)
		if err != nil {
			log.WithError(err).Error("failed delete all tasks by work id in scheduler")
		}
	}

	runCtx.NotifyEvents(ctx)
}

func (ae *Env) CheckBreachSLA(w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.StartSpan(r.Context(), "check_breach_sla")
	defer span.End()

	log := script.SetMainFuncLog(ctx,
		"CheckBreachSLA",
		script.MethodGet,
		script.HTTP,
		span.SpanContext().TraceID.String(),
		"v1",
	)

	errorhandler := newHTTPErrorHandler(log, w)

	steps, err := ae.DB.GetBlocksBreachedSLA(ctx)
	if err != nil {
		err := errors.New("couldn't get steps")
		log.WithError(err)
		errorhandler.handleError(UpdateBlockError, err)

		return
	}

	spCtx := span.SpanContext()

	// nolint // так надо и без этого нельзя
	routineCtx := c.WithValue(c.Background(), XRequestIDHeader, ctx.Value(XRequestIDHeader))

	routineCtx = logger.WithLogger(routineCtx, log)
	processCtx, fakeSpan := trace.StartSpanWithRemoteParent(routineCtx, "start_check_breach_sla", spCtx)
	fakeSpan.End()

	//nolint:gocritic //глобальная тема, лучше не трогать
	for i := range steps {
		item := steps[i]
		log = log.WithField(script.PipelineID, item.PipelineID).
			WithField(script.VersionID, item.VersionID).
			WithField(script.WorkID, item.TaskID).
			WithField(script.StepName, item.StepName)

		ae.handleBreachSlA(logger.WithLogger(processCtx, log), &item)
	}
}
