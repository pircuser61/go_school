package people

import (
	"net/http"

	"go.opencensus.io/plugin/ochttp"

	"gitlab.services.mts.ru/abp/myosotis/observability"

	cachekit "gitlab.services.mts.ru/jocasta/cache-kit"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/httpclient"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
)

const (
	searchPath = "search/attributes"
)

func NewServiceWithCache(cfg *Config, ssoS *sso.Service) (ServiceInterface, error) {
	service, err := NewService(cfg, ssoS)
	if err != nil {
		return nil, err
	}

	cache, cacheErr := cachekit.CreateCache(cachekit.Config(cfg.Cache))
	if cacheErr != nil {
		return nil, cacheErr
	}

	return &ServiceWithCache{
		People: service,
		Cache:  cache,
	}, nil
}

func NewService(cfg *Config, ssoS *sso.Service) (ServiceInterface, error) {
	httpClient := &http.Client{}

	tr := TransportForPeople{
		Transport: ochttp.Transport{
			Base:        httpClient.Transport,
			Propagation: observability.NewHTTPFormat(),
		},
		Sso:   ssoS,
		Scope: "",
	}

	httpClient.Transport = &tr
	newCli := httpclient.HTTPClientWithRetries(httpClient, nil, cfg.MaxRetries, cfg.RetryDelay)

	service := &Service{
		Cli: newCli,
		Sso: ssoS,
	}

	search, err := service.PathBuilder(cfg.URL, searchPath)
	if err != nil {
		return nil, err
	}

	service.SearchURL = search

	return service, nil
}
