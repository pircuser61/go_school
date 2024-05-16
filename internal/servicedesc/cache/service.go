package cache

import (
	cachekit "gitlab.services.mts.ru/jocasta/cache-kit"

	sd "gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc/nocache"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
)

type service struct {
	cache       cachekit.Cache
	servicedesc sd.Service
}

func NewService(cfg *sd.Config, ssoS *sso.Service) (sd.Service, error) {
	srv, err := nocache.NewService(cfg, ssoS)
	if err != nil {
		return nil, err
	}

	cache, cacheErr := cachekit.CreateCache(cachekit.Config(cfg.Cache))
	if cacheErr != nil {
		return nil, cacheErr
	}

	return &service{
		cache:       cache,
		servicedesc: srv,
	}, nil
}
