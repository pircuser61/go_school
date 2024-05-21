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
	SdURL string
	Cli   *retryablehttp.Client
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
		Cli:   httpclient.NewClient(httpClient, nil, cfg.MaxRetries, cfg.RetryDelay),
		SdURL: cfg.ServicedeskURL,
	}, nil
}

func (s *service) GetSdURL() string {
	return s.SdURL
}

func (s *service) SetCli(cli *retryablehttp.Client) {
	s.Cli = cli
}

func (s *service) GetCli() *retryablehttp.Client {
	return s.Cli
}
