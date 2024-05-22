package fileregistry

import (
	c "context"
	"net/http"

	"gitlab.services.mts.ru/abp/myosotis/observability"

	"go.opencensus.io/plugin/ochttp"

	"go.opencensus.io/plugin/ocgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
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
			grpc_retry.WithCodes(codes.Unavailable, codes.ResourceExhausted, codes.DataLoss, codes.DeadlineExceeded, codes.Unknown),
			grpc_retry.WithOnRetryCallback(func(ctx c.Context, attempt uint, err error) {
				log.WithError(err).WithField("attempt", attempt).Error("failed to reconnect to fileregistry")
			}),
		)))
	}

	conn, err := grpc.Dial(cfg.GRPC, opts...)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{}

	tr := transport{
		next: ochttp.Transport{
			Base:        httpClient.Transport,
			Propagation: observability.NewHTTPFormat(),
		},
		metrics: m,
	}

	httpClient.Transport = &tr

	return &service{
		grpcConn: conn,
		restCli:  httpclient.NewClient(httpClient, log, cfg.MaxRetries, cfg.RetryDelay),
		restURL:  cfg.REST,
		grpcCLi:  fileregistry.NewFileServiceClient(conn),
	}, nil
}

func (s *service) Ping(ctx c.Context) error {
	req, err := retryablehttp.NewRequest("HEAD", s.restURL, nil)
	if err != nil {
		return err
	}

	resp, err := s.restCli.Do(req)
	if err != nil {
		return err
	}

	return resp.Body.Close()
}