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

	integration_v1 "gitlab.services.mts.ru/jocasta/integrations/pkg/proto/gen/integration/v1"
	microservice_v1 "gitlab.services.mts.ru/jocasta/integrations/pkg/proto/gen/microservice/v1"
)

type Service struct {
	C          *grpc.ClientConn
	RPCIntCli  integration_v1.IntegrationServiceClient
	RPCMicrCli microservice_v1.MicroserviceServiceClient
	Cli        *http.Client
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
			grpc_retry.WithOnRetryCallback(func(ctx c.Context, attempt uint, err error) {
				log.WithError(err).WithField("attempt", attempt).Error("failed to reconnect to integrations")
			}),
		)))
	}

	conn, err := grpc.Dial(cfg.URL, opts...)
	if err != nil {
		return nil, err
	}

	clientInt := integration_v1.NewIntegrationServiceClient(conn)
	clientMic := microservice_v1.NewMicroserviceServiceClient(conn)

	return &Service{
		C:          conn,
		RPCIntCli:  clientInt,
		RPCMicrCli: clientMic,
		Cli:        &http.Client{},
	}, nil
}

func (s *Service) GetSystemsNames(ctx c.Context, systemIDs []uuid.UUID) (map[string]string, error) {
	ids := make([]string, 0, len(systemIDs))
	for _, systemID := range systemIDs {
		ids = append(ids, systemID.String())
	}

	res, err := s.RPCIntCli.GetIntegrationsNamesByIds(ctx, &integration_v1.GetIntegrationsNamesByIdsRequest{Ids: ids})
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
		res, err := s.RPCIntCli.GetIntegrationById(ctx, &integration_v1.GetIntegrationByIdRequest{IntegrationId: id.String()})
		if err != nil {
			return nil, err
		}

		if res != nil && res.Integration != nil {
			cc[id.String()] = res.Integration.ClientIds
		}
	}

	return cc, nil
}

func (s *Service) GetMicroserviceHumanKey(
	ctx c.Context, microserviceID, pipelineID, versionID, workNumber, clientID string,
) (string, error) {
	res, err := s.RPCMicrCli.GetMicroservice(ctx, &microservice_v1.GetMicroserviceRequest{
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
