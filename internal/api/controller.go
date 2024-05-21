package api

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"

	"github.com/hashicorp/go-retryablehttp"

	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/abp/myosotis/observability"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/configs"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	file_registry "gitlab.services.mts.ru/jocasta/pipeliner/internal/fileregistry"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/forms"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/functions"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/hrgate"
	human_tasks "gitlab.services.mts.ru/jocasta/pipeliner/internal/humantasks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/integrations"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/kafka"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	mail_fetcher "gitlab.services.mts.ru/jocasta/pipeliner/internal/mail/fetcher"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/metrics"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/scheduler"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sequence"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sla"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
	"gitlab.services.mts.ru/jocasta/pipeliner/statistic"
)

type Env struct {
	Log                     logger.Logger
	Metrics                 metrics.Metrics
	DB                      db.Database
	Remedy                  string
	FaaS                    string
	HTTPClient              *retryablehttp.Client
	Statistic               *statistic.Statistic
	Mail                    *mail.Service
	Kafka                   *kafka.Service
	People                  people.Service
	ServiceDesc             servicedesc.Service
	FunctionStore           functions.Service
	HumanTasks              human_tasks.ServiceInterface
	MailFetcher             mail_fetcher.Service
	FileRegistry            *file_registry.Service
	Integrations            *integrations.Service
	HrGate                  hrgate.ServiceInterface
	Scheduler               *scheduler.Service
	SLAService              sla.Service
	Forms                   *forms.Service
	Sequence                *sequence.Service
	IncludePlaceholderBlock bool
	HostURL                 string
	LogIndex                string
	FuncMsgResendDelay      time.Duration
}

type ServerParam struct {
	APIEnv            *Env
	SSOService        *sso.Service
	PeopleService     people.Service
	TimeoutMiddleware time.Duration
	ServerAddr        string

	ConsumerFuncsWorkers int
	ConsumerTasksWorkers int

	SvcsPing *configs.ServicesPing

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
	mux.Handle("/metrics", param.APIEnv.Metrics.ServePrometheus())

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
