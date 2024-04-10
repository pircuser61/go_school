package people

import (
	"gitlab.services.mts.ru/abp/myosotis/observability"
	cachekit "gitlab.services.mts.ru/jocasta/cache-kit"
	"go.opencensus.io/plugin/ochttp"
	"net/http"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
)

type ServiceWithCache struct {
	Cache  cachekit.Cache
	People PeopleInterface
}

type Service struct {
	SearchURL string

	Cli   *http.Client `json:"-"`
	Sso   *sso.Service
	Cache cachekit.Cache
}

func NewServiceWithCache(cfg Config, ssoS *sso.Service) (PeopleInterface, error) {
	service, err := NewService(cfg, ssoS)
	if err != nil {
		return nil, err
	}

	cache, cacheErr := cachekit.CreateCache(cachekit.Config(cfg.CacheConfig))
	if cacheErr != nil {
		return nil, cacheErr
	}

	return &ServiceWithCache{
		People: service,
		Cache:  cache,
	}, nil
}

func NewService(c Config, ssoS *sso.Service) (PeopleInterface, error) {
	newCli := &http.Client{}

	tr := TransportForPeople{
		transport: ochttp.Transport{
			Base:        newCli.Transport,
			Propagation: observability.NewHTTPFormat(),
		},
		sso:   ssoS,
		scope: "",
	}
	newCli.Transport = &tr

	service := &Service{
		Cli: newCli,
		Sso: ssoS,
	}

	search, err := service.pathBuilder(c.URL, searchPath)
	if err != nil {
		return nil, err
	}

	service.SearchURL = search

	return service, nil
}
