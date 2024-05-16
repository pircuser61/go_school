package nocache

import (
	"net/http"

	"go.opencensus.io/plugin/ochttp"

	"github.com/hashicorp/go-retryablehttp"

	"gitlab.services.mts.ru/abp/myosotis/observability"
	cachekit "gitlab.services.mts.ru/jocasta/cache-kit"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/httpclient"
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
	Cache cachekit.Cache
}

func NewService(cfg *servicedesc.Config, ssoS *sso.Service) (servicedesc.Service, error) {
	httpClient := &http.Client{}

	tr := transport{
		next: ochttp.Transport{
			Base:        httpClient.Transport,
			Propagation: observability.NewHTTPFormat(),
		},
		sso:   ssoS,
		scope: cfg.Scope,
	}

	httpClient.Transport = &tr
	newCli := httpclient.NewClient(httpClient, nil, cfg.MaxRetries, cfg.RetryDelay)

	return &service{
		Cli:   newCli,
		SdURL: cfg.ServicedeskURL,
	}, nil
}

func (s *service) GetSdURL() string {
	return s.SdURL
}

func (s *service) GetCli() *retryablehttp.Client {
	return s.Cli
}
