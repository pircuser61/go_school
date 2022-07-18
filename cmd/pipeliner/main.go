package main

//go:generate swag init -g ./cmd/pipeliner/main.go -o ./docs -d ../../.

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"contrib.go.opencensus.io/exporter/jaeger"

	"go.opencensus.io/trace"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/prometheus/client_golang/prometheus/push"

	httpSwagger "github.com/swaggo/http-swagger"

	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/abp/myosotis/observability"

	"gitlab.services.mts.ru/erius/monitoring/pkg/pipeliner/monitoring"
	netmon "gitlab.services.mts.ru/erius/network-monitor-client"
	scheduler "gitlab.services.mts.ru/erius/scheduler_client"

	"gitlab.services.mts.ru/jocasta/pipeliner/cmd/pipeliner/docs"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/configs"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db/mocks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/handlers"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/httpclient"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/metrics"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/server"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/test"
	"gitlab.services.mts.ru/jocasta/pipeliner/statistic"
)

const serviceName = "jocasta.pipeliner"

// @title Pipeliner API
// @version 0.1

// @host localhost:8181
// @BasePath /api/pipeliner/v1
func main() {
	configPath := flag.String("c", "./config.yaml", "path to config")
	flag.Parse()

	log := logger.CreateLogger(nil)

	cfg := &configs.Pipeliner{}

	err := configs.Read(*configPath, cfg)
	if err != nil {
		log.WithError(err).Fatal("can't read config")
	}

	log = logger.CreateLogger(cfg.Log)
	ctx := logger.WithLogger(context.Background(), log)

	log.WithField("config", cfg).Info("started with config")

	dbConn, err := db.ConnectPostgres(ctx, &cfg.DB)
	if err != nil {
		log.WithError(err).Error("can't connect database")

		return
	}

	httpClient := httpclient.HTTPClient(cfg.HTTPClientConfig)

	schedulerClient, err := scheduler.NewClient(cfg.SchedulerBaseURL.URL, httpClient)
	if err != nil {
		log.WithError(err).Error("can't create scheduler client")

		return
	}

	networkMonitoringClient, err := netmon.NewClient(cfg.NetworkMonitorBaseURL.URL, httpClient)
	if err != nil {
		log.WithError(err).Error("can't create network moninoring client")

		return
	}

	ssoService, err := sso.NewService(cfg.SSO, httpClient)
	if err != nil {
		log.WithError(err).Error("can't create sso service")

		return
	}

	peopleService, err := people.NewService(cfg.People, ssoService)
	if err != nil {
		log.WithError(err).Error("can't create people service")

		return
	}

	mailService, err := mail.NewService(cfg.Mail)
	if err != nil {
		log.WithError(err).Error("can't create mail service")

		return
	}

	stat, err := statistic.InitStatistic()
	if err != nil {
		log.WithError(err).Error("can't init statistic")

		return
	}

	// don't forget to update mock
	// TODO: remove MockDB and use MockedDatabase in tests
	var _ db.Database = (*mocks.MockedDatabase)(nil)
	var _ db.Database = (*test.MockDB)(nil)

	pipeliner := handlers.APIEnv{
		DB:                   &dbConn,
		ScriptManager:        cfg.ScriptManager,
		Remedy:               cfg.Remedy,
		FaaS:                 cfg.FaaS,
		SchedulerClient:      schedulerClient,
		NetworkMonitorClient: networkMonitoringClient,
		HTTPClient:           httpClient,
		Statistic:            stat,
		Mail:                 mailService,
		People:               peopleService,
	}

	jr, err := jaeger.NewExporter(jaeger.Options{
		CollectorEndpoint: cfg.Tracing.URL,
		Process: jaeger.Process{
			ServiceName: serviceName,
			Tags:        []jaeger.Tag{jaeger.StringTag("system", serviceName)},
		},
	})
	if err != nil {
		log.WithError(err).Error("can't create new exporter jaeger")
	} else {
		trace.RegisterExporter(jr)
		trace.ApplyConfig(trace.Config{DefaultSampler: trace.ProbabilitySampler(cfg.Tracing.SampleFraction)})
	}

	metrics.InitMetricsAuth()

	metrics.Pusher = push.New(cfg.Push.URL, cfg.Push.Job).Gatherer(metrics.Registry)

	initSwagger(cfg)

	httpServer := http.Server{
		Handler: registerRouter(ctx, cfg, &pipeliner, ssoService, peopleService),
		Addr:    cfg.ServeAddr,
	}

	go func() {
		log.Info("script manager service started on port", httpServer.Addr)

		if err = httpServer.ListenAndServe(); err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				log.Info("graceful shutdown")
			} else {
				log.WithError(err).Fatal("script manager service")
			}
		}
	}()

	grpcServer := server.NewGRPC(&server.GRPCConfig{
		Port: cfg.GRPCPort,
		Conn: dbConn,
	})
	go func() {
		if err = grpcServer.Listen(); err != nil {
			os.Exit(-2)
		}
	}()

	go func() {
		time.Sleep(time.Second)
		if err = server.ListenGRPCGW(&server.GRPCGWConfig{
			GRPCPort:   cfg.GRPCPort,
			GRPCGWPort: cfg.GRPCGWPort,
		}); err != nil {
			os.Exit(-3)
		}
	}()

	monitoring.Setup(cfg.Monitoring.Addr, &http.Client{Timeout: cfg.Monitoring.Timeout.Duration})

	sgnl := make(chan os.Signal, 1)
	signal.Notify(sgnl,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	stop := <-sgnl

	if err = httpServer.Shutdown(ctx); err != nil {
		log.WithError(err).Error("error on shutdown")
	}

	log.WithField("signal", stop).Info("stopping")
}

func registerRouter(ctx context.Context, cfg *configs.Pipeliner, pipeliner *handlers.APIEnv, ssoService *sso.Service,
	peopleService *people.Service) *chi.Mux {
	mux := chi.NewRouter()
	mux.Use(middleware.NoCache)
	mux.Use(handlers.LoggerMiddleware(logger.GetLogger(ctx)))
	mux.Use(observability.MiddlewareChi())
	mux.Use(handlers.RequestIDMiddleware)
	mux.Use(middleware.Timeout(cfg.Timeout.Duration))

	const baseURL = "/api/pipeliner/v1"

	mux.Mount(baseURL+"/pprof", middleware.Profiler())
	mux.Handle(baseURL+"/metrics", pipeliner.ServePrometheus())
	mux.Mount(baseURL+"/swagger", httpSwagger.Handler(httpSwagger.URL("../swagger/doc.json")))

	mux.With(middleware.SetHeader("Content-Type", "text/json")).
		Route(baseURL, func(r chi.Router) {
			r.Use(handlers.WithUserInfo(ssoService, logger.GetLogger(ctx)))
			r.Use(handlers.WithAsOtherUserInfo(peopleService, logger.GetLogger(ctx)))
			r.Use(handlers.StatisticMiddleware(pipeliner.Statistic))

			r.Get("/pipelines", pipeliner.ListPipelines)
			r.Post("/pipelines", pipeliner.CreatePipeline)
			r.Get("/pipelines/{pipelineID}", pipeliner.GetPipeline)
			r.Delete("/pipelines/{pipelineID}", pipeliner.DeletePipeline)

			r.Get("/pipelines/{pipelineID}/scheduler-tasks", pipeliner.ListSchedulerTasks)

			r.Put("/pipelines/{pipelineID}/tags/{ID}", pipeliner.AttachTag)
			r.Get("/pipelines/{pipelineID}/tags", pipeliner.GetPipelineTag)
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

	return mux
}

func initSwagger(cfg *configs.Pipeliner) {
	docs.SwaggerInfo.BasePath = cfg.Swag.BasePath
	docs.SwaggerInfo.Version = cfg.Swag.Version
	docs.SwaggerInfo.Host = cfg.Swag.Host + cfg.Swag.Port
}
