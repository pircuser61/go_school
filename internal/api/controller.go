package api

import (
	"context"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/functions"
	"net/http"
	"time"

	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"

	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/abp/myosotis/observability"
	netmon "gitlab.services.mts.ru/erius/network-monitor-client"
	scheduler "gitlab.services.mts.ru/erius/scheduler_client"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
	"gitlab.services.mts.ru/jocasta/pipeliner/statistic"
)

type APIEnv struct {
	DB                   db.Database
	Remedy               string
	FaaS                 string
	SchedulerClient      scheduler.Client
	NetworkMonitorClient netmon.Client
	HTTPClient           *http.Client
	Statistic            *statistic.Statistic
	Mail                 *mail.Service
	People               *people.Service
	ServiceDesc          *servicedesc.Service
	FunctionStore        *functions.Service
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

	return &http.Server{
		Addr:    param.ServerAddr,
		Handler: mux,
	}, nil
}
