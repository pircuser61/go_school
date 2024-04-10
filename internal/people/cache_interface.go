package people

import (
	"context"
	"fmt"
)

func (s *ServiceWithCache) getUser(ctx context.Context, search string, onlyEnabled bool) ([]SSOUser, error) {
	keyForCache := "user" + ":" + search

	valueFromCache, err := s.Cache.GetValue(ctx, keyForCache)
	if err == nil {
		resources, ok := valueFromCache.([]SSOUser)
		if !ok {
			return nil, fmt.Errorf("failed to cast value from cache to type []SSOUser")
		}

		return resources, nil
	}

	resources, err := s.People.getUser(ctx, search, onlyEnabled)
	if err != nil {
		return nil, err
	}

	err = s.Cache.SetValue(ctx, keyForCache, resources)
	if err != nil {
		return nil, fmt.Errorf("can't set resources to cache: %s", err)
	}

	return resources, nil
}

func (s *ServiceWithCache) getUsers(ctx context.Context, search string, limit int, filter []string) ([]SSOUser, error) {
	keyForCache := "users" + ":" + search

	valueFromCache, err := s.Cache.GetValue(ctx, keyForCache)
	if err == nil {
		resources, ok := valueFromCache.([]SSOUser)
		if !ok {
			return nil, fmt.Errorf("failed to cast value from cache to type []SSOUser")
		}

		return resources, nil
	}

	resources, err := s.People.getUsers(ctx, search, limit, filter)
	if err != nil {
		return nil, err
	}

	err = s.Cache.SetValue(ctx, keyForCache, resources)
	if err != nil {
		return nil, fmt.Errorf("can't set resources to cache: %s", err)
	}

	return resources, nil
}

func (s *ServiceWithCache) pathBuilder(mainpath, subpath string) (string, error) {
	return s.People.pathBuilder(mainpath, subpath)
}

func (s *ServiceWithCache) GetUserEmail(ctx context.Context, username string) (string, error) {
	return s.People.GetUserEmail(ctx, username)
}

func (s *ServiceWithCache) GetUser(ctx context.Context, username string) (SSOUser, error) {
	return s.People.GetUser(ctx, username)
}

func (s *ServiceWithCache) GetUsers(ctx context.Context, username string, limit *int, filter []string) ([]SSOUser, error) {
	return s.People.GetUsers(ctx, username, limit, filter)
}
