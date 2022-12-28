package server

import (
	"context"
	"errors"
	"net/http"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/api"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/kafka"
)

type Server struct {
	logger logger.Logger

	httpServer *http.Server

	kafka *kafka.Service
}

func NewServer(
	ctx context.Context,
	log logger.Logger,
	kf *kafka.Service,
	serverParam api.ServerParam,
) *Server {
	httpServer, err := api.NewServer(ctx, serverParam)
	if err != nil {
		log.Fatal(err)
	}

	return &Server{
		logger:     log,
		httpServer: httpServer,
		kafka:      kf,
	}
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

	//s.kafka.StartConsumer(ctx)
}

func (s *Server) Stop(ctx context.Context) {
	if err := s.httpServer.Shutdown(ctx); err != nil {
		s.logger.WithError(err).Error("error on http server shutdown")
	}

	if err := s.kafka.CloseProducer(); err != nil {
		s.logger.WithError(err).Error("error on producer shutdown")
	}
}
