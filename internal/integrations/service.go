package integrations

import (
	c "context"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/trace"

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
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
)

const externalSystemName = "integrations"

type service struct {
	conn       *grpc.ClientConn
	rpcIntCli  integration.IntegrationServiceClient
	rpcMicrCli microservice.MicroserviceServiceClient
	cli        *retryablehttp.Client
	url        string
}

func NewService(cfg Config, m metrics.Metrics, ssoService *sso.Service) (Service, error) {
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

	httpClient := &http.Client{}
	httpClient.Transport = &transport{
		next: ochttp.Transport{
			Base:        httpClient.Transport,
			Propagation: observability.NewHTTPFormat(),
		},
		sso:     ssoService,
		scope:   "",
		metrics: m,
	}

	return &service{
		conn:       conn,
		rpcIntCli:  integration.NewIntegrationServiceClient(conn),
		rpcMicrCli: microservice.NewMicroserviceServiceClient(conn),
		cli:        httpclient.NewClient(httpClient, nil, cfg.MaxRetries, cfg.RetryDelay),
		url:        cfg.URL,
	}, nil
}

func (s *service) Ping(ctx c.Context) error {
	_, err := s.rpcMicrCli.Ping(ctx, &microservice.PingRequest{})

	return err
}

func (s *service) GetRPCIntCli() integration.IntegrationServiceClient {
	return s.rpcIntCli
}

func (s *service) GetRPCMicrCli() microservice.MicroserviceServiceClient {
	return s.rpcMicrCli
}

func (s *service) GetCli() *retryablehttp.Client {
	return s.cli
}

func (s *service) GetSystemsNames(ctx c.Context, systemIDs []uuid.UUID) (map[string]string, error) {
	ctx, span := trace.StartSpan(ctx, "integrations.get_systems_names")
	defer span.End()

	log := logger.GetLogger(ctx).
		WithField("traceID", span.SpanContext().TraceID.String()).
		WithField("transport", "GRPC").
		WithField("integration_name", externalSystemName)

	ctx = logger.WithLogger(ctx, log)
	ctx = script.MakeContextWithRetryCnt(ctx)

	ids := make([]string, 0, len(systemIDs))
	for _, systemID := range systemIDs {
		ids = append(ids, systemID.String())
	}

	res, err := s.rpcIntCli.GetIntegrationsNamesByIds(ctx, &integration.GetIntegrationsNamesByIdsRequest{Ids: ids})
	if err != nil {
		script.LogRetryFailure(ctx, uint(s.cli.RetryMax))

		return nil, err
	}

	script.LogRetrySuccess(ctx)

	if res != nil {
		return res.Names, nil
	}

	return nil, errors.New("couldn't get system names")
}

func (s *service) GetSystemsClients(ctx c.Context, systemIDs []uuid.UUID) (map[string][]string, error) {
	ctx, span := trace.StartSpan(ctx, "integrations.get_systems_clients")
	defer span.End()

	log := logger.GetLogger(ctx).
		WithField("traceID", span.SpanContext().TraceID.String()).
		WithField("transport", "GRPC").
		WithField("integration_name", externalSystemName)

	ctx = logger.WithLogger(ctx, log)

	cc := make(map[string][]string)

	for _, id := range systemIDs {
		retryCtx := script.MakeContextWithRetryCnt(ctx)

		res, err := s.rpcIntCli.GetIntegrationById(retryCtx, &integration.GetIntegrationByIdRequest{IntegrationId: id.String()})
		if err != nil {
			script.LogRetryFailure(retryCtx, uint(s.cli.RetryMax))

			return nil, err
		}

		script.LogRetrySuccess(retryCtx)

		if res != nil && res.Integration != nil {
			cc[id.String()] = res.Integration.ClientIds
		}
	}

	return cc, nil
}

func (s *service) GetMicroserviceHumanKey(ctx c.Context, microSrvID, pID, vID, workNumber, clientID string) (string, error) {
	ctx, span := trace.StartSpan(ctx, "integrations.get_microservice_human_key")
	defer span.End()

	log := logger.GetLogger(ctx).
		WithField("traceID", span.SpanContext().TraceID.String()).
		WithField("transport", "GRPC").
		WithField("integration_name", externalSystemName)

	ctx = logger.WithLogger(ctx, log)
	ctx = script.MakeContextWithRetryCnt(ctx)

	res, err := s.rpcMicrCli.GetMicroservice(ctx, &microservice.GetMicroserviceRequest{
		MicroserviceId: microSrvID,
		PipelineId:     pID,
		VersionId:      vID,
		WorkNumber:     workNumber,
		ClientId:       clientID,
	})
	if err != nil {
		script.LogRetryFailure(ctx, uint(s.cli.RetryMax))

		return "", err
	}

	script.LogRetrySuccess(ctx)

	if res != nil && res.Microservice != nil && res.Microservice.Creds != nil && res.Microservice.Creds.Prod != nil {
		return res.Microservice.Creds.Prod.HumanKey, nil
	}

	return "", errors.New("couldn't get microservice human key")
}
