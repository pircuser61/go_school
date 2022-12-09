package main

//go:generate swag init -g ./cmd/pipeliner/main.go -o ./docs -d ../../.

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"contrib.go.opencensus.io/exporter/jaeger"

	"go.opencensus.io/trace"

	"github.com/prometheus/client_golang/prometheus/push"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/erius/monitoring/pkg/pipeliner/monitoring"
	netmon "gitlab.services.mts.ru/erius/network-monitor-client"
	scheduler "gitlab.services.mts.ru/erius/scheduler_client"

	"gitlab.services.mts.ru/jocasta/pipeliner/cmd/pipeliner/docs"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/api"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/configs"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db/mocks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/httpclient"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/kafka"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/metrics"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/server"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/test"
	"gitlab.services.mts.ru/jocasta/pipeliner/statistic"
)

const serviceName = "jocasta.pipeliner"

// @title Pipeliner API
// @version 0.1

// @host localhost:8181
// @BasePath /api/pipeliner/v1
//
//nolint:gocyclo //its ok here
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

	serviceDescService, err := servicedesc.NewService(cfg.ServiceDesc, ssoService)
	if err != nil {
		log.WithError(err).Error("can't create servicedesc service")

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

	kafkaService, err := kafka.NewService(log, cfg.Kafka)
	if err != nil {
		log.WithError(err).Error("can't create kafka service")

		return
	}

	APIEnv := &api.APIEnv{
		DB:                   &dbConn,
		Remedy:               cfg.Remedy,
		FaaS:                 cfg.FaaS,
		SchedulerClient:      schedulerClient,
		NetworkMonitorClient: networkMonitoringClient,
		HTTPClient:           httpClient,
		Statistic:            stat,
		Mail:                 mailService,
		Kafka:                kafkaService,
		People:               peopleService,
		ServiceDesc:          serviceDescService,
	}

	serverParam := api.ServerParam{
		APIEnv:            APIEnv,
		SSOService:        ssoService,
		PeopleService:     peopleService,
		TimeoutMiddleware: cfg.Timeout.Duration,
		ServerAddr:        cfg.ServeAddr,
	}

	kafkaService.InitMessageHandler(APIEnv.FunctionReturnHandler)

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

	grpcServerParam := &server.GRPCConfig{
		Port: cfg.GRPCPort,
		Conn: dbConn,
	}

	grpcGWServerParam := &server.GRPCGWConfig{
		GRPCPort:   cfg.GRPCPort,
		GRPCGWPort: cfg.GRPCGWPort,
	}

	monitoring.Setup(cfg.Monitoring.Addr, &http.Client{Timeout: cfg.Monitoring.Timeout.Duration})

	s := server.NewServer(ctx, log, kafkaService, serverParam, grpcServerParam, grpcGWServerParam)
	s.Run(ctx)

	sgnl := make(chan os.Signal, 1)
	signal.Notify(sgnl,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	stop := <-sgnl
	s.Stop(ctx)
	log.WithField("signal", stop).Info("stopping")
}

func initSwagger(cfg *configs.Pipeliner) {
	docs.SwaggerInfo.BasePath = cfg.Swag.BasePath
	docs.SwaggerInfo.Version = cfg.Swag.Version
	docs.SwaggerInfo.Host = cfg.Swag.Host + cfg.Swag.Port
}
