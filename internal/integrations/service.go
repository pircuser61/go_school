package integrations

import (
	c "context"

	"github.com/google/uuid"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.opencensus.io/plugin/ocgrpc"

	integration_v1 "gitlab.services.mts.ru/jocasta/integrations/pkg/proto/gen/integration/v1"
)

type Service struct {
	C   *grpc.ClientConn
	Cli integration_v1.IntegrationServiceClient
}

func NewService(cfg Config) (*Service, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(&ocgrpc.ClientHandler{})}
	conn, err := grpc.Dial(cfg.URL, opts...)
	if err != nil {
		return nil, err
	}
	client := integration_v1.NewIntegrationServiceClient(conn)

	return &Service{
		C:   conn,
		Cli: client,
	}, nil
}

func (s *Service) GetSystemsNames(ctx c.Context, systemIDs []uuid.UUID) (map[string]string, error) {
	ids := make([]string, 0, len(systemIDs))
	for _, systemID := range systemIDs {
		ids = append(ids, systemID.String())
	}

	res, err := s.Cli.GetIntegrationsNamesByIds(ctx, &integration_v1.GetIntegrationsNamesByIdsRequest{Ids: ids})
	if err != nil {
		return nil, err
	}

	if res != nil {
		return res.Names, nil
	}

	return nil, nil
}
