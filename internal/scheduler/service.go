package scheduler

import (
	scheduler_v1 "gitlab.services.mts.ru/jocasta/scheduler/pkg/proto/gen/src/task/v1"

	"go.opencensus.io/plugin/ocgrpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Service struct {
	c   *grpc.ClientConn
	cli scheduler_v1.TaskServiceClient
}

func NewService(cfg Config) (*Service, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(&ocgrpc.ClientHandler{})}
	conn, err := grpc.Dial(cfg.URL, opts...)
	if err != nil {
		return nil, err
	}
	client := scheduler_v1.NewTaskServiceClient(conn)

	return &Service{
		c:   conn,
		cli: client,
	}, nil
}
