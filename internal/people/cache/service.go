package cache

import (
	c "context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/go-retryablehttp"

	"go.opencensus.io/trace"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	cachekit "gitlab.services.mts.ru/jocasta/cache-kit"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/metrics"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people/nocache"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
)

const (
	userKeyPrefix  = "user:"
	usersKeyPrefix = "users:"
)

type service struct {
	Cache  cachekit.Cache
	People people.Service
}

func NewService(cfg *people.Config, ssoS *sso.Service, m metrics.Metrics) (people.Service, error) {
	srv, err := nocache.NewService(cfg, ssoS, m)
	if err != nil {
		return nil, err
	}

	cache, cacheErr := cachekit.CreateCache(cachekit.Config(cfg.Cache))
	if cacheErr != nil {
		return nil, cacheErr
	}

	return &service{
		People: srv,
		Cache:  cache,
	}, nil
}

func (s *service) SetCli(cli *retryablehttp.Client) {}

func (s *service) Ping(ctx c.Context) error {
	return s.People.Ping(ctx)
}

func (s *service) GetUser(ctx c.Context, username string) (people.SSOUser, error) {
	ctx, span := trace.StartSpan(ctx, "people.cache.get_user")
	defer span.End()

	log := logger.GetLogger(ctx)

	keyForCache := userKeyPrefix + username

	valueFromCache, err := s.Cache.GetValue(ctx, keyForCache)
	if err == nil {
		resources, ok := valueFromCache.(string)
		if ok {
			var data people.SSOUser

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

func (s *service) GetUsers(ctx c.Context, username string, limit *int, filter []string) ([]people.SSOUser, error) {
	ctx, span := trace.StartSpan(ctx, "people.cache.get_users")
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
				var data []people.SSOUser

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

func (s *service) PathBuilder(mainPath, subPath string) (string, error) {
	return s.People.PathBuilder(mainPath, subPath)
}

func (s *service) GetUserEmail(ctx c.Context, username string) (string, error) {
	ctx, span := trace.StartSpan(ctx, "people.cache.get_user_email")
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
