package server

import (
	"context"
	"errors"
	"net/http"
	"os"
	"time"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/api"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/kafka"
)

type Server struct {
	logger logger.Logger

	httpServer   *http.Server
	grpcServer   *GRPC
	grpcGWServer *GRPCGW

	kafka *kafka.Service
}

func NewServer(
	ctx context.Context,
	log logger.Logger,
	kf *kafka.Service,
	serverParam api.ServerParam,
	grpcServerParam *GRPCConfig,
	grpcGWServerParam *GRPCGWConfig,
) *Server {
	httpServer, err := api.NewServer(ctx, serverParam)
	if err != nil {
		log.Fatal(err)
	}

	grpcServer := NewGRPC(grpcServerParam)
	grpcGWServer := NewGRPCGW(grpcGWServerParam)

	return &Server{
		logger:       log,
		httpServer:   httpServer,
		grpcServer:   grpcServer,
		grpcGWServer: grpcGWServer,
		kafka:        kf,
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

	go func() {
		if err := s.grpcServer.Listen(); err != nil {
			os.Exit(-2)
		}
	}()

	go func() {
		time.Sleep(time.Second)
		if err := s.grpcGWServer.ListenGRPCGW(); err != nil {
			os.Exit(-3)
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
