package people

import (
	"context"
	"fmt"

	cachekit "gitlab.services.mts.ru/jocasta/cache-kit"
)

const (
	userKeyPrefix  = "user:"
	usersKeyPrefix = "users:"
)

type ServiceWithCache struct {
	Cache  cachekit.Cache
	People ServiceInterface
}

func (s *ServiceWithCache) GetUser(ctx context.Context, search string, onlyEnabled bool) ([]SSOUser, error) {
	keyForCache := userKeyPrefix + search

	valueFromCache, err := s.Cache.GetValue(ctx, keyForCache)
	if err == nil {
		resources, ok := valueFromCache.([]SSOUser)
		if !ok {
			return nil, fmt.Errorf("failed to cast value from cache to type []SSOUser")
		}

		return resources, nil
	}

	resources, err := s.People.GetUser(ctx, search, onlyEnabled)
	if err != nil {
		return nil, err
	}

	err = s.Cache.SetValue(ctx, keyForCache, resources)
	if err != nil {
		return nil, fmt.Errorf("can't set resources to cache: %s", err)
	}

	return resources, nil
}

// TODO создать ключ
func (s *ServiceWithCache) GetUsers(ctx context.Context, search string, limit int, filter []string) ([]SSOUser, error) {
	keyForCache := usersKeyPrefix + search

	valueFromCache, err := s.Cache.GetValue(ctx, keyForCache)
	if err == nil {
		resources, ok := valueFromCache.([]SSOUser)
		if !ok {
			return nil, fmt.Errorf("failed to cast value from cache to type []SSOUser")
		}

		return resources, nil
	}

	resources, err := s.People.GetUsers(ctx, search, limit, filter)
	if err != nil {
		return nil, err
	}

	err = s.Cache.SetValue(ctx, keyForCache, resources)
	if err != nil {
		return nil, fmt.Errorf("can't set resources to cache: %s", err)
	}

	return resources, nil
}

func (s *ServiceWithCache) PathBuilder(mainpath, subpath string) (string, error) {
	return s.People.PathBuilder(mainpath, subpath)
}

func (s *ServiceWithCache) GetUserEmail(ctx context.Context, username string) (string, error) {
	return s.People.GetUserEmail(ctx, username)
}
