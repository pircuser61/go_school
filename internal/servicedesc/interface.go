package servicedesc

import (
	"context"

	"github.com/hashicorp/go-retryablehttp"
)

type ServiceInterface interface {
	GetWorkGroup(ctx context.Context, groupID string) (*WorkGroup, error)
	GetSsoPerson(ctx context.Context, username string) (*SsoPerson, error)
	GetSchemaByID(ctx context.Context, schemaID string) (map[string]interface{}, error)
	GetSchemaByBlueprintID(ctx context.Context, blueprintID string) (map[string]interface{}, error)
	GetSdURL() string
	GetCli() *retryablehttp.Client
}
