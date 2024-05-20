package integrations

import (
	c "context"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"go.opencensus.io/plugin/ocgrpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
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

type Service struct {
	conn       *grpc.ClientConn
	RPCIntCli  integration.IntegrationServiceClient
	RPCMicrCli microservice.MicroserviceServiceClient
	Cli        *retryablehttp.Client
}

func NewService(cfg Config, log logger.Logger, m metrics.Metrics) (*Service, error) {
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
				log.WithError(err).WithField("attempt", attempt).Error("failed to reconnect to integrations")
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

	return &Service{
		conn:       conn,
		RPCIntCli:  integration.NewIntegrationServiceClient(conn),
		RPCMicrCli: microservice.NewMicroserviceServiceClient(conn),
		Cli:        httpclient.NewClient(httpClient, nil, cfg.MaxRetries, cfg.RetryDelay),
	}, nil
}

func (s *Service) GetSystemsNames(ctx c.Context, systemIDs []uuid.UUID) (map[string]string, error) {
	ids := make([]string, 0, len(systemIDs))
	for _, systemID := range systemIDs {
		ids = append(ids, systemID.String())
	}

	res, err := s.RPCIntCli.GetIntegrationsNamesByIds(ctx, &integration.GetIntegrationsNamesByIdsRequest{Ids: ids})
	if err != nil {
		return nil, err
	}

	if res != nil {
		return res.Names, nil
	}

	return nil, errors.New("couldn't get system names")
}

func (s *Service) GetSystemsClients(ctx c.Context, systemIDs []uuid.UUID) (map[string][]string, error) {
	cc := make(map[string][]string)

	for _, id := range systemIDs {
		res, err := s.RPCIntCli.GetIntegrationById(ctx, &integration.GetIntegrationByIdRequest{IntegrationId: id.String()})
		if err != nil {
			return nil, err
		}

		if res != nil && res.Integration != nil {
			cc[id.String()] = res.Integration.ClientIds
		}
	}

	return cc, nil
}

func (s *Service) GetMicroserviceHumanKey(ctx c.Context, microserviceID, pipelineID, versionID, workNumber, clientID string) (string, error) {
	res, err := s.RPCMicrCli.GetMicroservice(ctx, &microservice.GetMicroserviceRequest{
		MicroserviceId: microserviceID,
		PipelineId:     pipelineID,
		VersionId:      versionID,
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
