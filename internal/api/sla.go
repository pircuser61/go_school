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
)

func (ae *APIEnv) handleBreachSlA(ctx c.Context, item db.StepBreachedSLA) {
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

	notifName := item.WorkTitle
	if item.IsTest {
		notifName = notifName + " (ТЕСТОВАЯ ЗАЯВКА)"
	}
	// goroutines?
	runCtx := &pipeline.BlockRunContext{
		TaskID:     item.TaskID,
		WorkNumber: item.WorkNumber,
		WorkTitle:  item.WorkTitle,
		Initiator:  item.Initiator,
		Storage:    txStorage,
		VarStore:   item.VarStore,

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

		UpdateData: &script.BlockUpdateData{
			Action: string(item.Action),
		},
		IsTest:    item.IsTest,
		NotifName: notifName,
	}

	blockErr := pipeline.ProcessBlockWithEndMapping(ctx, item.StepName, item.BlockData, runCtx, true)
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
}

//nolint:gocyclo,staticcheck //its ok here
func (ae *APIEnv) CheckBreachSLA(w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.StartSpan(r.Context(), "check_breach_sla")
	defer span.End()

	log := logger.GetLogger(ctx).WithField("mainFuncName", "CheckBreachSLA")

	steps, err := ae.DB.GetBlocksBreachedSLA(ctx)
	if err != nil {
		e := UpdateBlockError
		log.Error(e.errorMessage(errors.New("couldn't get steps")))
		_ = e.sendError(w)

		return
	}

	spCtx := span.SpanContext()
	routineCtx := c.WithValue(c.Background(), XRequestIDHeader, ctx.Value(XRequestIDHeader))
	routineCtx = logger.WithLogger(routineCtx, log)
	processCtx, fakeSpan := trace.StartSpanWithRemoteParent(routineCtx, "start_check_breach_sla", spCtx)
	fakeSpan.End()

	for _, item := range steps {
		log = log.WithFields(map[string]interface{}{
			"taskID":   item.TaskID,
			"stepName": item.StepName,
		})
		ae.handleBreachSlA(logger.WithLogger(processCtx, log), item)
	}
}
