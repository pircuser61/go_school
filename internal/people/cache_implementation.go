package people

import (
	"context"
	"encoding/json"
	"fmt"
	"gitlab.services.mts.ru/abp/myosotis/logger"
	"go.opencensus.io/trace"

	"github.com/pkg/errors"

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

func (s *ServiceWithCache) GetUser(ctx context.Context, search string) (SSOUser, error) {
	log := logger.CreateLogger(nil)

	ctx, span := trace.StartSpan(ctx, "people.get_user")
	defer span.End()

	keyForCache := userKeyPrefix + search

	valueFromCache, err := s.Cache.GetValue(ctx, keyForCache)
	if err == nil {
		resources, ok := valueFromCache.(SSOUser)
		if !ok {
			err = s.Cache.DeleteValue(ctx, keyForCache)
			if err != nil {
				log.WithError(err).Error("can't delete key from cache")
			}
		}

		return resources, nil
	}

	resources, err := s.People.GetUser(ctx, search)
	if err != nil {
		return nil, err
	}

	err = s.Cache.SetValue(ctx, keyForCache, resources)
	if err != nil {
		return nil, fmt.Errorf("can't set resources to cache: %s", err)
	}

	return resources, nil
}

func (s *ServiceWithCache) GetUsers(ctx context.Context, search string, limit *int, filter []string) ([]SSOUser, error) {
	log := logger.CreateLogger(nil)

	ctx, span := trace.StartSpan(ctx, "people.get_users")
	defer span.End()

	f, err := json.Marshal(filter)
	if err != nil {
		log.WithError(err).Error("can't marshal filter")
	}

	keyForCache := usersKeyPrefix + search + string(rune(*limit)) + string(f)

	valueFromCache, err := s.Cache.GetValue(ctx, keyForCache)
	if err == nil {
		resources, ok := valueFromCache.([]SSOUser)
		if !ok {
			err = s.Cache.DeleteValue(ctx, keyForCache)
			if err != nil {
				log.WithError(err).Error("can't delete key from cache")
			}
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
	ctx, span := trace.StartSpan(ctx, "people.get_user_email")
	defer span.End()

	user, err := s.GetUser(ctx, username)
	if err != nil {
		return "", err
	}

	typed, err := user.ToSSOUserTyped()
	if err != nil {
		return "", errors.Wrap(err, "couldn't convert user")
	}

	return typed.Email, nil
}
