package hrgate

import (
	"net/http"

	"github.com/google/uuid"
	"go.opencensus.io/plugin/ochttp"

	"gitlab.services.mts.ru/abp/myosotis/observability"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
)

type Service struct {
	HrGateUrl             string
	DefaultCalendarUnitId *uuid.UUID
	Cli                   *ClientWithResponses
}

func NewService(cfg Config, ssoS *sso.Service) (*Service, error) {
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

	newCli, createClientErr := NewClientWithResponses(cfg.HrGateUrl, WithHTTPClient(httpClient), WithBaseURL(cfg.HrGateUrl))
	if createClientErr != nil {
		return nil, createClientErr
	}

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
