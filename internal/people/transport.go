package people

import (
	"net/http"

	"go.opencensus.io/plugin/ochttp"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
)

type TransportForPeople struct {
	Transport ochttp.Transport
	Sso       *sso.Service
	Scope     string
}

func (t *TransportForPeople) RoundTrip(req *http.Request) (*http.Response, error) {
	err := t.Sso.BindAuthHeader(req.Context(), req, t.Scope)
	if err != nil {
		return nil, err
	}

	return t.Transport.RoundTrip(req)
}
