package functions

import (
	"context"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"go.opencensus.io/plugin/ocgrpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"

	function_v1 "gitlab.services.mts.ru/jocasta/functions/pkg/proto/gen/function/v1"
)

type Service struct {
	c   *grpc.ClientConn
	cli function_v1.FunctionServiceClient
}

func NewService(cfg Config, log logger.Logger) (*Service, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(&ocgrpc.ClientHandler{}),
	}

	if cfg.MaxRetries != 0 {
		opts = append(opts, grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(
			grpc_retry.WithMax(cfg.MaxRetries),
			grpc_retry.WithBackoff(grpc_retry.BackoffLinear(cfg.RetryDelay)),
			grpc_retry.WithPerRetryTimeout(cfg.Timeout),
			grpc_retry.WithCodes(codes.Unavailable, codes.ResourceExhausted, codes.DataLoss, codes.DeadlineExceeded, codes.Unknown),
			grpc_retry.WithOnRetryCallback(func(ctx context.Context, attempt uint, err error) {
				log.WithError(err).WithField("attempt", attempt).Error("failed to reconnect to functions")
			}),
		)))
	}

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
