package nocache

import (
	"net/http"
	"strings"
	"time"

	"go.opencensus.io/plugin/ochttp"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/metrics"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
)

const (
	heraldSystemName     = "herald"
	chainsmithSystemName = "chainsmith"
	xRequestIDHeader     = "X-Request-Id"
)

type transport struct {
	next    ochttp.Transport
	sso     *sso.Service
	scope   string
	metrics metrics.Metrics
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	err := t.sso.BindAuthHeader(req.Context(), req, t.scope)
	if err != nil {
		return nil, err
	}

	systemName := heraldSystemName

	if strings.Contains(req.URL.String(), chainsmithSystemName) {
		systemName = chainsmithSystemName
	}

	info := metrics.NewExternalRequestInfo(systemName)
	info.Method = req.Method
	info.URL = req.URL.String()
	info.TraceID = req.Header.Get(xRequestIDHeader)

	start := time.Now()

	res, err := t.next.RoundTrip(req)
	code := http.StatusServiceUnavailable

	if res != nil {
		code = res.StatusCode
	}

	info.ResponseCode = code
	info.Duration = time.Since(start)

	t.metrics.Request2ExternalSystem(info)

	return res, err
}
