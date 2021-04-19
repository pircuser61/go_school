package statistic

import (
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
)

func responseStatus() *prometheus.CounterVec {
	return prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "response_status",
		Help: "Status of HTTP response",
	},
		[]string{"status"},
	)
}

func httpDuration() *prometheus.HistogramVec {
	return prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "http_response_time_sec",
		Help: "Duration of HTTP requests.",
	}, []string{"path"})
}

func requestCount() *prometheus.CounterVec {
	return prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "requests_count",
		Help: "request count",
	}, []string{"method", "path"})
}

type Statistic struct {
	RequestCount   *prometheus.CounterVec
	ResponseStatus *prometheus.CounterVec
	HTTPDuration   *prometheus.HistogramVec
}

func InitStatistic() (*Statistic, error) {
	stats := &Statistic{
		RequestCount:   requestCount(),
		ResponseStatus: responseStatus(),
		HTTPDuration:   httpDuration(),
	}

	if err := prometheus.Register(stats.ResponseStatus); err != nil {
		return nil, errors.WithMessage(err, "duplicate response status metric registration")
	}

	if err := prometheus.Register(stats.HTTPDuration); err != nil {
		return nil, errors.WithMessage(err, "duplicate http duration metric registration")
	}

	if err := prometheus.Register(stats.RequestCount); err != nil {
		return nil, errors.WithMessage(err, "duplicate response status metric registration")
	}

	return stats, nil
}
