package servicedesc

import (
	"context"
	"fmt"
	"net/http"

	cachekit "gitlab.services.mts.ru/jocasta/cache-kit"
)

const (
	workGroupKeyPrefix         = "workGroup:"
	schemaIDKeyPrefix          = "schemaID:"
	schemaBlueprintIDKeyPrefix = "schemaBlueprintID:"
)

type ServiceWithCache struct {
	Cache       cachekit.Cache
	Servicedesc ServiceInterface
}

func (s *ServiceWithCache) GetWorkGroup(ctx context.Context, groupID string) (*WorkGroup, error) {
	keyForCache := workGroupKeyPrefix + groupID

	valueFromCache, err := s.Cache.GetValue(ctx, keyForCache)
	if err == nil {
		resources, ok := valueFromCache.(*WorkGroup)
		if !ok {
			return nil, fmt.Errorf("failed to cast value from cache to type *WorkGroup")
		}

		return resources, nil
	}

	workGroup, err := s.Servicedesc.GetWorkGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}

	err = s.Cache.SetValue(ctx, keyForCache, workGroup)
	if err != nil {
		return nil, fmt.Errorf("can't set resources to cache: %s", err)
	}

	return workGroup, nil
}

func (s *ServiceWithCache) GetSchemaByID(ctx context.Context, schemaID string) (map[string]interface{}, error) {
	keyForCache := schemaIDKeyPrefix + schemaID

	valueFromCache, err := s.Cache.GetValue(ctx, keyForCache)
	if err == nil {
		schema, ok := valueFromCache.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("failed to cast value from cache to type schema")
		}

		return schema, nil
	}

	schema, err := s.Servicedesc.GetSchemaByID(ctx, schemaID)
	if err != nil {
		return nil, err
	}

	err = s.Cache.SetValue(ctx, keyForCache, schema)
	if err != nil {
		return nil, fmt.Errorf("can't set resources to cache: %s", err)
	}

	return schema, nil
}

func (s *ServiceWithCache) GetSchemaByBlueprintID(ctx context.Context, blueprintID string) (map[string]interface{}, error) {
	keyForCache := schemaBlueprintIDKeyPrefix + blueprintID

	valueFromCache, err := s.Cache.GetValue(ctx, keyForCache)
	if err == nil {
		blueprint, ok := valueFromCache.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("failed to cast value from cache to type blueprint")
		}

		return blueprint, nil
	}

	blueprint, err := s.Servicedesc.GetSchemaByBlueprintID(ctx, blueprintID)
	if err != nil {
		return nil, err
	}

	err = s.Cache.SetValue(ctx, keyForCache, blueprint)
	if err != nil {
		return nil, fmt.Errorf("can't set resources to cache: %s", err)
	}

	return blueprint, nil
}

// TODO добавить кеш
func (s *ServiceWithCache) GetSsoPerson(ctx context.Context, username string) (*SsoPerson, error) {
	return s.Servicedesc.GetSsoPerson(ctx, username)
}

func (s *ServiceWithCache) GetSdURL() string {
	return s.Servicedesc.GetSdURL()
}

func (s *ServiceWithCache) GetCli() *http.Client {
	return s.Servicedesc.GetCli()
}
