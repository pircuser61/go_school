package integrations

import (
	c "context"
	"net/http"

	"github.com/google/uuid"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.opencensus.io/plugin/ocgrpc"

	integration_v1 "gitlab.services.mts.ru/jocasta/integrations/pkg/proto/gen/integration/v1"
	microservice_v1 "gitlab.services.mts.ru/jocasta/integrations/pkg/proto/gen/microservice/v1"
)

type Service struct {
	C          *grpc.ClientConn
	RpcIntCli  integration_v1.IntegrationServiceClient
	RpcMicrCli microservice_v1.MicroserviceServiceClient
	Cli        *http.Client
}

func NewService(cfg Config) (*Service, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(&ocgrpc.ClientHandler{})}
	conn, err := grpc.Dial(cfg.URL, opts...)
	if err != nil {
		return nil, err
	}
	clientInt := integration_v1.NewIntegrationServiceClient(conn)
	clientMic := microservice_v1.NewMicroserviceServiceClient(conn)
	return &Service{
		C:          conn,
		RpcIntCli:  clientInt,
		RpcMicrCli: clientMic,
		Cli:        &http.Client{},
	}, nil
}

func (s *Service) GetSystemsNames(ctx c.Context, systemIDs []uuid.UUID) (map[string]string, error) {
	ids := make([]string, 0, len(systemIDs))
	for _, systemID := range systemIDs {
		ids = append(ids, systemID.String())
	}

	res, err := s.RpcIntCli.GetIntegrationsNamesByIds(ctx, &integration_v1.GetIntegrationsNamesByIdsRequest{Ids: ids})
	if err != nil {
		return nil, err
	}

	if res != nil {
		return res.Names, nil
	}

	return nil, nil
}
