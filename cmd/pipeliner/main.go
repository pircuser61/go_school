package main

import (
	"context"
	"flag"
	db2 "gitlab.services.mts.ru/erius/pipeliner/internal/dbconn"
	"gitlab.services.mts.ru/erius/pipeliner/internal/handlers"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"go.opencensus.io/plugin/ochttp"

	"gitlab.services.mts.ru/erius/pipeliner/internal/configs"

	"contrib.go.opencensus.io/exporter/jaeger"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/cors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gitlab.services.mts.ru/erius/pipeliner/internal/metrics"
	"gitlab.services.mts.ru/libs/logger"
	"go.opencensus.io/trace"
)

func main() {
	configPath := flag.String(
		"c",
		"./config.yaml",
		"path to config",
	)
	flag.Parse()

	log := logger.CreateLogger(nil)

	metrics.InitMetricsAuth()

	cfg := &configs.Pipeliner{}
	err := configs.Read(*configPath, cfg)
	if err != nil {
		log.WithError(err).Fatal("can't read config")
	}
	log.WithField("config", cfg).Info("started with config")

	log = logger.CreateLogger(cfg.Log)

	database, err := db2.DBConnect(cfg.DB)
	if err != nil {
		log.WithError(err).Error("can't connect database")
		return
	}

	pipeliner := handlers.ApiEnv{
		DBConnection: database,
		Logger:       log,
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

	server := http.Server{
		Handler: registerRouter(log, *cfg, pipeliner),
		Addr:    cfg.ServeAddr,
	}

	go func() {
		log.Infof("script manager service started on port %s", server.Addr)
		if err = server.ListenAndServe(); err != nil {
			if err == http.ErrServerClosed {
				log.Info("graceful shutdown")
			} else {
				log.WithError(err).Fatal("script manager service")
			}
		}
	}()

	go func() {
		metricsMux := chi.NewRouter()
		metricsMux.Handle("/metrics", promhttp.Handler())

		log.Infof("metrics for script manager service started on port %s", cfg.MetricsAddr)
		if err = http.ListenAndServe(cfg.MetricsAddr, metricsMux); err != nil {
			if err == http.ErrServerClosed {
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

func registerRouter(log logger.Logger, cfg configs.Pipeliner, pipeliner handlers.ApiEnv) *chi.Mux {
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
	mux.Use(middleware.SetHeader("Content-Type", "text/json"))
	mux.Use(cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{http.MethodPost, http.MethodGet, http.MethodHead, http.MethodPatch, http.MethodPut},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "metadata"},
		ExposedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "metadata"},
		AllowCredentials: true,
		MaxAge:           300,
	}).Handler)

	mux.Route("/api/v1", func(r chi.Router) {
		r.Get("/pipelines/", pipeliner.ListPipelines)            // list all pipelines + approve requests for admin
		r.Get("/pipelines/{pipelineID}", pipeliner.GetPipeline) // one pipeline
		r.Get("/pipelines/version/{versionID}", pipeliner.GetPipelineVersion)
		r.Post("/pipelines/", pipeliner.CreatePipeline)
		r.Post("/pipelines/version/{pipelineID}", pipeliner.CreateDraft)
		r.Put("/pipelines/version/{versionID}", pipeliner.EditDraft)
		r.Delete("/pipelines/version/{versionID}", pipeliner.DeleteDraft)
		r.Delete("/pipelines/{pipelineID}", pipeliner.DeletePipeline)

		r.Get("/modules/", pipeliner.GetModules)

		r.Get("/tags/", pipeliner.GetTags)
		r.Post("/tags/", pipeliner.CreateTag)
		r.Put("/tags/{ID}", pipeliner.EditTag)
		r.Delete("/tags/{ID}", pipeliner.RemoveTag)

		r.Post("/run/{pipelineID}", pipeliner.RunPipeline)
	})
	return mux
}
