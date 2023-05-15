package hrgate

import (
	"net/http"
	"time"

	"go.opencensus.io/plugin/ochttp"

	"gitlab.services.mts.ru/abp/myosotis/observability"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
)

type Service struct {
	HrGateUrl             string
	DefaultCalendarUnitId *string
	Location              time.Location
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

	location, getLocationErr := time.LoadLocation("Europe/Moscow")
	if getLocationErr != nil {
		return nil, getLocationErr
	}

	s := &Service{
		Cli:       newCli,
		HrGateUrl: cfg.HrGateUrl,
		Location:  *location,
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

func (s *Service) GetLocation() time.Location {
	return s.Location
}
