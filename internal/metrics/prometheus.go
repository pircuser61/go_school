package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/configs"
)

const (
	incomingRequests  = "incoming_requests"
	kafkaAvailability = "kafka_availability"
)

type appMetrics struct {
	registry *prometheus.Registry
	stand    string

	incomingRequests  *prometheus.SummaryVec
	kafkaAvailability prometheus.Gauge
}

type Metrics interface {
	ServePrometheus() http.Handler
	MustRegisterMetrics(registry *prometheus.Registry)

	RequestsIncrease(label *RequestInfo)
	KafkaAvailable()
	KafkaUnavailable()
}

type RequestInfo struct {
	Method     string
	Path       string
	PipelineID string
	VersionID  string
	ClientID   string
	WorkNumber string
	Status     int
	Duration   time.Duration
}

func NewPostRequestInfo(path string) *RequestInfo {
	return &RequestInfo{
		Method: http.MethodPost,
		Status: http.StatusOK,
		Path:   path,
	}
}

func NewGetRequestInfo(path string) *RequestInfo {
	return &RequestInfo{
		Method: http.MethodGet,
		Status: http.StatusOK,
		Path:   path,
	}
}

func New(config configs.PrometheusConfig) Metrics {
	registry := prometheus.NewRegistry()

	m := &appMetrics{
		registry: registry,
		stand:    config.Stand,
		incomingRequests: prometheus.NewSummaryVec(prometheus.SummaryOpts{
			Namespace: "jocasta",
			Subsystem: "pipeliner",
			Name:      incomingRequests,
		}, []string{"method", "stand", "path", "pipeline_id", "version_id", "client_id", "work_number", "status"}),
		kafkaAvailability: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "jocasta",
			Subsystem: "pipeliner",
			Help:      "Indicates whether Kafka is available(1) or not(0)",
			Name:      kafkaAvailability,
		}),
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
		m.kafkaAvailability,
	)
}

func (m *appMetrics) RequestsIncrease(label *RequestInfo) {
	m.incomingRequests.WithLabelValues([]string{
		label.Method,
		m.stand,
		label.Path,
		label.PipelineID,
		label.VersionID,
		label.ClientID,
		label.WorkNumber,
		strconv.Itoa(label.Status),
	}...).Observe(label.Duration.Seconds())
}

func (m *appMetrics) KafkaAvailable() {
	m.kafkaAvailability.Set(1)
}

func (m *appMetrics) KafkaUnavailable() {
	m.kafkaAvailability.Set(0)
}
