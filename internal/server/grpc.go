package server

import (
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_validator "github.com/grpc-ecosystem/go-grpc-middleware/validator"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/stats/view"
	"google.golang.org/grpc"
	"net"
)

type GRPCConfig struct {
	Port string
	Conn db.PGConnection
}

type GRPC struct {
	config *GRPCConfig
	s      *grpc.Server
}

func NewGRPC(config *GRPCConfig) *GRPC {
	grpcServer := grpc.NewServer(
		grpc_middleware.WithUnaryServerChain(
			grpc_validator.UnaryServerInterceptor(),
		),
		grpc.StatsHandler(&ocgrpc.ServerHandler{}),
	)

	//s := scenario.NewService(rep.NewScenarioRepository(config.Conn))
	//handler.NewScenarioHandler(s).Mount(grpcServer)

	return &GRPC{
		config: config,
		s:      grpcServer,
	}
}

func (s *GRPC) Listen() error {
	lis, err := net.Listen("tcp", s.config.Port)
	if err != nil {
		return err
	}
	if err := view.Register(ocgrpc.DefaultServerViews...); err != nil {
		return err
	}

	if err := s.s.Serve(lis); err != nil {
		return err
	}
	return nil
}
