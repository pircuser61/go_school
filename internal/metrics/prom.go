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

type RemedyStatus struct {
	Ok   prometheus.Gauge
	Fail prometheus.Gauge
}

type Statistic struct {
	Requests     prometheus.Counter
	NGSAPushes   NGSAStatus
	RemedyPushes RemedyStatus
}

//nolint:gochecknoglobals //its good
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
}
