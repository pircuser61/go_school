package sequence

import (
	"context"

	"go.opencensus.io/plugin/ocgrpc"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"

	sequence "gitlab.services.mts.ru/jocasta/sequence/pkg/proto/gen/src/sequence/v1"
)

type Service struct {
	c   *grpc.ClientConn
	log logger.Logger
	cli sequence.SequenceClient
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
				log.WithError(err).WithField("attempt", attempt).Error("failed to reconnect to sequence")
			}),
		)))
	}

	conn, err := grpc.Dial(cfg.URL, opts...)
	if err != nil {
		return nil, err
	}

	client := sequence.NewSequenceClient(conn)

	return &Service{
		c:   conn,
		log: log,
		cli: client,
	}, nil
}
