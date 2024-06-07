package fileregistry

import (
	c "context"
	"net/http"

	"gitlab.services.mts.ru/abp/myosotis/observability"

	"go.opencensus.io/plugin/ochttp"

	"go.opencensus.io/plugin/ocgrpc"
	"google.golang.org/grpc"
	gc "google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"github.com/hashicorp/go-retryablehttp"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"

	fileregistry "gitlab.services.mts.ru/jocasta/file-registry/pkg/proto/gen/file-registry/v1"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/httpclient"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/metrics"
)

type service struct {
	restURL  string
	restCli  *retryablehttp.Client
	grpcConn *grpc.ClientConn
	grpcCLi  fileregistry.FileServiceClient
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
				cnt := ctx.Value(retryCnt{})
				log.WithError(err).WithField("attempt", attempt).WithField("cnt", cnt).
					Error("failed to reconnect to fileregistry")
				i := cnt.(*int)
				*i = *i + 1
			}),
		)))
	}

	conn, err := grpc.NewClient(cfg.GRPC, opts...)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{}

	httpClient.Transport = &transport{
		next: ochttp.Transport{
			Base:        httpClient.Transport,
			Propagation: observability.NewHTTPFormat(),
		},
		metrics: m,
	}

	return &service{
		grpcConn: conn,
		restCli:  httpclient.NewClient(httpClient, log, cfg.MaxRetries, cfg.RetryDelay),
		restURL:  cfg.REST,
		grpcCLi:  fileregistry.NewFileServiceClient(conn),
	}, nil
}

func (s *service) Ping(ctx c.Context) error {
	_, err := s.grpcCLi.PingService(ctx, &fileregistry.PingRequest{})

	return err
}
