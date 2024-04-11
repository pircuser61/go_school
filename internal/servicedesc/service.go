package servicedesc

import (
	"net/http"

	cachekit "gitlab.services.mts.ru/jocasta/cache-kit"
	"go.opencensus.io/plugin/ochttp"

	"gitlab.services.mts.ru/abp/myosotis/observability"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
)

func NewServiceWithCache(cfg *Config, ssoS *sso.Service) (ServiceInterface, error) {
	service, err := NewService(cfg, ssoS)
	if err != nil {
		return nil, err
	}

	cache, cacheErr := cachekit.CreateCache(cachekit.Config(cfg.CacheConfig))
	if cacheErr != nil {
		return nil, cacheErr
	}

	return &ServiceWithCache{
		Cache:       cache,
		Servicedesc: service,
	}, nil
}

func NewService(cfg *Config, ssoS *sso.Service) (ServiceInterface, error) {
	newCli := &http.Client{}

	tr := TransportForPeople{
		transport: ochttp.Transport{
			Base:        newCli.Transport,
			Propagation: observability.NewHTTPFormat(),
		},
		sso:   ssoS,
		scope: cfg.Scope,
	}
	newCli.Transport = &tr

	return &Service{
		Cli:   newCli,
		SdURL: cfg.ServicedeskURL,
	}, nil
}
