package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
)

type Metrics interface {
	ServePrometheus() http.Handler
	MustRegisterMetrics(registry *prometheus.Registry)

	RequestsIncrease(label *RequestInfo)
	KafkaAvailable()
	KafkaUnavailable()

	Request2ExternalSystem(label *ExternalRequestInfo)
}
