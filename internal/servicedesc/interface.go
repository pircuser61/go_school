package servicedesc

import (
	c "context"

	"github.com/hashicorp/go-retryablehttp"
)

type Service interface {
	GetWorkGroup(ctx c.Context, groupID string) (*WorkGroup, error)
	GetSsoPerson(ctx c.Context, username string) (*SsoPerson, error)
	GetSchemaByID(ctx c.Context, schemaID string) (map[string]interface{}, error)
	GetSchemaByBlueprintID(ctx c.Context, blueprintID string) (map[string]interface{}, error)
	GetSdURL() string
	GetCli() *retryablehttp.Client
}
