package cache

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/go-retryablehttp"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	sd "gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc"
)

const (
	workGroupKeyPrefix         = "workGroup:"
	schemaIDKeyPrefix          = "schemaID:"
	schemaBlueprintIDKeyPrefix = "schemaBlueprintID:"
	ssoPersonKeyPrefix         = "ssoPerson:"
)

//nolint:dupl //так нужно
func (s *service) GetWorkGroup(ctx context.Context, groupID string) (*sd.WorkGroup, error) {
	ctx, span := trace.StartSpan(ctx, "servicedesc.get_work_group(cached)")
	defer span.End()

	log := logger.GetLogger(ctx)

	keyForCache := workGroupKeyPrefix + groupID

	valueFromCache, err := s.cache.GetValue(ctx, keyForCache)
	if err == nil {
		resources, ok := valueFromCache.(string)
		if ok {
			var data *sd.WorkGroup

			unmErr := json.Unmarshal([]byte(resources), &data)
			if unmErr == nil {
				log.Info("got groupResources from cache")

				return data, nil
			}
		}

		err = s.cache.DeleteValue(ctx, keyForCache)
		if err != nil {
			log.WithError(err).Error("can't delete key from cache")
		}
	}

	workGroup, err := s.servicedesc.GetWorkGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}

	workGroupData, err := json.Marshal(workGroup)
	if err == nil && keyForCache != "" {
		err = s.cache.SetValue(ctx, keyForCache, string(workGroupData))
		if err != nil {
			return nil, fmt.Errorf("can't set resources to cache: %s", err)
		}
	}

	return workGroup, nil
}

//nolint:dupl //так нужно
func (s *service) GetSchemaByID(ctx context.Context, schemaID string) (map[string]interface{}, error) {
	ctx, span := trace.StartSpan(ctx, "servicedesc.get_schema_by_id(cached)")
	defer span.End()

	log := logger.GetLogger(ctx)

	keyForCache := schemaIDKeyPrefix + schemaID

	valueFromCache, err := s.cache.GetValue(ctx, keyForCache)
	if err == nil {
		schema, ok := valueFromCache.(string)
		if ok {
			var data map[string]interface{}

			unmErr := json.Unmarshal([]byte(schema), &data)
			if unmErr == nil {
				log.Info("got schema from cache")

				return data, nil
			}
		}

		err = s.cache.DeleteValue(ctx, keyForCache)
		if err != nil {
			log.WithError(err).Error("can't delete key from cache")
		}
	}

	schema, err := s.servicedesc.GetSchemaByID(ctx, schemaID)
	if err != nil {
		return nil, err
	}

	schemaData, err := json.Marshal(schema)
	if err == nil && keyForCache != "" {
		err = s.cache.SetValue(ctx, keyForCache, string(schemaData))
		if err != nil {
			return nil, fmt.Errorf("can't set resources to cache: %s", err)
		}
	}

	return schema, nil
}

//nolint:dupl //так нужно
func (s *service) GetSchemaByBlueprintID(ctx context.Context, blueprintID string) (map[string]interface{}, error) {
	ctx, span := trace.StartSpan(ctx, "servicedesc.get_schema_by_blueprint_id(cached)")
	defer span.End()

	log := logger.GetLogger(ctx)

	keyForCache := schemaBlueprintIDKeyPrefix + blueprintID

	valueFromCache, err := s.cache.GetValue(ctx, keyForCache)
	if err == nil {
		blueprint, ok := valueFromCache.(string)
		if ok {
			var data map[string]interface{}

			unmErr := json.Unmarshal([]byte(blueprint), &data)
			if unmErr == nil {
				log.Info("got blueprint from cache")

				return data, nil
			}
		}

		err = s.cache.DeleteValue(ctx, keyForCache)
		if err != nil {
			log.WithError(err).Error("can't delete key from cache")
		}
	}

	blueprint, err := s.servicedesc.GetSchemaByBlueprintID(ctx, blueprintID)
	if err != nil {
		return nil, err
	}

	blueprintData, err := json.Marshal(blueprint)
	if err == nil && keyForCache != "" {
		err = s.cache.SetValue(ctx, keyForCache, string(blueprintData))
		if err != nil {
			return nil, fmt.Errorf("can't set resources to cache: %s", err)
		}
	}

	return blueprint, nil
}

func (s *service) GetSsoPerson(ctx context.Context, username string) (*sd.SsoPerson, error) {
	ctx, span := trace.StartSpan(ctx, "servicedesc.get_sso_person(cached)")
	defer span.End()

	log := logger.GetLogger(ctx)

	keyForCache := ssoPersonKeyPrefix + username

	valueFromCache, err := s.cache.GetValue(ctx, keyForCache)
	if err == nil {
		person, ok := valueFromCache.(string)
		if ok {
			var data *sd.SsoPerson

			unmErr := json.Unmarshal([]byte(person), &data)
			if unmErr == nil {
				log.Info("got res from cache")

				return data, nil
			}
		}

		err = s.cache.DeleteValue(ctx, keyForCache)
		if err != nil {
			log.WithError(err).Error("can't delete key from cache")
		}
	}

	person, err := s.servicedesc.GetSsoPerson(ctx, username)
	if err != nil {
		return nil, err
	}

	personData, err := json.Marshal(person)
	if err == nil && keyForCache != "" {
		err = s.cache.SetValue(ctx, keyForCache, string(personData))
		if err != nil {
			return nil, fmt.Errorf("can't set res to cache: %s", err)
		}
	}

	return person, nil
}

func (s *service) GetSdURL() string {
	return s.servicedesc.GetSdURL()
}

func (s *service) GetCli() *retryablehttp.Client {
	return s.servicedesc.GetCli()
}
