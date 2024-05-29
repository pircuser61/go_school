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
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
)

const (
	getSchemaByID          = "/api/herald/v1/schema/"
	getSchemaByBlueprintID = "/api/herald/v1/schema/"
	getUserInfo            = "/api/herald/v1/externalData/user/single?search=%s"
	getWorkGroup           = "/api/chainsmith/v1/workGroup/"
)

type service struct {
	sdURL   string
	cli     *retryablehttp.Client
	cliPing *http.Client
}

func NewService(cfg *servicedesc.Config, ssoS *sso.Service, m metrics.Metrics) (servicedesc.Service, error) {
	httpClient := &http.Client{}

	httpClient.Transport = &transport{
		next: ochttp.Transport{
			Base:        httpClient.Transport,
			Propagation: observability.NewHTTPFormat(),
		},
		sso:     ssoS,
		scope:   cfg.Scope,
		metrics: m,
	}

	return &service{
		sdURL:   cfg.ServicedeskURL,
		cliPing: &http.Client{Timeout: time.Second * 2},
		cli:     httpclient.NewClient(httpClient, nil, cfg.MaxRetries, cfg.RetryDelay),
	}, nil
}

func (s *service) GetSdURL() string {
	return s.sdURL
}

func (s *service) SetCli(cli *retryablehttp.Client) {
	s.cli = cli
}

func (s *service) GetCli() *retryablehttp.Client {
	return s.cli
}

func (s *service) Ping(ctx c.Context) error {
	req, err := http.NewRequestWithContext(ctx, "HEAD", s.sdURL, http.NoBody)
	if err != nil {
		return err
	}

	resp, err := s.cliPing.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusUnauthorized {
		return fmt.Errorf("wrong status code: %d", resp.StatusCode)
	}

	return resp.Body.Close()
}
