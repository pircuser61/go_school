package metrics

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	namespace = "jocasta"
	subsystem = "pipeliner"

	incomingRequests           = "incoming_requests"
	kafkaAvailability          = "kafka_availability"
	schedulerAvailability      = "scheduler_availability"
	fileRegistryAvailability   = "file_registry_availability"
	humanTasksAvailability     = "human_tasks_availability"
	functionStoreAvailability  = "function_store_availability"
	serviceDescAvailability    = "service_desc_availability"
	peopleAvailability         = "people_availability"
	mailAvailability           = "mail_availability"
	integrationsAvailability   = "integrations_availability"
	hrGateAvailability         = "hrGate_availability"
	sequenceAvailability       = "sequence_availability"
	dbAvailability             = "db_availability"
	request2ExternalSystem     = "request_2_external_system"
	incomingRequestsCount      = "incoming_requests_count"
	externalSystemRequestCount = "external_system_requests_count"
)

type service struct {
	registry *prometheus.Registry
	stand    string

	kafkaAvailability         prometheus.Gauge
	dbAvailability            prometheus.Gauge
	schedulerAvailability     prometheus.Gauge
	fileRegistryAvailability  prometheus.Gauge
	humanTasksAvailability    prometheus.Gauge
	functionStoreAvailability prometheus.Gauge
	serviceDescAvailability   prometheus.Gauge
	peopleAvailability        prometheus.Gauge
	mailAvailability          prometheus.Gauge
	integrationsAvailability  prometheus.Gauge
	hrGateAvailability        prometheus.Gauge
	sequenceAvailability      prometheus.Gauge

	incomingRequestsCount       *prometheus.CounterVec
	externalSystemRequestsCount *prometheus.CounterVec

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
		kafkaAvailability: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Help:      "Indicates whether Kafka is available(1) or not(0)",
			Name:      kafkaAvailability,
		}),
		request2ExternalSystem: prometheus.NewSummaryVec(prometheus.SummaryOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      request2ExternalSystem,
		}, []string{"method", "stand", "url", "integration_name", "response_code", "trace_id"}),
		dbAvailability: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Help:      "Indicates whether service is available(1) or not(0)",
			Name:      dbAvailability,
		}),
		schedulerAvailability: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Help:      "Indicates whether service is available(1) or not(0)",
			Name:      schedulerAvailability,
		}),
		fileRegistryAvailability: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Help:      "Indicates whether service is available(1) or not(0)",
			Name:      fileRegistryAvailability,
		}),
		humanTasksAvailability: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Help:      "Indicates whether service is available(1) or not(0)",
			Name:      humanTasksAvailability,
		}),
		functionStoreAvailability: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Help:      "Indicates whether service is available(1) or not(0)",
			Name:      functionStoreAvailability,
		}),
		serviceDescAvailability: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Help:      "Indicates whether service is available(1) or not(0)",
			Name:      serviceDescAvailability,
		}),
		peopleAvailability: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Help:      "Indicates whether service is available(1) or not(0)",
			Name:      peopleAvailability,
		}),
		mailAvailability: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Help:      "Indicates whether service is available(1) or not(0)",
			Name:      mailAvailability,
		}),
		integrationsAvailability: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Help:      "Indicates whether service is available(1) or not(0)",
			Name:      integrationsAvailability,
		}),
		hrGateAvailability: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Help:      "Indicates whether service is available(1) or not(0)",
			Name:      hrGateAvailability,
		}),
		sequenceAvailability: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Help:      "Indicates whether service is available(1) or not(0)",
			Name:      sequenceAvailability,
		}),
		incomingRequestsCount: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      incomingRequestsCount,
			Help:      "Total number of incoming requests",
		}, []string{"method", "stand", "path", "http_status"}),
		externalSystemRequestsCount: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      externalSystemRequestCount,
			Help:      "Total number of requests to external systems",
		}, []string{"method", "stand", "path", "http_status", "service"}),
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
		m.kafkaAvailability,
		m.request2ExternalSystem,
		m.dbAvailability,
		m.schedulerAvailability,
		m.fileRegistryAvailability,
		m.humanTasksAvailability,
		m.functionStoreAvailability,
		m.serviceDescAvailability,
		m.peopleAvailability,
		m.mailAvailability,
		m.integrationsAvailability,
		m.hrGateAvailability,
		m.sequenceAvailability,
		m.externalSystemRequestsCount,
		m.incomingRequestsCount,
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

	m.externalSystemRequestsCount.With(prometheus.Labels{
		"method":      label.Method,
		"stand":       m.stand,
		"service":     label.ExternalSystem,
		"path":        parsedURL.Path,
		"http_status": strconv.Itoa(label.ResponseCode),
	}).Inc()
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (writer *loggingResponseWriter) WriteHeader(statusCode int) {
	writer.status = statusCode
	writer.ResponseWriter.WriteHeader(statusCode)
}

func (writer *loggingResponseWriter) Write(p []byte) (int, error) {
	if writer.status == 0 {
		writer.status = http.StatusOK
	}

	return writer.ResponseWriter.Write(p)
}

func (m *service) IncomingRequestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		wrappedRespWriter := &loggingResponseWriter{
			ResponseWriter: writer,
		}

		next.ServeHTTP(wrappedRespWriter, request)

		m.incomingRequestsCount.With(prometheus.Labels{
			"method":      request.Method,
			"stand":       m.stand,
			"path":        request.URL.Path,
			"http_status": strconv.Itoa(wrappedRespWriter.status),
		}).Inc()
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
	m.kafkaAvailability.Set(1)
}

func (m *service) KafkaUnavailable() {
	m.kafkaAvailability.Set(0)
}

func (m *service) SchedulerAvailable() {
	m.schedulerAvailability.Set(1)
}

func (m *service) SchedulerUnavailable() {
	m.schedulerAvailability.Set(0)
}

func (m *service) FileRegistryAvailable() {
	m.fileRegistryAvailability.Set(1)
}

func (m *service) FileRegistryUnavailable() {
	m.fileRegistryAvailability.Set(0)
}

func (m *service) HumanTasksAvailable() {
	m.humanTasksAvailability.Set(1)
}

func (m *service) HumanTasksUnavailable() {
	m.humanTasksAvailability.Set(0)
}

func (m *service) FunctionStoreAvailable() {
	m.functionStoreAvailability.Set(1)
}

func (m *service) FunctionStoreUnavailable() {
	m.functionStoreAvailability.Set(0)
}

func (m *service) ServiceDescAvailable() {
	m.serviceDescAvailability.Set(1)
}

func (m *service) ServiceDescUnavailable() {
	m.serviceDescAvailability.Set(0)
}

func (m *service) PeopleAvailable() {
	m.peopleAvailability.Set(1)
}

func (m *service) PeopleStoreUnavailable() {
	m.peopleAvailability.Set(0)
}

func (m *service) MailAvailable() {
	m.mailAvailability.Set(1)
}

func (m *service) MailUnavailable() {
	m.mailAvailability.Set(0)
}

func (m *service) IntegrationsAvailable() {
	m.integrationsAvailability.Set(1)
}

func (m *service) IntegrationsUnavailable() {
	m.integrationsAvailability.Set(0)
}

func (m *service) HrGateAvailable() {
	m.hrGateAvailability.Set(1)
}

func (m *service) HrGateUnavailable() {
	m.hrGateAvailability.Set(0)
}

func (m *service) SequenceAvailable() {
	m.sequenceAvailability.Set(1)
}

func (m *service) SequenceUnavailable() {
	m.sequenceAvailability.Set(0)
}

func (m *service) DBAvailable() {
	m.dbAvailability.Set(1)
}

func (m *service) DBUnavailable() {
	m.dbAvailability.Set(0)
}
