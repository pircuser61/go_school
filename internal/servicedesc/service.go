package servicedesc

import (
	cachekit "gitlab.services.mts.ru/jocasta/cache-kit"
	"net/http"

	"go.opencensus.io/plugin/ochttp"

	"gitlab.services.mts.ru/abp/myosotis/observability"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
)

type ServiceWithCache struct {
	Cache       cachekit.Cache
	Servicedesc ServicedescInterface
}

type Service struct {
	SdURL string
	Cli   *http.Client
	Cache cachekit.Cache
}

func NewServiceWithCache(cfg Config, ssoS *sso.Service) (ServicedescInterface, error) {
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

func NewService(cfg Config, ssoS *sso.Service) (ServicedescInterface, error) {
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
