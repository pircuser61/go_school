package servicedesc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	cachekit "gitlab.services.mts.ru/jocasta/cache-kit"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
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
	ctx, span := trace.StartSpan(ctx, "servicedesc.get_work_group(cached)")
	defer span.End()

	log := logger.GetLogger(ctx)

	keyForCache := workGroupKeyPrefix + groupID

	valueFromCache, err := s.Cache.GetValue(ctx, keyForCache)
	if err == nil {
		resources, ok := valueFromCache.(*WorkGroup)
		if ok {
			log.Info("got resources from cache")

			return resources, nil
		}

		err = s.Cache.DeleteValue(ctx, keyForCache)
		if err != nil {
			log.WithError(err).Error("can't delete key from cache")
		}
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
	ctx, span := trace.StartSpan(ctx, "servicedesc.get_schema_by_id(cached)")
	defer span.End()

	log := logger.GetLogger(ctx)

	keyForCache := schemaIDKeyPrefix + schemaID

	valueFromCache, err := s.Cache.GetValue(ctx, keyForCache)
	if err == nil {
		schema, ok := valueFromCache.(map[string]interface{})
		if ok {
			log.Info("got schema from cache")

			return schema, nil
		}

		err = s.Cache.DeleteValue(ctx, keyForCache)
		if err != nil {
			log.WithError(err).Error("can't delete key from cache")
		}
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
	ctx, span := trace.StartSpan(ctx, "servicedesc.get_schema_by_blueprint_id(cached)")
	defer span.End()

	log := logger.GetLogger(ctx)

	keyForCache := schemaBlueprintIDKeyPrefix + blueprintID

	valueFromCache, err := s.Cache.GetValue(ctx, keyForCache)
	if err == nil {
		blueprint, ok := valueFromCache.(map[string]interface{})
		if ok {
			log.Info("got blueprint from cache")

			return blueprint, nil
		}

		err = s.Cache.DeleteValue(ctx, keyForCache)
		if err != nil {
			log.WithError(err).Error("can't delete key from cache")
		}
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
	ctxLocal, span := trace.StartSpan(ctx, "servicedesc.get_sso_person(cached)")
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
