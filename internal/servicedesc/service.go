package servicedesc

import (
	"net/http"

	"go.opencensus.io/plugin/ochttp"

	"gitlab.services.mts.ru/abp/myosotis/observability"

	cachekit "gitlab.services.mts.ru/jocasta/cache-kit"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/httpclient"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
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
		Cache:       cache,
		Servicedesc: service,
	}, nil
}

func NewService(cfg *Config, ssoS *sso.Service) (ServiceInterface, error) {
	httpClient := &http.Client{}

	tr := TransportForPeople{
		transport: ochttp.Transport{
			Base:        httpClient.Transport,
			Propagation: observability.NewHTTPFormat(),
		},
		sso:   ssoS,
		scope: cfg.Scope,
	}

	httpClient.Transport = &tr
	newCli := httpclient.NewClient(httpClient, nil, cfg.MaxRetries, cfg.RetryDelay)

	return &Service{
		Cli:   newCli,
		SdURL: cfg.ServicedeskURL,
	}, nil
}
