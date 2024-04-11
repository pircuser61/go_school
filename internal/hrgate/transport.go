package hrgate

import (
	"net/http"

	"go.opencensus.io/plugin/ochttp"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
)

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
