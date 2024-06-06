package nocache

import (
	c "context"
	"net/http"

	"go.opencensus.io/plugin/ochttp"

	"github.com/hashicorp/go-retryablehttp"

	"gitlab.services.mts.ru/abp/myosotis/observability"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/metrics"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"

	iga_kit "gitlab.services.mts.ru/jocasta/iga-kit"
)

type service struct {
	iga iga_kit.Service
}

func NewService(cfg *people.Config, m metrics.Metrics) (people.Service, error) {
	tr := &transport{
		next: ochttp.Transport{
			Base:        http.Client{}.Transport,
			Propagation: observability.NewHTTPFormat(),
		},
		metrics: m,
	}

	iga, err := iga_kit.NewIGA(&iga_kit.Config{
		URL:        cfg.URL,
		MaxRetries: cfg.MaxRetries,
		RetryDelay: cfg.RetryDelay,
	}, tr)
	if err != nil {
		return nil, err
	}

	res := &service{
		iga: iga,
	}

	return res, nil
}

func (s *service) SetCli(cli *retryablehttp.Client) {
	s.iga.SetCli(cli)
}

func (s *service) Ping(ctx c.Context) error {
	return s.iga.Ping(ctx)
}
