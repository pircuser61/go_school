package api

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"

	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/abp/myosotis/observability"
	netmon "gitlab.services.mts.ru/erius/network-monitor-client"
	scheduler "gitlab.services.mts.ru/erius/scheduler_client"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/file"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/functions"
	human_tasks "gitlab.services.mts.ru/jocasta/pipeliner/internal/human-tasks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/integrations"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/kafka"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	mail_fetcher "gitlab.services.mts.ru/jocasta/pipeliner/internal/mail/fetcher"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
	"gitlab.services.mts.ru/jocasta/pipeliner/statistic"
)

type APIEnv struct {
	Log                  logger.Logger
	DB                   db.Database
	Remedy               string
	FaaS                 string
	SchedulerClient      scheduler.Client
	NetworkMonitorClient netmon.Client
	HTTPClient           *http.Client
	Statistic            *statistic.Statistic
	Mail                 *mail.Service
	Kafka                *kafka.Service
	People               *people.Service
	ServiceDesc          *servicedesc.Service
	FunctionStore        *functions.Service
	HumanTasks           *human_tasks.Service
	MailFetcher          mail_fetcher.Service
	Minio                *file.Service
	Integrations         *integrations.Service
}

type ServerParam struct {
	APIEnv            *APIEnv
	SSOService        *sso.Service
	PeopleService     *people.Service
	TimeoutMiddleware time.Duration
	ServerAddr        string

	LivenessPath  string
	ReadinessPath string
}

func NewServer(ctx context.Context, param *ServerParam) (*http.Server, error) {
	mux := chi.NewRouter()
	mux.Use(middleware.NoCache)
	mux.Use(LoggerMiddleware(logger.GetLogger(ctx)))
	mux.Use(observability.MiddlewareChi())
	mux.Use(RequestIDMiddleware)
	mux.Use(middleware.Timeout(param.TimeoutMiddleware))

	mux.Get(param.LivenessPath, param.APIEnv.Alive)
	mux.Get(param.ReadinessPath, param.APIEnv.Ready)

	const (
		baseURL = "/api/pipeliner/v1"
	)

	mux.Mount(baseURL+"/pprof", middleware.Profiler())
	mux.Handle(baseURL+"/metrics", param.APIEnv.ServePrometheus())

	mux.With(middleware.SetHeader("Content-Type", "text/json")).
		Route(baseURL, func(r chi.Router) {
			r.Use(WithUserInfo(param.SSOService, logger.GetLogger(ctx)))
			r.Use(WithAsOtherUserInfo(param.PeopleService, logger.GetLogger(ctx)))
			r.Use(StatisticMiddleware(param.APIEnv.Statistic))
			r.Use(SetAuthTokenInContext(logger.GetLogger(ctx)))

			HandlerFromMux(param.APIEnv, r)
		})

	//nolint:gosec // no lint ReadHeaderTimeout
	return &http.Server{
		Addr:    param.ServerAddr,
		Handler: mux,
	}, nil
}
