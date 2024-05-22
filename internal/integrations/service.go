package integrations

import (
	c "context"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"go.opencensus.io/plugin/ocgrpc"

	"google.golang.org/grpc"
	gc "google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"github.com/hashicorp/go-retryablehttp"

	"gitlab.services.mts.ru/abp/myosotis/observability"

	"go.opencensus.io/plugin/ochttp"

	integration "gitlab.services.mts.ru/jocasta/integrations/pkg/proto/gen/integration/v1"
	microservice "gitlab.services.mts.ru/jocasta/integrations/pkg/proto/gen/microservice/v1"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/httpclient"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/metrics"
)

const externalSystemName = "integrations"

type service struct {
	conn       *grpc.ClientConn
	rpcIntCli  integration.IntegrationServiceClient
	rpcMicrCli microservice.MicroserviceServiceClient
	cli        *retryablehttp.Client
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
					Error("failed to reconnect to integrations")
			}),
		)))
	}

	conn, err := grpc.Dial(cfg.URL, opts...)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{}
	httpClient.Transport = &transport{
		next: ochttp.Transport{
			Base:        httpClient.Transport,
			Propagation: observability.NewHTTPFormat(),
		},
		scope:   "",
		metrics: m,
	}

	return &service{
		conn:       conn,
		rpcIntCli:  integration.NewIntegrationServiceClient(conn),
		rpcMicrCli: microservice.NewMicroserviceServiceClient(conn),
		cli:        httpclient.NewClient(httpClient, nil, cfg.MaxRetries, cfg.RetryDelay),
	}, nil
}

func (s *service) Ping(ctx c.Context) error {
	return nil
}

func (s *service) GetRpcIntCli() integration.IntegrationServiceClient {
	return s.rpcIntCli
}

func (s *service) GetRpcMicrCli() microservice.MicroserviceServiceClient {
	return s.rpcMicrCli
}

func (s *service) GetCli() *retryablehttp.Client {
	return s.cli
}

func (s *service) GetSystemsNames(ctx c.Context, systemIDs []uuid.UUID) (map[string]string, error) {
	ids := make([]string, 0, len(systemIDs))
	for _, systemID := range systemIDs {
		ids = append(ids, systemID.String())
	}

	res, err := s.rpcIntCli.GetIntegrationsNamesByIds(ctx, &integration.GetIntegrationsNamesByIdsRequest{Ids: ids})
	if err != nil {
		return nil, err
	}

	if res != nil {
		return res.Names, nil
	}

	return nil, errors.New("couldn't get system names")
}

func (s *service) GetSystemsClients(ctx c.Context, systemIDs []uuid.UUID) (map[string][]string, error) {
	cc := make(map[string][]string)

	for _, id := range systemIDs {
		res, err := s.rpcIntCli.GetIntegrationById(ctx, &integration.GetIntegrationByIdRequest{IntegrationId: id.String()})
		if err != nil {
			return nil, err
		}

		if res != nil && res.Integration != nil {
			cc[id.String()] = res.Integration.ClientIds
		}
	}

	return cc, nil
}

func (s *service) GetMicroserviceHumanKey(ctx c.Context, microSrvID, pID, vID, workNumber, clientID string) (string, error) {
	res, err := s.rpcMicrCli.GetMicroservice(ctx, &microservice.GetMicroserviceRequest{
		MicroserviceId: microSrvID,
		PipelineId:     pID,
		VersionId:      vID,
		WorkNumber:     workNumber,
		ClientId:       clientID,
	})
	if err != nil {
		return "", err
	}

	if res != nil && res.Microservice != nil && res.Microservice.Creds != nil && res.Microservice.Creds.Prod != nil {
		return res.Microservice.Creds.Prod.HumanKey, nil
	}

	return "", errors.New("couldn't get microservice human key")
}
