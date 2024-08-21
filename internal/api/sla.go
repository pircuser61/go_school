package api

import (
	c "context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

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
		err := errors.New("couldn't get steps")
		log.WithError(err)
		errorhandler.handleError(UpdateBlockError, err)

		return
	}

	type fTask struct {
		Id                  string
		Content             map[string]json.RawMessage
		CurrentExecutorData map[string]json.RawMessage
	}

	firedTasks := make(map[string][]fTask)
	for _, s := range steps {
		if s.ID.String() == "aea6fbb2-216f-4282-b39a-8500b1b3609b" {
			val := s.CurrentExecutorData["people"]
			people := []string{}

			err = json.Unmarshal([]byte(val), &people)
			if err != nil {
				log.WithError(err).Error("failed to unmarshal people")
				return
			}

			if len(people) != 1 {
				continue
			}

			val = s.Content["State"]
			state := make(map[string]interface{})
			err = json.Unmarshal([]byte(val), &state)

			for k, v := range state {
				if strings.Contains(k, "execution") {
					vv := v.(map[string]interface{})

					isTaken := vv["is_taken_in_work"]

					if isTaken.(bool) == true {
						firedTasks[people[0]] = append(firedTasks[people[0]], fTask{
							Id:                  s.ID.String(),
							Content:             s.Content,
							CurrentExecutorData: s.CurrentExecutorData,
						})
					}

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

	sort.Strings(logins)

	result, err := ae.HrGate.GetComplexAssignmentsV2(ctx, logins)
	if err != nil {
		log.WithError(err).Error("failed to unmarshal people")
		return
	}

	//TODO remove
	fmt.Println("\n\n\n\n\n")
	for _, v := range result {
		fmt.Println(v.Id, "\n\n")
	}
	// сопоставить логины с id и датой увольнения
	// удалить все логины из мапы что не уволены (текущая дата больше даты терминации)

	// для оставшихся пройтись по таскам в массиве в мапе[пользователь] и обновить контекст и тд
	for _, v := range firedTasks {
		for _, task := range v {
			jsonData := task.Content["State"]
			var data map[string]interface{}

			err = json.Unmarshal([]byte(jsonData), &data)
			if err != nil {
				log.WithError(err).Error("failed to unmarshal tasks content")
				return
			}

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

			updatedJsonData, err := json.Marshal(data)
			if err != nil {
				log.WithError(err).Error("failed to update tasks content")
				return
			}
			task.Content["State"] = updatedJsonData

			if initialPeople, ok := task.CurrentExecutorData["initial_people"]; ok {
				task.CurrentExecutorData["people"] = initialPeople
			}
		}
	}

	// обновить записи в бд

}
