package cache

import (
	c "context"

	"github.com/hashicorp/go-retryablehttp"

	cachekit "gitlab.services.mts.ru/jocasta/cache-kit"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/metrics"
	sd "gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc/nocache"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
)

type service struct {
	cache       cachekit.Cache
	servicedesc sd.Service
}

func NewService(cfg *sd.Config, ssoS *sso.Service, m metrics.Metrics) (sd.Service, error) {
	srv, err := nocache.NewService(cfg, ssoS, m)
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

func (s *service) Ping(ctx c.Context) error {
	return s.servicedesc.Ping(ctx)
}

func (s *service) SetCli(*retryablehttp.Client) {}
