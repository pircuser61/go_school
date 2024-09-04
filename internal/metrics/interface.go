package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
)

type Metrics interface {
	ServePrometheus() http.Handler
	MustRegisterMetrics(registry *prometheus.Registry)

	KafkaAvailable()
	KafkaUnavailable()

	DBAvailable()
	DBUnavailable()

	SchedulerAvailable()
	SchedulerUnavailable()

	FileRegistryAvailable()
	FileRegistryUnavailable()

	HumanTasksAvailable()
	HumanTasksUnavailable()

	FunctionStoreAvailable()
	FunctionStoreUnavailable()

	ServiceDescAvailable()
	ServiceDescUnavailable()

	PeopleAvailable()
	PeopleStoreUnavailable()

	MailAvailable()
	MailUnavailable()

	IntegrationsAvailable()
	IntegrationsUnavailable()

	HrGateAvailable()
	HrGateUnavailable()

	SequenceAvailable()
	SequenceUnavailable()

	RequestsIncrease(label *RequestInfo)
	Request2ExternalSystem(label *ExternalRequestInfo)

	RequestMiddleware(next http.Handler) http.Handler
}
