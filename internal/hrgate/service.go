package hrgate

import (
	"gitlab.services.mts.ru/abp/myosotis/observability"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
	"go.opencensus.io/plugin/ochttp"
	"net/http"
)

type Service struct {
	HrGateUrl string
	Cli       *http.Client
}

func NewService(cfg Config, ssoS *sso.Service) (*Service, error) {
	newCli := &http.Client{}

	tr := TransportForHrGate{
		transport: ochttp.Transport{
			Base:        newCli.Transport,
			Propagation: observability.NewHTTPFormat(),
		},
		sso:   ssoS,
		scope: cfg.Scope,
	}
	newCli.Transport = &tr

	s := &Service{
		Cli:       newCli,
		HrGateUrl: cfg.HrGateUrl,
	}

	return s, nil
}

type TransportForHrGate struct {
	transport ochttp.Transport
	sso       *sso.Service
	scope     string
}

func (t *TransportForHrGate) RoundTrip(req *http.Request) (*http.Response, error) {
	err := t.sso.BindAuthHeader(req.Context(), req, t.scope)
	if err != nil {
		return nil, err
	}

	return t.transport.RoundTrip(req)
}
