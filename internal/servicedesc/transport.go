package servicedesc

import (
	"net/http"

	"go.opencensus.io/plugin/ochttp"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
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
