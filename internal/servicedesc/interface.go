package servicedesc

import (
	c "context"

	"github.com/hashicorp/go-retryablehttp"
)

type Service interface {
	Setter

	GetWorkGroup(ctx c.Context, groupID string) (*WorkGroup, error)
	GetSchemaByID(ctx c.Context, schemaID string) (map[string]interface{}, error)
	GetSchemaByBlueprintID(ctx c.Context, blueprintID string) (map[string]interface{}, error)
	GetSdURL() string
	Ping(ctx c.Context) error
}

type Setter interface {
	SetCli(cli *retryablehttp.Client)
	GetCli() *retryablehttp.Client
}
