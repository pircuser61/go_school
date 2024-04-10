package server

import (
	"context"
	"errors"
	"net/http"
	"time"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/api"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/kafka"
)

type Server struct {
	logger logger.Logger

	httpServer *http.Server

	kafka *kafka.Service
	svcs  *api.Env

	SvcsPingTimer   time.Duration
	SvcsFailedCount int
	SvcsOkCount     int
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
	}

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

	s.kafka.StartConsumer(ctx)
}

func (s *Server) Stop(ctx context.Context) {
	if err := s.httpServer.Shutdown(ctx); err != nil {
		s.logger.WithError(err).Error("error on http server shutdown")
	}

	if err := s.kafka.CloseProducer(); err != nil {
		s.logger.WithError(err).Error("error on producer shutdown")
	}
}

func (s *Server) StartKafkaWorkers(ctx context.Context, message kafka.RunnerInMessage) error {
	messageCh := make(chan kafka.RunnerInMessage)

	for i := 0; i < 10; i++ {
		go s.svcs.WorkFunctionHandler(ctx, messageCh)
	}

	messageCh <- message

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
		}

		time.Sleep(s.SvcsPingTimer)
	}
}

func (s *Server) PingSvcs(ctx context.Context, failedCh chan bool) {
	var kafkaStopped bool
	var failedCount int
	var okCount int

	for {
		time.Sleep(s.SvcsPingTimer)

		err := s.svcs.DB.Ping(ctx)
		err = s.svcs.Scheduler.Ping(ctx)

		if err != nil {
			if failedCount < s.SvcsFailedCount {
				failedCount++
			}
			okCount = 0

			if kafkaStopped {
				continue
			}

			kafkaStopped = true

			failedCh <- true

			continue
		}

		okCount++
		if okCount < s.SvcsOkCount {
			continue
		}

		failedCount = 0
		kafkaStopped = false

		failedCh <- false
	}
}
