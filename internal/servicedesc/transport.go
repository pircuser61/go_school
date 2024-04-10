package servicedesc

import (
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
	"go.opencensus.io/plugin/ochttp"
	"net/http"
)

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
