package functions

import (
	c "context"

	"go.opencensus.io/plugin/ocgrpc"

	"google.golang.org/grpc"
	gc "google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"

	function "gitlab.services.mts.ru/jocasta/functions/pkg/proto/gen/function/v1"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/metrics"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

const externalSystemName = "functions"
const transportGRPC = "GRPC"

type service struct {
	conn          *grpc.ClientConn
	cli           function.FunctionServiceClient
	maxRetryCount uint
}

func NewService(cfg Config, m metrics.Metrics) (Service, error) {
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
			grpc_retry.WithCodes(gc.Unavailable, gc.ResourceExhausted, gc.DataLoss, gc.DeadlineExceeded, gc.Unknown),
			grpc_retry.WithOnRetryCallback(func(ctx c.Context, attempt uint, err error) {
				script.IncreaseReqRetryCntGRPC(ctx)
			}),
		)))
	}

	conn, err := grpc.NewClient(cfg.URL, opts...)
	if err != nil {
		return nil, err
	}

	return &service{
		conn:          conn,
		cli:           function.NewFunctionServiceClient(conn),
		maxRetryCount: cfg.MaxRetries,
	}, nil
}

func (s *service) Ping(ctx c.Context) error {
	_, err := s.cli.PingService(ctx, &function.PingRequest{})

	return err
}
