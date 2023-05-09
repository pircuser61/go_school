package api

import (
	c "context"
	"net/http"

	"github.com/pkg/errors"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

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
		txStorage, transactionErr := ae.DB.StartTransaction(processCtx)
		if transactionErr != nil {
			log.WithError(transactionErr).Error("couldn't set SLA breach")
			continue
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
			FaaS:          ae.FaaS,

			UpdateData: &script.BlockUpdateData{
				Action: string(item.Action),
			},
		}

		blockErr := pipeline.ProcessBlockWithEndMapping(processCtx, item.StepName, item.BlockData, runCtx, true)
		if blockErr != nil {
			log.WithError(blockErr).Error("couldn't set SLA breach")
			if txErr := txStorage.RollbackTransaction(processCtx); txErr != nil {
				log.WithField("funcName", "ProcessBlock").
					WithError(errors.New("couldn't rollback tx")).
					Error(txErr)
			}
			continue
		}
		if commitErr := txStorage.CommitTransaction(processCtx); commitErr != nil {
			log.WithError(commitErr).Error("couldn't set SLA breach")
			if txErr := txStorage.RollbackTransaction(routineCtx); txErr != nil {
				log.Error(txErr)
			}
		}
	}
}
