package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus/push"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	TagKek = "kek"
)

type NGSAStatus struct {
	Ok   prometheus.Gauge
	Fail prometheus.Gauge
}

type Statistic struct {
	Requests   prometheus.Counter
	NGSAPushes NGSAStatus
}

var (
	Stats    = Statistic{}
	once     = &sync.Once{}
	Registry = prometheus.NewRegistry()
	Pusher   *push.Pusher
)

func InitMetricsAuth() {
	once.Do(func() {
		Stats.Requests = prometheus.NewCounter(prometheus.CounterOpts{
			Name: "requests_count",
			Help: "count of requests",
		})

		prometheus.MustRegister(Stats.Requests)

		Stats.NGSAPushes.Ok = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "ngsa_ok",
			Help: "time and status of last success NGSA push"})

		Stats.NGSAPushes.Fail = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "ngsa_fail",
			Help: "time and status of last not success NGSA push"})

		Registry.MustRegister(Stats.NGSAPushes.Ok)
		Registry.MustRegister(Stats.NGSAPushes.Fail)
	})
}
