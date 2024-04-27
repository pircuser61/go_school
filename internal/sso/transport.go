package sso

import (
	"go.opencensus.io/plugin/ochttp"
	"net/http"
)

type TransportForSso struct {
	Transport ochttp.Transport
	Scope     string
}

func (t *TransportForSso) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.Transport.RoundTrip(req)
}
