package sequence

import (
	c "context"

	"go.opencensus.io/plugin/ocgrpc"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"google.golang.org/grpc"
	gc "google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"

	sequence "gitlab.services.mts.ru/jocasta/sequence/pkg/proto/gen/src/sequence/v1"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/metrics"
)

const externalSystemName = "sequence"

type service struct {
	c   *grpc.ClientConn
	log logger.Logger
	cli sequence.SequenceClient
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
			grpc_retry.WithCodes(gc.Unavailable, gc.ResourceExhausted, gc.DataLoss, gc.DeadlineExceeded, gc.Unknown),
			grpc_retry.WithOnRetryCallback(func(ctx c.Context, attempt uint, err error) {
				log.WithError(err).WithField("attempt", attempt).
					Error("failed to reconnect to sequence")
			}),
		)))
	}

	conn, err := grpc.Dial(cfg.URL, opts...)
	if err != nil {
		return nil, err
	}

	client := sequence.NewSequenceClient(conn)

	return &service{
		c:   conn,
		log: log,
		cli: client,
	}, nil
}

func (s *service) Ping(ctx c.Context) error {
	_, err := s.cli.Ping(ctx, &sequence.PingRequest{})

	return err
}
