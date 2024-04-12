package people

import (
	"context"
	"encoding/json"
	"fmt"

	"go.opencensus.io/trace"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/abp/myosotis/logger"

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

func (s *ServiceWithCache) GetUser(ctx context.Context, username string) (SSOUser, error) {
	ctx, span := trace.StartSpan(ctx, "people.get_user(cached)")
	defer span.End()

	log := logger.GetLogger(ctx)

	keyForCache := userKeyPrefix + username

	valueFromCache, err := s.Cache.GetValue(ctx, keyForCache)
	if err == nil {
		resources, ok := valueFromCache.(string)
		if ok {
			var data SSOUser

			unmErr := json.Unmarshal([]byte(resources), &data)
			if unmErr == nil {
				log.Info("got resources from cache")

				return data, nil
			}
		}

		err = s.Cache.DeleteValue(ctx, keyForCache)
		if err != nil {
			log.WithError(err).Error("can't delete key from cache")
		}
	}

	resources, err := s.People.GetUser(ctx, username)
	if err != nil {
		return nil, err
	}

	resourcesData, err := json.Marshal(resources)
	if err == nil && keyForCache != "" {
		err = s.Cache.SetValue(ctx, keyForCache, string(resourcesData))
		if err != nil {
			return nil, fmt.Errorf("can't set resources to cache: %s", err)
		}
	}

	return resources, nil
}

func (s *ServiceWithCache) GetUsers(ctx context.Context, username string, limit *int, filter []string) ([]SSOUser, error) {
	ctx, span := trace.StartSpan(ctx, "people.get_users(cached)")
	defer span.End()

	log := logger.GetLogger(ctx)

	var keyForCache string

	f, err := json.Marshal(filter)
	if err == nil { //nolint:nestif //так нужно
		keyForCache = fmt.Sprintf("%s%s%d%s", usersKeyPrefix, username, *limit, f)

		valueFromCache, err := s.Cache.GetValue(ctx, keyForCache) //nolint:govet //ничего страшного
		if err == nil {
			resources, ok := valueFromCache.(string)
			if ok {
				var data []SSOUser

				unmErr := json.Unmarshal([]byte(resources), &data)
				if unmErr == nil {
					log.Info("got resources from cache")

					return data, nil
				}
			}

			err = s.Cache.DeleteValue(ctx, keyForCache)
			if err != nil {
				log.WithError(err).Error("can't delete key from cache")
			}
		}
	}

	resources, err := s.People.GetUsers(ctx, username, limit, filter)
	if err != nil {
		return nil, err
	}

	resourcesData, err := json.Marshal(resources)
	if err == nil && keyForCache != "" {
		err = s.Cache.SetValue(ctx, keyForCache, string(resourcesData))
		if err != nil {
			return nil, fmt.Errorf("can't set resources to cache: %s", err)
		}
	}

	return resources, nil
}

func (s *ServiceWithCache) PathBuilder(mainpath, subpath string) (string, error) {
	return s.People.PathBuilder(mainpath, subpath)
}

func (s *ServiceWithCache) GetUserEmail(ctx context.Context, username string) (string, error) {
	ctx, span := trace.StartSpan(ctx, "people.get_user_email(cached)")
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
