package api

import (
	c "context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

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
			HTTPClient:    ae.HTTPClient,
			Storage:       ae.DB,
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
			JocastaURL:    ae.HostURL,
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

//nolint:gocognit,gocyclo //большая сложность
func (ae *Env) CheckFired(w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.StartSpan(r.Context(), "check_fired")
	defer span.End()

	log := script.SetMainFuncLog(ctx,
		"CheckFired",
		script.MethodGet,
		script.HTTP,
		span.SpanContext().TraceID.String(),
		"v1",
	)

	errorhandler := newHTTPErrorHandler(log, w)

	steps, err := ae.DB.GetRunningExecutionBlocks(ctx)
	if err != nil {
		err = errors.New("couldn't get steps")
		log.WithError(err)
		errorhandler.handleError(UpdateBlockError, err)

		return
	}

	type fTask struct {
		ID                  string
		WorkID              string
		StepName            string
		Content             map[string]json.RawMessage
		CurrentExecutorData map[string]json.RawMessage
	}

	firedTasks := make(map[string][]fTask)

	var val json.RawMessage

	for i := range steps {
		val = steps[i].CurrentExecutorData["people"]
		people := []string{}

		err = json.Unmarshal([]byte(val), &people)
		if err != nil {
			log.WithError(err).Error("failed to unmarshal people")

			return
		}

		if len(people) != 1 {
			continue
		}

		val = steps[i].Content["State"]
		state := make(map[string]interface{})

		err = json.Unmarshal([]byte(val), &state)
		if err != nil {
			log.WithError(err).Error("failed to unmarshal people")

			return
		}

		for k, v := range state {
			if strings.Contains(k, "execution") {
				vv := v.(map[string]interface{})

				isTaken := vv["is_taken_in_work"]

				if isTaken != nil && isTaken.(bool) {
					firedTasks[people[0]] = append(firedTasks[people[0]], fTask{
						ID:                  steps[i].ID.String(),
						WorkID:              steps[i].WorkID.String(),
						StepName:            steps[i].Name,
						Content:             steps[i].Content,
						CurrentExecutorData: steps[i].CurrentExecutorData,
					})
				}
			}
		}
	}

	logins := []string{}
	for login := range firedTasks {
		logins = append(logins, login)
	}

	if len(logins) == 0 {
		return
	}

	result, err := ae.HrGate.GetComplexAssignmentsV2(ctx, logins)
	if err != nil {
		log.WithError(err).Error("failed to unmarshal people")

		return
	}

	firedLog := make(map[string]struct{})

	dateNow := time.Now()
	formDate := dateNow.Format("2000-12-31")

	for i := range result {
		if result[i].ActualTerminationDate != "" && result[i].ActualTerminationDate < formDate {
			smallLog := strings.ToLower(result[i].Employee.Login)
			firedLog[smallLog] = struct{}{}
		}
	}

	for login := range firedTasks {
		if _, ok := firedLog[login]; !ok {
			delete(firedTasks, login)
		}
	}

	for _, v := range firedTasks {
		for _, task := range v {
			jsonData := task.Content["State"]

			var data map[string]interface{}

			err = json.Unmarshal([]byte(jsonData), &data)
			if err != nil {
				log.WithError(err).Error("failed to unmarshal tasks content")

				continue
			}

			//nolint:nestif //большая вложенность структуры хранения данных
			for key, value := range data {
				if strings.Contains(key, "execution") {
					if execution, ok := value.(map[string]interface{}); ok {
						if _, exists := execution["is_taken_in_work"]; exists {
							execution["is_taken_in_work"] = false

							if initialExecutors, ok := execution["initial_executors"]; ok {
								execution["executors"] = initialExecutors
							}
						}
					}
				}
			}

			updatedJSONData, err := json.Marshal(data)
			if err != nil {
				log.WithError(err).Error("failed to update tasks content")

				return
			}

			task.Content["State"] = updatedJSONData

			if initialPeople, ok := task.CurrentExecutorData["initial_people"]; ok {
				task.CurrentExecutorData["people"] = initialPeople
			}

			state := make(map[string]interface{})
			content := task.Content["State"]

			if err = json.Unmarshal(content, &state); err != nil {
				log.WithError(err).Error("failed to unmarshal tasks content")

				continue
			}

			err = ae.DB.UpdateStepContent(ctx, task.ID, task.WorkID, task.StepName, state, map[string]interface{}{})
			if err != nil {
				log.WithError(err).Error("failed to update step content")

				continue
			}

			err = ae.DB.SetStartMembers(ctx, task.ID)
			if err != nil {
				log.WithError(err).Error("failed to update step content")

				continue
			}
		}
	}
}
