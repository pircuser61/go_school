package cache

import (
	c "context"
	"encoding/json"
	"strings"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/humantasks/nocache"
	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	cachekit "gitlab.services.mts.ru/jocasta/cache-kit"

	d "gitlab.services.mts.ru/jocasta/human-tasks/pkg/proto/gen/proto/go/delegation"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/humantasks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/metrics"
)

const (
	delegationsKeyPrefix = "delegations:"
)

type Service struct {
	Cache      cachekit.Cache
	Humantasks humantasks.Service
}

func NewService(cfg *humantasks.Config, m metrics.Metrics) (humantasks.Service, error) {
	srv, err := nocache.NewService(cfg, m)
	if err != nil {
		return nil, err
	}

	cache, cacheErr := cachekit.CreateCache(cachekit.Config(cfg.Cache))
	if cacheErr != nil {
		return nil, cacheErr
	}

	return &Service{
		Cache:      cache,
		Humantasks: srv,
	}, nil
}

func (*Service) SetCli(d.DelegationServiceClient) {}

func (s *Service) Ping(ctx c.Context) error {
	return s.Humantasks.Ping(ctx)
}

func (s *Service) GetDelegations(ctx c.Context, req *d.GetDelegationsRequest) (humantasks.Delegations, error) {
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
				var data humantasks.Delegations

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

func (s *Service) GetDelegationsFromLogin(ctx c.Context, login string) (humantasks.Delegations, error) {
	ctx, span := trace.StartSpan(ctx, "humantasks.get_delegations_from_login(cached)")
	defer span.End()

	req := &d.GetDelegationsRequest{
		FilterBy:  nocache.FromLoginFilter,
		FromLogin: login,
	}

	res, err := s.GetDelegations(ctx, req)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (s *Service) GetDelegationsToLogin(ctx c.Context, login string) (humantasks.Delegations, error) {
	ctx, span := trace.StartSpan(ctx, "humantasks.get_delegations_to_login(cached)")
	defer span.End()

	req := &d.GetDelegationsRequest{
		FilterBy: nocache.ToLoginFilter,
		ToLogin:  login,
	}

	res, err := s.GetDelegations(ctx, req)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (s *Service) GetDelegationsToLogins(ctx c.Context, logins []string) (humantasks.Delegations, error) {
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
		FilterBy: nocache.ToLoginsFilter,
		ToLogins: sb.String(),
	}

	res, err := s.GetDelegations(ctx, req)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (s *Service) GetDelegationsByLogins(ctx c.Context, logins []string) (humantasks.Delegations, error) {
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
		FilterBy:   nocache.FromLoginsFilter,
		FromLogins: sb.String(),
	}

	res, err := s.GetDelegations(ctx, req)
	if err != nil {
		return nil, err
	}

	return res, nil
}
