package hrgate

import (
	"net/http"
	"time"

	"github.com/hashicorp/go-retryablehttp"

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
		HRGate: service,
		Cache:  cache,
	}, nil
}

func NewService(cfg *Config, ssoS *sso.Service) (ServiceInterface, error) {
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
	retryableCli := httpclient.NewClient(httpClient, nil, cfg.MaxRetries, cfg.RetryDelay)
	wrappedRetryableCli := httpRequestDoer{retryableCli}

	newCli, createClientErr := NewClientWithResponses(cfg.HRGateURL, WithHTTPClient(wrappedRetryableCli), WithBaseURL(cfg.HRGateURL))
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

type httpRequestDoer struct {
	*retryablehttp.Client
}

func (h httpRequestDoer) Do(req *http.Request) (*http.Response, error) {
	wrappedRequest, err := retryablehttp.FromRequest(req)
	if err != nil {
		return nil, err
	}

	return h.Client.Do(wrappedRequest)
}
