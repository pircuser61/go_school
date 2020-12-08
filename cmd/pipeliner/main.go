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
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus/push"
	httpSwagger "github.com/swaggo/http-swagger"

	"gitlab.services.mts.ru/abp/myosotis/logger"
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

const (
	maxAge = 300
)

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

	log.WithField("config", cfg).Info("started with config")

	log = logger.CreateLogger(cfg.Log)

	dbConn, err := db.ConnectPostgres(&cfg.DB)
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
		Logger:          log,
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
			ServiceName: "no-auth",
			Tags:        []jaeger.Tag{jaeger.StringTag("system", "pipeliner")},
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
		Handler: registerRouter(log, cfg, &pipeliner),
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

	if err = server.Shutdown(context.Background()); err != nil {
		log.WithError(err).Error("error on shutdown")
	}

	log.WithField("signal", stop).Info("stopping")
}

func registerRouter(log logger.Logger, cfg *configs.Pipeliner, pipeliner *handlers.APIEnv) *chi.Mux {
	mux := chi.NewRouter()
	mux.Use(middleware.NoCache)
	mux.Use(func(next http.Handler) http.Handler {
		return ochttp.Handler{
			Handler: http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
				ctx := logger.WithLogger(req.Context(), log)

				next.ServeHTTP(res, req.WithContext(ctx))
			}),
		}.Handler
	})

	mux.Use(middleware.Timeout(cfg.Timeout.Duration))
	mux.Use(cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{http.MethodPost, http.MethodGet, http.MethodHead, http.MethodPatch, http.MethodPut},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "metadata"},
		ExposedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "metadata"},
		AllowCredentials: true,
		MaxAge:           maxAge,
	}).Handler)

	mux.With(middleware.SetHeader("Content-Type", "text/json")).
		Route("/api/pipeliner/v1", func(r chi.Router) {
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
			r.Put("/tags/{ID}", pipeliner.EditTag)
			r.Delete("/tags/{ID}", pipeliner.RemoveTag)

			r.With(handlers.SetRequestID).Post("/run/{pipelineID}", pipeliner.RunPipeline)
			r.With(handlers.SetRequestID).Post("/run/version/{versionID}", pipeliner.RunVersion)

			r.Route("/tasks/", func(r chi.Router) {
				r.Get("/{taskID}", pipeliner.GetTask)
				r.Get("/last-by-version/{versionID}", pipeliner.LastVersionDebugTask)
				r.Get("/pipeline/{pipelineID}", pipeliner.GetPipelineTasks)
				r.Get("/version/{versionID}", pipeliner.GetVersionTasks)
			})

			r.Route("/m-debug/", func(r chi.Router) {
				r.Post("/", pipeliner.CreateDebugTask)
				r.Post("/run", pipeliner.StartDebugTask)
			})
		})

	mux.Mount("/api/pipeliner/v1/swagger/", httpSwagger.Handler(httpSwagger.URL("../swagger/doc.json")))

	return mux
}

func initSwagger(cfg *configs.Pipeliner) {
	docs.SwaggerInfo.BasePath = cfg.Swag.BasePath
	docs.SwaggerInfo.Version = cfg.Swag.Version
	docs.SwaggerInfo.Host = cfg.Swag.Host + cfg.Swag.Port
}
