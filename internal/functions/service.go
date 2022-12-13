package functions

import (
	"crypto/tls"

	function_v1 "gitlab.services.mts.ru/jocasta/functions/pkg/proto/gen/function/v1"

	"go.opencensus.io/plugin/ocgrpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type Service struct {
	sdURL string

	c   *grpc.ClientConn
	cli function_v1.FunctionServiceClient
}

func NewService(cfg Config) (*Service, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})),
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
