package api

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
	httpSwagger "github.com/swaggo/http-swagger"
	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/abp/myosotis/observability"
	netmon "gitlab.services.mts.ru/erius/network-monitor-client"
	scheduler "gitlab.services.mts.ru/erius/scheduler_client"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/handlers"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
	"gitlab.services.mts.ru/jocasta/pipeliner/statistic"
)

type APIEnv struct {
	DB                   db.Database
	ScriptManager        string
	Remedy               string
	FaaS                 string
	SchedulerClient      scheduler.Client
	NetworkMonitorClient netmon.Client
	HTTPClient           *http.Client
	Statistic            *statistic.Statistic
	Mail                 *mail.Service
	People               *people.Service
}

type ServerParam struct {
	APIEnv            *APIEnv
	SSOService        *sso.Service
	PeopleService     *people.Service
	TimeoutMiddleware time.Duration
	ServerAddr        string
}

func NewServer(ctx context.Context, param ServerParam) (*http.Server, error) {
	mux := chi.NewRouter()
	mux.Use(middleware.NoCache)
	mux.Use(LoggerMiddleware(logger.GetLogger(ctx)))
	mux.Use(observability.MiddlewareChi())
	mux.Use(RequestIDMiddleware)
	mux.Use(middleware.Timeout(param.TimeoutMiddleware))

	const (
		baseURL1 = "/api/pipeliner/v1"
		baseURL2 = "/api/pipeliner/v2"
	)

	mux.Mount(baseURL2+"/pprof", middleware.Profiler())
	mux.Handle(baseURL2+"/metrics", param.APIEnv.ServePrometheus())

	pipeliner := &handlers.APIEnv{
		DB:                   param.APIEnv.DB,
		ScriptManager:        param.APIEnv.ScriptManager,
		Remedy:               param.APIEnv.Remedy,
		FaaS:                 param.APIEnv.FaaS,
		SchedulerClient:      param.APIEnv.SchedulerClient,
		NetworkMonitorClient: param.APIEnv.NetworkMonitorClient,
		HTTPClient:           param.APIEnv.HTTPClient,
		Statistic:            param.APIEnv.Statistic,
		Mail:                 param.APIEnv.Mail,
		People:               param.APIEnv.People,
	}

	mux.With(middleware.SetHeader("Content-Type", "text/json")).
		Route(baseURL2, func(r chi.Router) {
			r.Use(WithUserInfo(param.SSOService, logger.GetLogger(ctx)))
			r.Use(WithAsOtherUserInfo(param.PeopleService, logger.GetLogger(ctx)))
			r.Use(StatisticMiddleware(param.APIEnv.Statistic))

			HandlerFromMux(param.APIEnv, r)
		})

	mux.Mount(baseURL1+"/pprof", middleware.Profiler())
	mux.Handle(baseURL1+"/metrics", pipeliner.ServePrometheus())
	mux.Mount(baseURL1+"/swagger", httpSwagger.Handler(httpSwagger.URL("../swagger/doc.json")))

	mux.With(middleware.SetHeader("Content-Type", "text/json")).
		Route(baseURL1, func(r chi.Router) {
			r.Use(handlers.WithUserInfo(param.SSOService, logger.GetLogger(ctx)))
			r.Use(handlers.WithAsOtherUserInfo(param.PeopleService, logger.GetLogger(ctx)))
			r.Use(handlers.StatisticMiddleware(pipeliner.Statistic))

			r.Get("/pipelines", pipeliner.ListPipelines)
			r.Post("/pipelines", pipeliner.CreatePipeline)
			r.Get("/pipelines/{pipelineID}", pipeliner.GetPipeline)
			r.Delete("/pipelines/{pipelineID}", pipeliner.DeletePipeline)

			r.Get("/pipelines/{pipelineID}/scheduler-tasks", pipeliner.ListSchedulerTasks)

			r.Put("/pipelines/{pipelineID}/tags/{ID}", pipeliner.AttachTag)
			r.Get("/pipelines/{pipelineID}/tags", pipeliner.GetPipelineTags)
			r.Delete("/pipelines/{pipelineID}/tags/{ID}", pipeliner.DetachTag)

			r.Get("/pipelines/version/{versionID}", pipeliner.GetPipelineVersion)
			r.Post("/pipelines/version/{pipelineID}", pipeliner.CreatePipelineVersion)
			r.Put("/pipelines/version", pipeliner.EditVersion)
			r.Delete("/pipelines/version/{versionID}", pipeliner.DeleteVersion)

			r.Get("/modules", pipeliner.GetModules)
			r.Get("/modules/usage", pipeliner.AllModulesUsage)
			r.Get("/modules/{moduleName}/usage", pipeliner.ModuleUsage)
			r.Post("/modules/{moduleName}", pipeliner.ModuleRun)

			r.Get("/tags", pipeliner.GetTags)
			r.Post("/tags", pipeliner.CreateTag)
			r.Put("/tags", pipeliner.EditTag)
			r.Delete("/tags/{ID}", pipeliner.RemoveTag)

			r.Post("/run/{pipelineID}", pipeliner.RunPipeline)
			r.Post("/run/version/{versionID}", pipeliner.RunVersion)
			r.Post("/run/versions/blueprint_id", pipeliner.RunVersionsByBlueprintID)
			r.Post("/run/version/new_version", pipeliner.RunNewVersionByPrevVersion)

			r.Get("/tasks", pipeliner.GetTasks)

			r.Route("/tasks/", func(r chi.Router) {
				r.Get("/{workNumber}", pipeliner.GetTask)
				r.Post("/{workNumber}", pipeliner.UpdateTask)
				r.Get("/last-by-version/{versionID}", pipeliner.LastVersionDebugTask)
				r.Get("/pipeline/{pipelineID}", pipeliner.GetPipelineTasks)
				r.Get("/version/{versionID}", pipeliner.GetVersionTasks)
				r.Get("/count", pipeliner.GetTasksCount)
			})
			r.Route("/debug/", func(r chi.Router) {
				r.Post("/run", pipeliner.StartDebugTask)
				r.Post("/", pipeliner.CreateDebugTask)
				r.Get("/{workNumber}", pipeliner.DebugTask)
			})
		})

	return &http.Server{
		Addr:    param.ServerAddr,
		Handler: mux,
	}, nil
}
