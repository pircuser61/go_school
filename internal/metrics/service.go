package metrics

import (
	"net/http"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	namespace = "jocasta"
	subsystem = "pipeliner"

	incomingRequests       = "incoming_requests"
	kafkaAvailability      = "kafka_availability"
	request2ExternalSystem = "request_2_external_system"
)

type service struct {
	registry *prometheus.Registry
	stand    string

	incomingRequests *prometheus.SummaryVec

	kafkaAvailability         prometheus.Gauge
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
	m.sequenceAvailability.Set(1)
}

func (m *service) DBUnavailable() {
	m.sequenceAvailability.Set(0)
}
