package nocache

import (
	"net/http"

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
	sdURL string
	cli   *retryablehttp.Client
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
		cli:   httpclient.NewClient(httpClient, nil, cfg.MaxRetries, cfg.RetryDelay),
		sdURL: cfg.ServicedeskURL,
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

func (s *service) Ping() error {
	req, err := retryablehttp.NewRequest("HEAD", s.sdURL, nil)
	if err != nil {
		return err
	}

	resp, err := s.cli.Do(req)
	if err != nil {
		return err
	}

	return resp.Body.Close()
}
