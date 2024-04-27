package sso

import (
	"net/http"

	"go.opencensus.io/plugin/ochttp"
)

type TransportForSso struct {
	Transport ochttp.Transport
	Scope     string
}

func (t *TransportForSso) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.Transport.RoundTrip(req)
}
