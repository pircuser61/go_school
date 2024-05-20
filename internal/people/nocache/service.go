package nocache

import (
	"net/http"

	"go.opencensus.io/plugin/ochttp"

	"github.com/hashicorp/go-retryablehttp"

	"gitlab.services.mts.ru/abp/myosotis/observability"

	cachekit "gitlab.services.mts.ru/jocasta/cache-kit"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/httpclient"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/metrics"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
)

const searchPath = "search/attributes"

type service struct {
	searchURL string
	cli       *retryablehttp.Client
	sso       *sso.Service
	cache     cachekit.Cache
}

func NewService(cfg *people.Config, ssoS *sso.Service, m metrics.Metrics) (people.Service, error) {
	httpClient := &http.Client{}

	httpClient.Transport = &transport{
		next: ochttp.Transport{
			Base:        httpClient.Transport,
			Propagation: observability.NewHTTPFormat(),
		},
		sso:     ssoS,
		scope:   "",
		metrics: m,
	}

	res := &service{
		cli: httpclient.NewClient(httpClient, nil, cfg.MaxRetries, cfg.RetryDelay),
		sso: ssoS,
	}

	search, err := res.PathBuilder(cfg.URL, searchPath)
	if err != nil {
		return nil, err
	}

	res.searchURL = search

	return res, nil
}
