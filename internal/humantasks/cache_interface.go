package humantasks

import (
	c "context"
	"encoding/json"
	"fmt"
	"gitlab.services.mts.ru/abp/myosotis/logger"
	d "gitlab.services.mts.ru/jocasta/human-tasks/pkg/proto/gen/proto/go/delegation"
)

func (s *ServiceWithCache) getDelegationsInternal(ctx c.Context, req *d.GetDelegationsRequest) (ds Delegations, err error) {
	log := logger.CreateLogger(nil)

	key, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %s", err)
	}

	keyForCache := "delegations" + ":" + string(key)

	valueFromCache, err := s.Cache.GetValue(ctx, keyForCache)
	if err == nil {
		ds, ok := valueFromCache.(Delegations)
		if !ok {
			err = s.Cache.DeleteValue(ctx, keyForCache)
			if err != nil {
				log.WithError(err).Error("can't delete key from cache")
			}
		}

		return ds, nil
	}

	ds, err = s.Humantasks.getDelegationsInternal(ctx, req)
	if err != nil {
		return nil, err
	}

	err = s.Cache.SetValue(ctx, keyForCache, ds)
	if err != nil {
		log.WithError(err).Error("can't send data to cache")
	}

	return ds, nil
}

func (s *ServiceWithCache) GetDelegationsFromLogin(ctx c.Context, login string) (ds Delegations, err error) {
	return s.Humantasks.GetDelegationsFromLogin(ctx, login)
}

func (s *ServiceWithCache) GetDelegationsToLogin(ctx c.Context, login string) (ds Delegations, err error) {
	return s.Humantasks.GetDelegationsToLogin(ctx, login)
}

func (s *ServiceWithCache) GetDelegationsToLogins(ctx c.Context, logins []string) (ds Delegations, err error) {
	return s.Humantasks.GetDelegationsToLogins(ctx, logins)
}

func (s *ServiceWithCache) GetDelegationsByLogins(ctx c.Context, logins []string) (ds Delegations, err error) {
	return s.Humantasks.GetDelegationsByLogins(ctx, logins)
}
