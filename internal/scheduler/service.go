package scheduler

import (
	"context"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"go.opencensus.io/plugin/ocgrpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"

	scheduler_v1 "gitlab.services.mts.ru/jocasta/scheduler/pkg/proto/gen/src/task/v1"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/metrics"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

const externalSystemName = "scheduler"

type Service struct {
	c   *grpc.ClientConn
	cli scheduler_v1.TaskServiceClient
}

func (s *Service) Ping(ctx context.Context) error {
	_, err := s.cli.Ping(ctx, &scheduler_v1.PingRequest{})

	return err
}

func NewService(cfg Config, _ logger.Logger, m metrics.Metrics) (*Service, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(&ocgrpc.ClientHandler{}),
		grpc.WithUnaryInterceptor(metrics.GrpcMetrics(externalSystemName, m)),
	}

	if cfg.MaxRetries != 0 {
		opts = append(opts, grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(
			grpc_retry.WithMax(cfg.MaxRetries),
			grpc_retry.WithBackoff(grpc_retry.BackoffLinear(cfg.RetryDelay)),
			grpc_retry.WithPerRetryTimeout(cfg.Timeout),
			grpc_retry.WithCodes(codes.Unavailable, codes.ResourceExhausted, codes.DataLoss, codes.DeadlineExceeded, codes.Unknown),
			grpc_retry.WithOnRetryCallback(func(ctx context.Context, attempt uint, err error) {
				script.IncreaseReqRetryCntGRPC(ctx)
			}),
		)))
	}

	conn, err := grpc.NewClient(cfg.URL, opts...)
	if err != nil {
		return nil, err
	}

	client := scheduler_v1.NewTaskServiceClient(conn)

	return &Service{
		c:   conn,
		cli: client,
	}, nil
}
