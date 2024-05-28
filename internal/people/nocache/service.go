package nocache

import (
	c "context"
	"fmt"
	"net/http"
	"time"

	"go.opencensus.io/plugin/ochttp"

	"github.com/hashicorp/go-retryablehttp"

	"gitlab.services.mts.ru/abp/myosotis/observability"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/httpclient"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/metrics"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
)

const searchPath = "search/attributes"

type service struct {
	searchURL string
	baseURL   string
	cli       *retryablehttp.Client
	cliPing   *http.Client
	sso       *sso.Service
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
		sso:     ssoS,
		cliPing: &http.Client{Timeout: time.Second * 2},
		cli:     httpclient.NewClient(httpClient, nil, cfg.MaxRetries, cfg.RetryDelay),
	}

	search, err := res.PathBuilder(cfg.URL, searchPath)
	if err != nil {
		return nil, err
	}

	res.searchURL = search

	return res, nil
}

func (s *service) SetCli(cli *retryablehttp.Client) {
	s.cli = cli
}

func (s *service) Ping(ctx c.Context) error {
	req, err := http.NewRequestWithContext(ctx, "HEAD", s.searchURL, http.NoBody)
	if err != nil {
		return err
	}

	resp, err := s.cliPing.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusMethodNotAllowed {
		return fmt.Errorf("wrong status code: %d", resp.StatusCode)
	}

	return resp.Body.Close()
}
