package sequence

import (
	"go.opencensus.io/plugin/ocgrpc"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	sequence "gitlab.services.mts.ru/jocasta/sequence/pkg/proto/gen/src/sequence/v1"
)

type Service struct {
	c   *grpc.ClientConn
	log logger.Logger
	cli sequence.SequenceClient
}

func NewService(cfg Config) (*Service, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(&ocgrpc.ClientHandler{}),
	}

	conn, err := grpc.NewClient(cfg.URL, opts...)
	if err != nil {
		return nil, err
	}

	client := sequence.NewSequenceClient(conn)

	return &Service{
		c:   conn,
		cli: client,
	}, nil
}
