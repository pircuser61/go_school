package server

import (
	c "context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/api"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/configs"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/kafka"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/metrics"
)

type service struct {
	logger logger.Logger

	httpServer *http.Server

	kafka  *kafka.Service
	apiEnv *api.Env

	servicesPing *configs.ServicesPing

	consumerWorkerCh  chan kafka.TimedRunnerInMessage
	consumerWorkerCnt int

	consumerRunTaskWorkerCh chan kafka.TimedRunTaskMessage
	consumerRunTaskWorkers  int

	metrics metrics.Metrics

	pings map[string]func()
}

//nolint:all //ok
func NewServer(ctx c.Context, log logger.Logger, kf *kafka.Service, params *api.ServerParam, m metrics.Metrics) *service {
	httpServer, err := api.NewServer(ctx, params)
	if err != nil {
		log.Fatal(err)
	}

	s := &service{
		logger:     log,
		httpServer: httpServer,
		kafka:      kf,
		apiEnv:     params.APIEnv,
		metrics:    m,

		consumerWorkerCh:  make(chan kafka.TimedRunnerInMessage),
		consumerWorkerCnt: params.ConsumerFuncsWorkers,

		consumerRunTaskWorkerCh: make(chan kafka.TimedRunTaskMessage),
		consumerRunTaskWorkers:  params.ConsumerTasksWorkers,

		servicesPing: &configs.ServicesPing{
			PingTimer:    params.SvcsPing.PingTimer,
			MaxFailedCnt: params.SvcsPing.MaxFailedCnt,
			MaxOkCnt:     params.SvcsPing.MaxOkCnt,
		},
	}

	s.startKafkaWorkers(ctx)

	go s.checkServicesAvailability(ctx)

	return s
}

func (s *service) Run(ctx c.Context) {
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

	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		defer wg.Done()

		if err := s.rerunUnfinishedFunctions(ctx); err != nil {
			s.logger.WithError(err).Error("cannot rerun unfinished functions")
		}
	}()

	go func() {
		defer wg.Done()

		if err := s.rerunUnfinishedTasks(ctx); err != nil {
			s.logger.WithError(err).Error("cannot rerun unfinished functions")
		}
	}()

	wg.Wait()

	s.kafka.StartConsumer(ctx)
}

func (s *service) Stop(ctx c.Context) {
	if err := s.httpServer.Shutdown(ctx); err != nil {
		s.logger.WithError(err).Error("error on http server shutdown")
	}

	if err := s.kafka.CloseProducer(); err != nil {
		s.logger.WithError(err).Error("error on producer shutdown")
	}

	s.kafka.StopConsumer()
}

func (s *service) startKafkaWorkers(ctx c.Context) {
	for i := 0; i < s.consumerWorkerCnt; i++ {
		go s.apiEnv.WorkFunctionHandler(ctx, s.consumerWorkerCh)
	}

	for i := 0; i < s.consumerRunTaskWorkers; i++ {
		go s.apiEnv.WorkRunTaskHandler(ctx, s.consumerRunTaskWorkerCh)
	}
}

//nolint:all // ok
func (s *service) SendMessageToWorkers(ctx c.Context, message kafka.RunnerInMessage) error {
	timedMsg := kafka.TimedRunnerInMessage{
		Msg:     message,
		TimeNow: time.Now(),
	}

	s.logger.Info("Получено сообщение из functions: ", message.TaskID) // TODO: DEV-STAGE only

	if err := s.kafka.SetRunnerInMsg(ctx, strconv.Itoa(int(timedMsg.TimeNow.Unix())), &message); err != nil {
		s.logger.WithField("stepID", message.TaskID).
			WithError(err).Error("cannot set function-result message to cache")
	}

	s.consumerWorkerCh <- timedMsg

	return nil
}

//nolint:all // ok
func (s *service) SendRunTaskMessageToWorkers(ctx c.Context, message kafka.RunTaskMessage) error {
	timedMsg := kafka.TimedRunTaskMessage{
		Msg:     message,
		TimeNow: time.Now(),
	}

	if err := s.kafka.SetRunTaskMsg(ctx, strconv.Itoa(int(timedMsg.TimeNow.Unix())), &message); err != nil {
		s.logger.WithField("workNumber", message.WorkNumber).
			WithError(err).Error("cannot set run-task message to cache")
	}

	s.consumerRunTaskWorkerCh <- timedMsg

	return nil
}

func (s *service) checkServicesAvailability(ctx c.Context) {
	failedCh := make(chan bool)

	go s.PingServices(ctx, failedCh)

	for {
		areServicesFailed := <-failedCh
		if areServicesFailed {
			s.kafka.StopConsumer()
		} else {
			s.kafka.StartConsumer(ctx)
		}
	}
}

func (s *service) rerunUnfinishedFunctions(ctx c.Context) error {
	keys, keysErr := s.kafka.GetCachedKeys(ctx, kafka.RunnerInMsgPrefix+"*")
	if keysErr != nil {
		return fmt.Errorf("got error from GetCachedKeys: %w", keysErr)
	}

	for _, fullkey := range keys {
		keyParts := strings.Split(fullkey, ":")
		if len(keyParts) == 1 {
			continue
		}

		key := keyParts[1]

		msg, getErr := s.kafka.GetRunnerInMsg(ctx, key)
		if getErr != nil {
			s.logger.WithError(getErr).Error("cannot get unfinished function result")

			continue
		}

		s.logger.Info("restored unfinished function message")

		s.apiEnv.FunctionReturnHandler(ctx, msg) //nolint:errcheck // Все ошибки уже обрабатываются внутри

		if delErr := s.kafka.DelRunnerInMsg(ctx, key); delErr != nil {
			return fmt.Errorf("cannot get unfinished function result: %w", delErr)
		}
	}

	return nil
}

func (s *service) rerunUnfinishedTasks(ctx c.Context) error {
	keys, keysErr := s.kafka.GetCachedKeys(ctx, kafka.RunTaskMsgPrefix+"*")
	if keysErr != nil {
		return fmt.Errorf("got error from GetCachedKeys: %w", keysErr)
	}

	for _, fullkey := range keys {
		keyParts := strings.Split(fullkey, ":")
		if len(keyParts) == 1 {
			continue
		}

		key := keyParts[1]

		msg, getErr := s.kafka.GetRunTaskMsg(ctx, key)
		if getErr != nil {
			s.logger.WithError(getErr).Error("cannot get unfinished task")

			continue
		}

		s.apiEnv.RunTaskHandler(ctx, msg) //nolint:errcheck // Все ошибки уже обрабатываются внутри

		if delErr := s.kafka.DelRunTaskMsg(ctx, key); delErr != nil {
			return fmt.Errorf("cannot get unfinished function result: %w", delErr)
		}
	}

	return nil
}
