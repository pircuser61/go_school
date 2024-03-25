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
	log := logger.GetLogger(ctx)

	txStorage, transactionErr := ae.DB.StartTransaction(ctx)
	if transactionErr != nil {
		log.WithError(transactionErr).Error("couldn't set SLA breach")

		return
	}

	defer func() {
		if r := recover(); r != nil {
			log = log.WithField("funcName", "handleBreachSlA").
				WithField("panic handle", true)
			log.Error(r)

			if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
				log.WithError(errors.New("couldn't rollback tx")).
					Error(txErr)
			}
		}
	}()

	runCtx := &pipeline.BlockRunContext{
		TaskID:     item.TaskID,
		WorkNumber: item.WorkNumber,
		WorkTitle:  item.WorkTitle,
		Initiator:  item.Initiator,
		VarStore:   item.VarStore,

		Services: pipeline.RunContextServices{
			HTTPClient:    ae.HTTPClient,
			Storage:       txStorage,
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
		},
		BlockRunResults: &pipeline.BlockRunResults{},

		UpdateData: &script.BlockUpdateData{
			Action: string(item.Action),
		},
		IsTest:      item.IsTest,
		CustomTitle: item.CustomTitle,
		NotifName:   utils.MakeTaskTitle(item.WorkTitle, item.CustomTitle, item.IsTest),
		Productive:  true,
	}

	runCtx.SetTaskEvents(ctx)

	workFinished, blockErr := pipeline.ProcessBlockWithEndMapping(ctx, item.StepName, item.BlockData, runCtx, true)
	if blockErr != nil {
		log.WithError(blockErr).Error("couldn't set SLA breach")

		if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
			log.WithField("funcName", "handleBreachSlA").
				WithError(errors.New("couldn't rollback tx")).
				Error(txErr)
		}

		return
	}

	if commitErr := txStorage.CommitTransaction(ctx); commitErr != nil {
		log.WithError(commitErr).Error("couldn't set SLA breach")

		if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
			log.Error(txErr)
		}
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

	log := logger.GetLogger(ctx).WithField("mainFuncName", "CheckBreachSLA")
	errorhandler := newHTTPErrorHandler(log, w)

	steps, err := ae.DB.GetBlocksBreachedSLA(ctx)
	if err != nil {
		err := errors.New("couldn't get steps")
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
		log = log.WithFields(map[string]interface{}{
			"taskID":   item.TaskID,
			"stepName": item.StepName,
		})

		ae.handleBreachSlA(logger.WithLogger(processCtx, log), &item)
	}
}
