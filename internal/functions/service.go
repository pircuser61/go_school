package functions

import (
	c "context"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"go.opencensus.io/plugin/ocgrpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"

	function "gitlab.services.mts.ru/jocasta/functions/pkg/proto/gen/function/v1"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/metrics"
)

const externalSystemName = "functions"

type service struct {
	conn *grpc.ClientConn
	cli  function.FunctionServiceClient
}

func NewService(cfg Config, log logger.Logger, m metrics.Metrics) (Service, error) {
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
			grpc_retry.WithOnRetryCallback(func(ctx c.Context, attempt uint, err error) {
				log.WithError(err).WithField("attempt", attempt).Error("failed to reconnect to functions")
			}),
		)))
	}

	conn, err := grpc.Dial(cfg.URL, opts...)
	if err != nil {
		return nil, err
	}

	return &service{
		conn: conn,
		cli:  function.NewFunctionServiceClient(conn),
	}, nil
}

func (s *service) Ping(ctx c.Context) error {
	return nil
}
