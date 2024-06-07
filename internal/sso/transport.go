package sso

import (
	"net/http"
	"time"

	"go.opencensus.io/plugin/ochttp"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/metrics"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

const (
	externalSystemName = "isso"
	xRequestIDHeader   = "X-Request-Id"
)

type transport struct {
	next    ochttp.Transport
	metrics metrics.Metrics
	Scope   string
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	info := metrics.NewExternalRequestInfo(externalSystemName)
	info.Method = req.Method
	info.URL = req.URL.String()
	info.TraceID = req.Header.Get(xRequestIDHeader)

	start := time.Now()

	res, err := t.next.RoundTrip(req)
	code := http.StatusServiceUnavailable
	script.IncreaseReqRetryCntREST(req)

	if res != nil {
		code = res.StatusCode
	}

	info.ResponseCode = code
	info.Duration = time.Since(start)

	t.metrics.Request2ExternalSystem(info)

	return res, err
}
