package servicedesc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
	"go.opencensus.io/trace"

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
	ctx, span := trace.StartSpan(ctx, "servicedesc.get_work_group")
	defer span.End()

	log := logger.CreateLogger(nil)

	keyForCache := workGroupKeyPrefix + groupID

	valueFromCache, err := s.Cache.GetValue(ctx, keyForCache)
	if err == nil {
		resources, ok := valueFromCache.(*WorkGroup)
		if !ok {
			err = s.Cache.DeleteValue(ctx, keyForCache)
			if err != nil {
				log.WithError(err).Error("can't delete key from cache")
			}
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
	ctx, span := trace.StartSpan(ctx, "servicedesc.get_schema_by_id")
	defer span.End()

	log := logger.CreateLogger(nil)

	keyForCache := schemaIDKeyPrefix + schemaID

	valueFromCache, err := s.Cache.GetValue(ctx, keyForCache)
	if err == nil {
		schema, ok := valueFromCache.(map[string]interface{})
		if !ok {
			err = s.Cache.DeleteValue(ctx, keyForCache)
			if err != nil {
				log.WithError(err).Error("can't delete key from cache")
			}
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
	ctx, span := trace.StartSpan(ctx, "servicedesc.get_schema_by_blueprint_id")
	defer span.End()

	log := logger.CreateLogger(nil)

	keyForCache := schemaBlueprintIDKeyPrefix + blueprintID

	valueFromCache, err := s.Cache.GetValue(ctx, keyForCache)
	if err == nil {
		blueprint, ok := valueFromCache.(map[string]interface{})
		if !ok {
			err = s.Cache.DeleteValue(ctx, keyForCache)
			if err != nil {
				log.WithError(err).Error("can't delete key from cache")
			}
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

func (s *ServiceWithCache) GetSsoPerson(ctx context.Context, username string) (*SsoPerson, error) {
	ctxLocal, span := trace.StartSpan(ctx, "servicedesc.get_sso_person")
	defer span.End()

	if sso.IsServiceUserName(username) {
		return &SsoPerson{
			Username: username,
		}, nil
	}

	sdURL := s.GetSdURL()

	reqURL := fmt.Sprintf("%s%s", sdURL, fmt.Sprintf(getUserInfo, username))

	req, err := http.NewRequestWithContext(ctxLocal, http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return nil, err
	}

	cli := s.GetCli()

	resp, err := cli.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status code from sso: %d, username: %s", resp.StatusCode, username)
	}

	res := &SsoPerson{}
	if unmErr := json.NewDecoder(resp.Body).Decode(&res); unmErr != nil {
		return nil, unmErr
	}

	return res, nil
}

func (s *ServiceWithCache) GetSdURL() string {
	return s.Servicedesc.GetSdURL()
}

func (s *ServiceWithCache) GetCli() *http.Client {
	return s.Servicedesc.GetCli()
}
