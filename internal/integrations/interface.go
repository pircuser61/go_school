package integrations

import (
	c "context"

	"github.com/hashicorp/go-retryablehttp"

	integration "gitlab.services.mts.ru/jocasta/integrations/pkg/proto/gen/integration/v1"

	microservice "gitlab.services.mts.ru/jocasta/integrations/pkg/proto/gen/microservice/v1"

	"github.com/google/uuid"
)

type Service interface {
	Getter

	GetSystemsNames(ctx c.Context, systemIDs []uuid.UUID) (map[string]string, error)
	GetSystemsClients(ctx c.Context, systemIDs []uuid.UUID) (map[string][]string, error)
	GetMicroserviceHumanKey(ctx c.Context, microSrvID, pID, vID, workNumber, clientID string) (string, error)
	GetToken(ctx c.Context, scopes []string, clientSecret, clientID, stand string) (token string, err error)
	FillAuth(ctx c.Context, key, pID, vID, wNumber, clientID string) (res *Auth, err error)
	Ping(ctx c.Context) error
}

type Getter interface {
	GetCli() *retryablehttp.Client
	GetRpcIntCli() integration.IntegrationServiceClient
	GetRpcMicrCli() microservice.MicroserviceServiceClient
}
