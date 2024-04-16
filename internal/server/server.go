package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/configs"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/api"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/kafka"
	redisdb "gitlab.services.mts.ru/jocasta/pipeliner/internal/redis"
)

type Server struct {
	logger logger.Logger

	httpServer *http.Server

	kafka  *kafka.Service
	apiEnv *api.Env

	svcsPing *configs.ServicesPing

	consumerWorkerCh  chan kafka.RunnerInMessage
	consumerWorkerCnt int

	consumerTaskRunnerWorkerCh  chan kafka.RunTaskMessage
	consumerTaskRunnerWorkerCnt int
}

func NewServer(
	ctx context.Context,
	log logger.Logger,
	kf *kafka.Service,
	serverParam *api.ServerParam,
) *Server {
	httpServer, err := api.NewServer(ctx, serverParam)
	if err != nil {
		log.Fatal(err)
	}

	s := &Server{
		logger:     log,
		httpServer: httpServer,
		kafka:      kf,
		apiEnv:     serverParam.APIEnv,

		consumerWorkerCh:  make(chan kafka.RunnerInMessage),
		consumerWorkerCnt: serverParam.ConsumerWorkerCnt,

		consumerTaskRunnerWorkerCh:  make(chan kafka.RunTaskMessage),
		consumerTaskRunnerWorkerCnt: serverParam.ConsumerWorkerCnt,

		svcsPing: &configs.ServicesPing{
			PingTimer:    serverParam.SvcsPing.PingTimer,
			MaxFailedCnt: serverParam.SvcsPing.MaxFailedCnt,
			MaxOkCnt:     serverParam.SvcsPing.MaxOkCnt,
		},
	}

	s.startKafkaWorkers(ctx)

	go s.checkSvcsAvailability(ctx)

	return s
}

func (s *Server) Run(ctx context.Context) {
	go func() {
		s.logger.Info("script manager service started on port", s.httpServer.Addr)

		if err := s.httpServer.ListenAndServe(); err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				s.logger.Info("graceful shutdown")
			} else {
				s.logger.WithError(err).Fatal("script manager service")
			}
		}
	}()

	if err := s.rerunUnfinishedFunctions(ctx); err != nil {
		s.logger.WithError(err).Error("cannot rerun unfinished functions")
	}

	s.kafka.StartConsumer(ctx)
}

func (s *Server) Stop(ctx context.Context) {
	if err := s.httpServer.Shutdown(ctx); err != nil {
		s.logger.WithError(err).Error("error on http server shutdown")
	}

	if err := s.kafka.CloseProducer(); err != nil {
		s.logger.WithError(err).Error("error on producer shutdown")
	}

	s.kafka.StopConsumer()
}

func (s *Server) startKafkaWorkers(ctx context.Context) {
	for i := 0; i < s.consumerWorkerCnt; i++ {
		go s.apiEnv.WorkFunctionHandler(ctx, strconv.Itoa(i), s.consumerWorkerCh)
	}
}

//nolint:all // ok
func (s *Server) SendMessageToWorkers(_ context.Context, message kafka.RunnerInMessage) error {
	s.consumerWorkerCh <- message

	return nil
}

func (s *Server) checkSvcsAvailability(ctx context.Context) {
	failedCh := make(chan bool)

	go s.PingSvcs(ctx, failedCh)

	for {
		select {
		case areSvcsFailed := <-failedCh:
			if areSvcsFailed {
				s.kafka.StopConsumer()
			} else {
				s.kafka.StartConsumer(ctx)
			}
		default:
			continue
		}

		<-time.After(s.svcsPing.PingTimer)
	}
}

func (s *Server) PingSvcs(ctx context.Context, failedCh chan bool) {
	var (
		kafkaStopped bool
		failedCount  int
		okCount      int
	)

	for {
		<-time.After(s.svcsPing.PingTimer)

		dbErr := s.apiEnv.DB.Ping(ctx)
		sdlErr := s.apiEnv.Scheduler.Ping(ctx)

		if dbErr != nil || sdlErr != nil {
			if kafkaStopped {
				continue
			}

			okCount = 0

			failedCount++
			if failedCount < s.svcsPing.MaxFailedCnt {
				continue
			}

			kafkaStopped = true
			failedCh <- true

			continue
		}

		if !kafkaStopped {
			continue
		}

		okCount++
		if okCount < s.svcsPing.MaxOkCnt {
			continue
		}

		failedCount = 0

		kafkaStopped = false
		failedCh <- false
	}
}

func (s *Server) rerunUnfinishedFunctions(ctx context.Context) error {
	keys, keysErr := s.apiEnv.Rdb.Keys(ctx, redisdb.RunnerInMsgPrefix+"*").Result()
	if keysErr != nil {
		return fmt.Errorf("cannot get unfinished functions keys: %w", keysErr)
	}

	for _, k := range keys {
		msg, getErr := s.apiEnv.Rdb.GetRunnerInMsg(ctx, k)
		if getErr != nil {
			return fmt.Errorf("cannot get unfinished function result: %w", getErr)
		}

		s.apiEnv.FunctionReturnHandler(ctx, msg) //nolint:errcheck // Все ошибки уже обрабатываются внутри
	}

	return nil
}
