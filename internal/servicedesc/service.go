package servicedesc

import (
	"net/http"

	"go.opencensus.io/plugin/ochttp"

	"gitlab.services.mts.ru/abp/myosotis/observability"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
)

type Service struct {
	sdURL string

	cli *http.Client
}

func NewService(cfg Config, ssoS *sso.Service) (*Service, error) {
	newCli := &http.Client{}

	tr := TransportForPeople{
		transport: ochttp.Transport{
			Base:        newCli.Transport,
			Propagation: observability.NewHTTPFormat(),
		},
		sso:   ssoS,
		scope: cfg.Scope,
	}
	newCli.Transport = &tr

	s := &Service{
		cli:   newCli,
		sdURL: cfg.ServicedeskURL,
	}

	return s, nil
}

type TransportForPeople struct {
	transport ochttp.Transport
	sso       *sso.Service
	scope     string
}

func (t *TransportForPeople) RoundTrip(req *http.Request) (*http.Response, error) {
	err := t.sso.BindAuthHeader(req.Context(), req, t.scope)
	if err != nil {
		return nil, err
	}

	return t.transport.RoundTrip(req)
}
