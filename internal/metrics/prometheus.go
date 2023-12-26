package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	incomingRequests = "incoming_requests"
)

type appMetrics struct {
	registry *prometheus.Registry

	incomingRequests *prometheus.SummaryVec
}

type Metrics interface {
	ServePrometheus() http.Handler
	MustRegisterMetrics(registry *prometheus.Registry)
	RequestsIncrease(label *RequestInfo)
}

type RequestInfo struct {
	Method     string
	Path       string
	PipelineID string
	ClientID   string
	WorkNumber string
	Status     int
	Duration   time.Duration
}

func New() Metrics {
	registry := prometheus.NewRegistry()

	m := &appMetrics{
		registry: registry,
		incomingRequests: prometheus.NewSummaryVec(prometheus.SummaryOpts{
			Namespace: "jocasta",
			Subsystem: "pipeliner",
			Name:      incomingRequests,
		}, []string{"method", "path", "pipeline_id", "client_id", "work_number", "status"}),
	}

	m.MustRegisterMetrics(registry)

	return m
}

func (m *appMetrics) ServePrometheus() http.Handler {
	return http.HandlerFunc(m.handleMetricsRequest)
}

func (m *appMetrics) handleMetricsRequest(w http.ResponseWriter, r *http.Request) {
	h := promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)
}

func (m *appMetrics) MustRegisterMetrics(registry *prometheus.Registry) {
	registry.MustRegister(
		m.incomingRequests,
	)
}

func (m *appMetrics) RequestsIncrease(label *RequestInfo) {
	m.incomingRequests.WithLabelValues([]string{
		label.Method,
		label.Path,
		label.PipelineID,
		label.ClientID,
		label.WorkNumber,
		strconv.Itoa(label.Status),
	}...).Observe(label.Duration.Seconds())
}
