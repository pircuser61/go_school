package functions

import (
	function_v1 "gitlab.services.mts.ru/jocasta/functions/pkg/proto/gen/function/v1"

	"go.opencensus.io/plugin/ocgrpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Service struct {
	sdURL string

	c   *grpc.ClientConn
	cli function_v1.FunctionServiceClient
}

func NewService(cfg Config) (*Service, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(&ocgrpc.ClientHandler{})}
	conn, err := grpc.Dial(cfg.URL, opts...)
	if err != nil {
		return nil, err
	}
	client := function_v1.NewFunctionServiceClient(conn)

	return &Service{
		c:   conn,
		cli: client,
	}, nil
}
