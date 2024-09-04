package metrics

import (
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	namespace = "jocasta"
	subsystem = "pipeliner"

	incomingRequests            = "incoming_requests"
	request2ExternalSystem      = "request_2_external_system"
	externalSystemAvailability  = "external_system_availability"
	incomingRequestsTotal       = "incoming_requests_total"
	externalSystemRequestsTotal = "external_system_requests_total"
)

type service struct {
	registry *prometheus.Registry
	stand    string

	externalSystemAvailability *prometheus.GaugeVec

	incomingRequestsTotal       *prometheus.HistogramVec
	externalSystemRequestsTotal *prometheus.HistogramVec

	incomingRequests       *prometheus.SummaryVec
	request2ExternalSystem *prometheus.SummaryVec
}

func New(config PrometheusConfig) Metrics {
	registry := prometheus.NewRegistry()

	m := &service{
		registry: registry,
		stand:    config.Stand,
		incomingRequests: prometheus.NewSummaryVec(prometheus.SummaryOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      incomingRequests,
		}, []string{"method", "stand", "path", "pipeline_id", "version_id", "client_id", "work_number", "status"}),
		request2ExternalSystem: prometheus.NewSummaryVec(prometheus.SummaryOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      request2ExternalSystem,
		}, []string{"method", "stand", "url", "integration_name", "response_code", "trace_id"}),
		incomingRequestsTotal: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      incomingRequestsTotal,
			Help:      "Duration of incoming requests in seconds",
			Buckets:   prometheus.DefBuckets,
		}, []string{"method", "path", "http_status"}),
		externalSystemRequestsTotal: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      externalSystemRequestsTotal,
			Help:      "Duration of requests to external systems in seconds",
			Buckets:   prometheus.DefBuckets,
		}, []string{"method", "path", "http_status", "integration_name"}),
		externalSystemAvailability: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Help:      "Indicates whether service is available(1) or not(0)",
			Name:      externalSystemAvailability,
		}, []string{"integration_name"}),
	}

	m.MustRegisterMetrics(registry)

	return m
}

func (m *service) ServePrometheus() http.Handler {
	return http.HandlerFunc(m.handleMetricsRequest)
}

func (m *service) handleMetricsRequest(w http.ResponseWriter, r *http.Request) {
	h := promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)
}

func (m *service) MustRegisterMetrics(registry *prometheus.Registry) {
	registry.MustRegister(
		m.incomingRequests,
		m.request2ExternalSystem,
		m.externalSystemRequestsTotal,
		m.incomingRequestsTotal,
		m.externalSystemAvailability,
	)
}

func (m *service) Request2ExternalSystem(label *ExternalRequestInfo) {
	m.request2ExternalSystem.WithLabelValues([]string{
		label.Method,
		m.stand,
		label.URL,
		label.ExternalSystem,
		strconv.Itoa(label.ResponseCode),
		label.TraceID,
	}...).Observe(label.Duration.Seconds())

	//nolint:errcheck //url must be relevant
	parsedURL, _ := url.Parse(label.URL)

	m.externalSystemRequestsTotal.With(prometheus.Labels{
		"method":           label.Method,
		"integration_name": label.ExternalSystem,
		"path":             parsedURL.Path,
		"http_status":      strconv.Itoa(label.ResponseCode),
	}).Observe(label.Duration.Seconds())
}

func (m *service) RequestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		wrappedRespWriter := middleware.NewWrapResponseWriter(writer, 0)

		start := time.Now()

		next.ServeHTTP(wrappedRespWriter, request)

		duration := time.Since(start)

		var path string
		if routeContext := chi.RouteContext(request.Context()); routeContext != nil {
			path = routeContext.RoutePattern()
		}

		// empty handlers like '/alive' doesn't call Write() and WriteHeader() methods
		status := wrappedRespWriter.Status()
		if status == 0 {
			status = http.StatusOK
		}

		m.incomingRequestsTotal.With(prometheus.Labels{
			"method":      request.Method,
			"path":        path,
			"http_status": strconv.Itoa(status),
		}).Observe(duration.Seconds())
	})
}

func (m *service) RequestsIncrease(label *RequestInfo) {
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

func (m *service) KafkaAvailable() {
	m.externalSystemAvailability.With(map[string]string{
		"integration_name": "kafka",
	}).Set(1)
}

func (m *service) KafkaUnavailable() {
	m.externalSystemAvailability.With(map[string]string{
		"integration_name": "kafka",
	}).Set(0)
}

func (m *service) SchedulerAvailable() {
	m.externalSystemAvailability.With(map[string]string{
		"integration_name": "scheduler",
	}).Set(1)
}

func (m *service) SchedulerUnavailable() {
	m.externalSystemAvailability.With(map[string]string{
		"integration_name": "scheduler",
	}).Set(0)
}

func (m *service) FileRegistryAvailable() {
	m.externalSystemAvailability.With(map[string]string{
		"integration_name": "file-registry",
	}).Set(1)
}

func (m *service) FileRegistryUnavailable() {
	m.externalSystemAvailability.With(map[string]string{
		"integration_name": "file-registry",
	}).Set(0)
}

func (m *service) HumanTasksAvailable() {
	m.externalSystemAvailability.With(map[string]string{
		"integration_name": "human-tasks",
	}).Set(1)
}

func (m *service) HumanTasksUnavailable() {
	m.externalSystemAvailability.With(map[string]string{
		"integration_name": "human-tasks",
	}).Set(0)
}

func (m *service) FunctionStoreAvailable() {
	m.externalSystemAvailability.With(map[string]string{
		"integration_name": "function-store",
	}).Set(1)
}

func (m *service) FunctionStoreUnavailable() {
	m.externalSystemAvailability.With(map[string]string{
		"integration_name": "function-store",
	}).Set(0)
}

func (m *service) ServiceDescAvailable() {
	m.externalSystemAvailability.With(map[string]string{
		"integration_name": "servicedesk",
	}).Set(1)
}

func (m *service) ServiceDescUnavailable() {
	m.externalSystemAvailability.With(map[string]string{
		"integration_name": "servicedesk",
	}).Set(0)
}

func (m *service) PeopleAvailable() {
	m.externalSystemAvailability.With(map[string]string{
		"integration_name": "iga",
	}).Set(1)
}

func (m *service) PeopleStoreUnavailable() {
	m.externalSystemAvailability.With(map[string]string{
		"integration_name": "iga",
	}).Set(0)
}

func (m *service) MailAvailable() {
	m.externalSystemAvailability.With(map[string]string{
		"integration_name": "mail.inside",
	}).Set(1)
}

func (m *service) MailUnavailable() {
	m.externalSystemAvailability.With(map[string]string{
		"integration_name": "mail.inside",
	}).Set(0)
}

func (m *service) IntegrationsAvailable() {
	m.externalSystemAvailability.With(map[string]string{
		"integration_name": "integrations",
	}).Set(1)
}

func (m *service) IntegrationsUnavailable() {
	m.externalSystemAvailability.With(map[string]string{
		"integration_name": "integrations",
	}).Set(0)
}

func (m *service) HrGateAvailable() {
	m.externalSystemAvailability.With(map[string]string{
		"integration_name": "hrGate",
	}).Set(1)
}

func (m *service) HrGateUnavailable() {
	m.externalSystemAvailability.With(map[string]string{
		"integration_name": "hrGate",
	}).Set(0)
}

func (m *service) SequenceAvailable() {
	m.externalSystemAvailability.With(map[string]string{
		"integration_name": "sequence",
	}).Set(1)
}

func (m *service) SequenceUnavailable() {
	m.externalSystemAvailability.With(map[string]string{
		"integration_name": "sequence",
	}).Set(0)
}

func (m *service) DBAvailable() {
	m.externalSystemAvailability.With(map[string]string{
		"integration_name": "db",
	}).Set(1)
}

func (m *service) DBUnavailable() {
	m.externalSystemAvailability.With(map[string]string{
		"integration_name": "db",
	}).Set(0)
}
