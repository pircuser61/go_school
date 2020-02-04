package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	TagKek = "kek"
)

type Statistic struct {
	Requests prometheus.Counter
}

var (
	Stats = Statistic{}
	once  = &sync.Once{}
)

func InitMetricsAuth() {
	once.Do(func() {
		Stats.Requests = prometheus.NewCounter(prometheus.CounterOpts{
			Name: "requests_count",
			Help: "count of requests",
		})

		prometheus.MustRegister(Stats.Requests)
	})
}
