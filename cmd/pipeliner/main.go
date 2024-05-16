package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"contrib.go.opencensus.io/exporter/jaeger"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/api"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/configs"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db/mocks"
	file_registry "gitlab.services.mts.ru/jocasta/pipeliner/internal/fileregistry"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/forms"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/functions"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/hrgate"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/httpclient"
	human_tasks "gitlab.services.mts.ru/jocasta/pipeliner/internal/humantasks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/integrations"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/kafka"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	mail_fetcher "gitlab.services.mts.ru/jocasta/pipeliner/internal/mail/fetcher"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/metrics"
	iga_cache "gitlab.services.mts.ru/jocasta/pipeliner/internal/people/cache"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/scheduler"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sequence"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/server"
	sd_cache "gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc/cache"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sla"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
	"gitlab.services.mts.ru/jocasta/pipeliner/statistic"
)

const serviceName = "jocasta.pipeliner"

// @title Pipeliner API
// @version 0.1

// @host localhost:8181
// @BasePath /api/pipeliner/v1
//
//nolint:gocyclo //it's ok here
func main() {
	configPath := flag.String("c", "cmd/pipeliner/config.yaml", "path to config")
	flag.Parse()

	log := logger.CreateLogger(nil)

	cfg := &configs.Pipeliner{}

	err := configs.Read(*configPath, cfg)
	if err != nil {
		log.WithError(err).Fatal("can't read config")
	}

	log = logger.CreateLogger(cfg.Log)
	ctx := logger.WithLogger(context.Background(), log)

	metrics.InitMetricsAuth(cfg.Prometheus)

	m := metrics.New(cfg.Prometheus)

	log.WithField("config", cfg).Info("started with config")

	dbConn, err := db.ConnectPostgres(ctx, &cfg.DB)
	if err != nil {
		log.WithError(err).Error("can't connect database")

		return
	}

	httpClient := httpclient.NewClient(
		httpclient.HTTPClient(cfg.HTTPClientConfig), log, cfg.HTTPClientConfig.MaxRetries, cfg.HTTPClientConfig.RetryDelay,
	)

	ssoService, err := sso.NewService(cfg.SSO)
	if err != nil {
		log.WithError(err).Error("can't create sso service")

		return
	}

	peopleService, err := iga_cache.NewService(&cfg.People, ssoService, m)
	if err != nil {
		log.WithError(err).Error("can't create people service")

		return
	}

	serviceDescService, err := sd_cache.NewService(&cfg.ServiceDesc, ssoService)
	if err != nil {
		log.WithError(err).Error("can't create servicedesc service")

		return
	}

	cfg.Mail.FetchEmail = cfg.MailFetcher.ImapUserName

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
	var _ db.Database = (*mocks.MockedDatabase)(nil)

	kafkaService, canRestart, err := kafka.NewService(log, cfg.Kafka, m)
	if err != nil {
		log.WithError(err).Error("can't create kafka service")

		if !canRestart {
			return
		}
	}

	schedulerService, err := scheduler.NewService(cfg.SchedulerTasks, log)
	if err != nil {
		log.WithError(err).Error("can't create scheduler service")

		return
	}

	functionsService, err := functions.NewService(cfg.FunctionStore, log)
	if err != nil {
		log.WithError(err).Error("can't create functions service")

		return
	}

	humanTasksService, err := human_tasks.NewService(&cfg.HumanTasks, log)
	if err != nil {
		log.WithError(err).Error("can't create human tasks service")

		return
	}

	mailFetcher, err := mail_fetcher.NewService(cfg.MailFetcher)
	if err != nil {
		log.WithError(err).Error("can't create mail fetcher service")

		return
	}

	integrationsService, err := integrations.NewService(cfg.Integrations, log)
	if err != nil {
		log.WithError(err).Error("can't create integrations service")

		return
	}

	hrgateService, err := hrgate.NewService(&cfg.HrGate, ssoService)
	if err != nil {
		log.WithError(err).Error("can't create hrgate service")

		return
	}

	fillErr := hrgateService.FillDefaultUnitID(ctx)
	if fillErr != nil {
		log.WithError(fillErr).Error("can't fill default unit id")
	}

	fileRegistryService, err := file_registry.NewService(cfg.FileRegistry, log)
	if err != nil {
		log.WithError(err).Error("can't create file-registry service")

		return
	}

	formsService, err := forms.NewService(cfg.Forms, log)
	if err != nil {
		log.WithError(err).Error("can't create forms service")

		return
	}

	sequenceService, err := sequence.NewService(cfg.Sequence, log)
	if err != nil {
		log.WithError(err).Error("can't create sequence service")

		return
	}

	slaService := sla.NewSLAService(hrgateService)

	includePlaceholderBlock := cfg.IncludePlaceholderBlock

	APIEnv := &api.Env{
		Log:                     log,
		Metrics:                 m,
		DB:                      &dbConn,
		Remedy:                  cfg.Remedy,
		FaaS:                    cfg.FaaS,
		HTTPClient:              httpClient,
		Statistic:               stat,
		Mail:                    mailService,
		Kafka:                   kafkaService,
		People:                  peopleService,
		ServiceDesc:             serviceDescService,
		FunctionStore:           functionsService,
		HumanTasks:              humanTasksService,
		MailFetcher:             mailFetcher,
		FileRegistry:            fileRegistryService,
		Integrations:            integrationsService,
		HrGate:                  hrgateService,
		Scheduler:               schedulerService,
		IncludePlaceholderBlock: includePlaceholderBlock,
		SLAService:              slaService,
		Forms:                   formsService,
		Sequence:                sequenceService,
		HostURL:                 cfg.HostURL,
		LogIndex:                cfg.LogIndex,
		FuncMsgResendDelay:      cfg.Kafka.FuncMessageResendDelay,
	}

	serverParam := api.ServerParam{
		APIEnv:               APIEnv,
		SSOService:           ssoService,
		PeopleService:        peopleService,
		TimeoutMiddleware:    cfg.Timeout,
		ServerAddr:           cfg.ServeAddr,
		ReadinessPath:        cfg.Probes.Readiness,
		LivenessPath:         cfg.Probes.Liveness,
		ConsumerFuncsWorkers: cfg.ConsumerFuncsWorkers,
		ConsumerTasksWorkers: cfg.ConsumerTasksWorkers,
		SvcsPing: &configs.ServicesPing{
			PingTimer:    cfg.ServicesPing.PingTimer,
			MaxFailedCnt: cfg.ServicesPing.MaxFailedCnt,
			MaxOkCnt:     cfg.ServicesPing.MaxOkCnt,
		},
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

	s := server.NewServer(ctx, log, kafkaService, &serverParam)

	kafkaService.InitMessageHandler(s.SendMessageToWorkers, s.SendRunTaskMessageToWorkers)

	go kafkaService.StartCheckHealth()

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
