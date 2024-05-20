package humantasks

import (
	c "context"
	"encoding/json"
	"strings"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	cachekit "gitlab.services.mts.ru/jocasta/cache-kit"

	d "gitlab.services.mts.ru/jocasta/human-tasks/pkg/proto/gen/proto/go/delegation"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/metrics"
)

const (
	delegationsKeyPrefix = "delegations:"
)

type ServiceWithCache struct {
	Cache      cachekit.Cache
	Humantasks ServiceInterface
}

func NewServiceWithCache(cfg *Config, log logger.Logger, m metrics.Metrics) (ServiceInterface, error) {
	srv, err := NewService(cfg, log, m)
	if err != nil {
		return nil, err
	}

	cache, cacheErr := cachekit.CreateCache(cachekit.Config(cfg.Cache))
	if cacheErr != nil {
		return nil, cacheErr
	}

	return &ServiceWithCache{
		Cache:      cache,
		Humantasks: srv,
	}, nil
}

func (s *ServiceWithCache) SetCli(cli d.DelegationServiceClient) {}

func (s *ServiceWithCache) GetDelegations(ctx c.Context, req *d.GetDelegationsRequest) (Delegations, error) {
	ctx, span := trace.StartSpan(ctx, "humantasks.get_delegations(cached)")
	defer span.End()

	log := logger.GetLogger(ctx)

	var keyForCache string

	key, err := json.Marshal(req)
	if err == nil { //nolint:nestif //так нужно
		keyForCache = delegationsKeyPrefix + string(key)

		valueFromCache, err := s.Cache.GetValue(ctx, keyForCache) //nolint:govet //ничего страшного
		if err == nil {
			delegations, ok := valueFromCache.(string)
			if ok {
				var data Delegations

				unmErr := json.Unmarshal([]byte(delegations), &data)
				if unmErr == nil {
					log.Info("got delegations from cache")

					return data, nil
				}
			}

			err = s.Cache.DeleteValue(ctx, keyForCache)
			if err != nil {
				log.WithError(err).Error("can't delete key from cache")
			}
		}
	}

	delegations, err := s.Humantasks.GetDelegations(ctx, req)
	if err != nil {
		return nil, err
	}

	delegationsData, err := json.Marshal(delegations)
	if err == nil && keyForCache != "" {
		err = s.Cache.SetValue(ctx, keyForCache, string(delegationsData))
		if err != nil {
			log.WithError(err).Error("can't send data to cache")
		}
	}

	return delegations, nil
}

func (s *ServiceWithCache) GetDelegationsFromLogin(ctx c.Context, login string) (Delegations, error) {
	ctx, span := trace.StartSpan(ctx, "humantasks.get_delegations_from_login(cached)")
	defer span.End()

	req := &d.GetDelegationsRequest{
		FilterBy:  FromLoginFilter,
		FromLogin: login,
	}

	res, err := s.GetDelegations(ctx, req)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (s *ServiceWithCache) GetDelegationsToLogin(ctx c.Context, login string) (Delegations, error) {
	ctx, span := trace.StartSpan(ctx, "humantasks.get_delegations_to_login(cached)")
	defer span.End()

	req := &d.GetDelegationsRequest{
		FilterBy: ToLoginFilter,
		ToLogin:  login,
	}

	res, err := s.GetDelegations(ctx, req)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (s *ServiceWithCache) GetDelegationsToLogins(ctx c.Context, logins []string) (Delegations, error) {
	ctx, span := trace.StartSpan(ctx, "humantasks.get_delegations_to_logins(cached)")
	defer span.End()

	var sb strings.Builder

	for i, login := range logins {
		sb.WriteString(login)

		if i < len(logins)-1 {
			sb.WriteString(",")
		}
	}

	req := &d.GetDelegationsRequest{
		FilterBy: ToLoginsFilter,
		ToLogins: sb.String(),
	}

	res, err := s.GetDelegations(ctx, req)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (s *ServiceWithCache) GetDelegationsByLogins(ctx c.Context, logins []string) (Delegations, error) {
	ctx, span := trace.StartSpan(ctx, "humantasks.get_delegations_by_logins(cached)")
	defer span.End()

	var sb strings.Builder

	for i, login := range logins {
		sb.WriteString(login)

		if i < len(logins)-1 {
			sb.WriteString(",")
		}
	}

	req := &d.GetDelegationsRequest{
		FilterBy:   FromLoginsFilter,
		FromLogins: sb.String(),
	}

	res, err := s.GetDelegations(ctx, req)
	if err != nil {
		return nil, err
	}

	return res, nil
}
