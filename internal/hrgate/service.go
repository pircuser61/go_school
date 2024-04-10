package hrgate

import (
	"net/http"
	"time"

	"gitlab.services.mts.ru/abp/myosotis/observability"
	cachekit "gitlab.services.mts.ru/jocasta/cache-kit"
	"go.opencensus.io/plugin/ochttp"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
)

type ServiceWithCache struct {
	Cache  cachekit.Cache
	HRGate HRGateInterface
}

type Service struct {
	HRGateURL             string
	DefaultCalendarUnitID *string
	Location              time.Location
	Cli                   *ClientWithResponses
	Cache                 cachekit.Cache
}

func NewServiceWithCache(cfg Config, ssoS *sso.Service) (HRGateInterface, error) {
	service, err := NewService(cfg, ssoS)
	if err != nil {
		return nil, err
	}

	cache, cacheErr := cachekit.CreateCache(cachekit.Config(cfg.CacheConfig))
	if cacheErr != nil {
		return nil, cacheErr
	}

	return &ServiceWithCache{
		HRGate: service,
		Cache:  cache,
	}, nil
}

func NewService(cfg Config, ssoS *sso.Service) (HRGateInterface, error) {
	httpClient := &http.Client{}
	tr := TransportForHrGate{
		transport: ochttp.Transport{
			Base:        httpClient.Transport,
			Propagation: observability.NewHTTPFormat(),
		},
		sso:   ssoS,
		scope: cfg.Scope,
	}
	httpClient.Transport = &tr

	newCli, createClientErr := NewClientWithResponses(cfg.HRGateURL, WithHTTPClient(httpClient), WithBaseURL(cfg.HRGateURL))
	if createClientErr != nil {
		return nil, createClientErr
	}

	location, getLocationErr := time.LoadLocation("Europe/Moscow")
	if getLocationErr != nil {
		return nil, getLocationErr
	}

	return &Service{
		Cli:       newCli,
		HRGateURL: cfg.HRGateURL,
		Location:  *location,
	}, nil
}
