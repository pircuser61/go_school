package server

import (
	"context"
	"net/http"

	"google.golang.org/grpc"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"

	pb "gitlab.services.mts.ru/jocasta/proto/gen/proto/go/scenario/v1"
)

type GRPCGWConfig struct {
	GRPCPort   string
	GRPCGWPort string
}

type GRPCGW struct {
	config *GRPCGWConfig
}

func ListenGRPCGW(config *GRPCGWConfig) error {

	conn, err := grpc.Dial(config.GRPCPort, grpc.WithInsecure())
	if err != nil {
		return err
	}
	mux := runtime.NewServeMux()
	ctx := context.TODO()
	if err := pb.RegisterScenarioServiceHandler(ctx, mux, conn); err != nil {
		return err
	}
	h := http.Handler(mux)
	err = http.ListenAndServe(config.GRPCGWPort, h)
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
