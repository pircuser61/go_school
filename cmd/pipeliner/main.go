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

	"contrib.go.opencensus.io/exporter/jaeger"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/trace"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus/push"
	httpSwagger "github.com/swaggo/http-swagger"

	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/abp/myosotis/observability"
	"gitlab.services.mts.ru/erius/admin/pkg/auth"
	"gitlab.services.mts.ru/erius/monitoring/pkg/pipeliner/monitoring"
	scheduler "gitlab.services.mts.ru/erius/scheduler_client"

	"gitlab.services.mts.ru/erius/pipeliner/cmd/pipeliner/docs"
	"gitlab.services.mts.ru/erius/pipeliner/internal/configs"
	"gitlab.services.mts.ru/erius/pipeliner/internal/db"
	"gitlab.services.mts.ru/erius/pipeliner/internal/handlers"
	"gitlab.services.mts.ru/erius/pipeliner/internal/httpclient"
	"gitlab.services.mts.ru/erius/pipeliner/internal/metrics"
)

const serviceName = "erius.pipeliner"

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
	auth.InjectTransport(httpClient)

	authClient, err := auth.NewClient(cfg.AuthBaseURL.URL, httpClient)
	if err != nil {
		log.WithError(err).Error("can't create auth client")

		return
	}

	schedulerClient, err := scheduler.NewClient(cfg.SchedulerBaseURL.URL, httpClient)
	if err != nil {
		log.WithError(err).Error("can't create scheduler client")

		return
	}

	pipeliner := handlers.APIEnv{
		DB:              &dbConn,
		ScriptManager:   cfg.ScriptManager,
		Remedy:          cfg.Remedy,
		FaaS:            cfg.FaaS,
		AuthClient:      authClient,
		SchedulerClient: schedulerClient,
		HTTPClient:      httpClient,
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

	server := http.Server{
		Handler: registerRouter(ctx, cfg, &pipeliner),
		Addr:    cfg.ServeAddr,
	}

	go func() {
		log.Info("script manager service started on port", server.Addr)

		if err = server.ListenAndServe(); err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				log.Info("graceful shutdown")
			} else {
				log.WithError(err).Fatal("script manager service")
			}
		}
	}()

	monitoring.Setup(cfg.Monitoring.Addr, &http.Client{Timeout: cfg.Monitoring.Timeout.Duration})

	go func() {
		metricsMux := chi.NewRouter()
		metricsMux.Handle("/metrics", promhttp.Handler())

		log.Info("metrics for script manager service started on port", cfg.MetricsAddr)

		if err = http.ListenAndServe(cfg.MetricsAddr, metricsMux); err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				log.Info("graceful shutdown")
			} else {
				log.WithError(err).Fatal("script manager metrics")
			}
		}
	}()

	sgnl := make(chan os.Signal, 1)
	signal.Notify(sgnl,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	stop := <-sgnl

	if err = server.Shutdown(ctx); err != nil {
		log.WithError(err).Error("error on shutdown")
	}

	log.WithField("signal", stop).Info("stopping")
}

func registerRouter(ctx context.Context, cfg *configs.Pipeliner, pipeliner *handlers.APIEnv) *chi.Mux {
	mux := chi.NewRouter()
	mux.Use(middleware.NoCache)
	mux.Use(LoggerMiddleware(logger.GetLogger(ctx)))
	mux.Use(observability.MiddlewareChi())
	mux.Use(middleware.Timeout(cfg.Timeout.Duration))

	const baseURL = "/api/pipeliner/v1"

	mux.With(middleware.SetHeader("Content-Type", "text/json")).
		Route(baseURL, func(r chi.Router) {
			r.Use(auth.UserMiddleware(pipeliner.AuthClient))
			r.Get("/pipelines/", pipeliner.ListPipelines)
			r.Post("/pipelines/", pipeliner.CreatePipeline)
			r.Get("/pipelines/{pipelineID}", pipeliner.GetPipeline)
			r.Delete("/pipelines/{pipelineID}", pipeliner.DeletePipeline)

			r.Get("/pipelines/{pipelineID}/scheduler-tasks", pipeliner.ListSchedulerTasks)

			r.Put("/pipelines/{pipelineID}/tags/{ID}", pipeliner.AttachTag)
			r.Get("/pipelines/{pipelineID}/tags/", pipeliner.GetPipelineTag)
			r.Delete("/pipelines/{pipelineID}/tags/{ID}", pipeliner.DetachTag)

			r.Get("/pipelines/version/{versionID}", pipeliner.GetPipelineVersion)
			r.Post("/pipelines/version/{pipelineID}", pipeliner.CreatePipelineVersion)
			r.Put("/pipelines/version/", pipeliner.EditVersion)
			r.Delete("/pipelines/version/{versionID}", pipeliner.DeleteVersion)

			r.Get("/modules/", pipeliner.GetModules)
			r.Get("/modules/usage", pipeliner.AllModulesUsage)
			r.Get("/modules/{moduleName}/usage", pipeliner.ModuleUsage)
			r.Post("/modules/{moduleName}", pipeliner.ModuleRun)

			r.Get("/tags/", pipeliner.GetTags)
			r.Post("/tags/", pipeliner.CreateTag)
			r.Put("/tags/", pipeliner.EditTag)
			r.Delete("/tags/{ID}", pipeliner.RemoveTag)

			r.With(handlers.SetRequestID).Post("/run/{pipelineID}", pipeliner.RunPipeline)
			r.With(handlers.SetRequestID).Post("/run/version/{versionID}", pipeliner.RunVersion)

			r.Route("/tasks/", func(r chi.Router) {
				r.Get("/{taskID}", pipeliner.GetTask)
				r.Get("/last-by-version/{versionID}", pipeliner.LastVersionDebugTask)
				r.Get("/pipeline/{pipelineID}", pipeliner.GetPipelineTasks)
				r.Get("/version/{versionID}", pipeliner.GetVersionTasks)
			})
		})

	mux.Mount(baseURL+"/debug/", middleware.Profiler())
	mux.Mount(baseURL+"/swagger/", httpSwagger.Handler(httpSwagger.URL("../swagger/doc.json")))

	return mux
}

func initSwagger(cfg *configs.Pipeliner) {
	docs.SwaggerInfo.BasePath = cfg.Swag.BasePath
	docs.SwaggerInfo.Version = cfg.Swag.Version
	docs.SwaggerInfo.Host = cfg.Swag.Host + cfg.Swag.Port
}

func LoggerMiddleware(log logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return ochttp.Handler{
			Handler: http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
				ctxLocal, span := trace.StartSpan(req.Context(), req.Method+" "+req.URL.String())
				defer span.End()

				newLogger := log.WithField("TraceID", trace.FromContext(ctxLocal).SpanContext().TraceID.String())
				newLogger.WithFields(map[string]interface{}{
					"method": req.Method,
					"url":    req.URL.String(),
					"host":   req.Host,
				}).Info("got request")
				ctx := logger.WithLogger(ctxLocal, newLogger)

				next.ServeHTTP(res, req.WithContext(ctx))
			}),
		}.Handler
	}
}
