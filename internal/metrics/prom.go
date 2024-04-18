package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
)

type NGSAStatus struct {
	Ok   prometheus.Gauge
	Fail prometheus.Gauge
}

type RemedyStatus struct {
	Ok   prometheus.Gauge
	Fail prometheus.Gauge
}

type Statistic struct {
	NGSAPushes   NGSAStatus
	RemedyPushes RemedyStatus

	RequestCount   *prometheus.CounterVec
	ResponseStatus *prometheus.CounterVec
	HTTPDuration   *prometheus.HistogramVec
}

//nolint:gochecknoglobals //its good
var (
	Stats    = Statistic{}
	once     = &sync.Once{}
	Registry = prometheus.NewRegistry()
	Pusher   *push.Pusher
)

func InitMetricsAuth(config PrometheusConfig) {
	once.Do(func() {
		Stats.NGSAPushes.Ok = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "ngsa_push_ok_unixtime",
			Help: "Last time success push to NGSA",
		})

		Stats.NGSAPushes.Fail = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "ngsa_push_fail_unixtime",
			Help: "Last time not success push to NGSA",
		})

		Stats.RemedyPushes.Ok = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "remedy_push_ok_unixtime",
			Help: "Last time sending remedy update succeed",
		})

		Stats.RemedyPushes.Fail = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "remedy_push_fail_unixtime",
			Help: "Last time sending remedy update failed",
		})

		Registry.MustRegister(Stats.NGSAPushes.Ok)
		Registry.MustRegister(Stats.NGSAPushes.Fail)

		Registry.MustRegister(Stats.RemedyPushes.Ok)
		Registry.MustRegister(Stats.RemedyPushes.Fail)
	})

	Pusher = push.New(config.Push.URL, config.Push.Job).Gatherer(Registry)
}
