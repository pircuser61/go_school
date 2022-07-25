package api

import (
	"net/http"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"

	"gitlab.services.mts.ru/jocasta/pipeliner/statistic"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{w, http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func StatisticMiddleware(stat *statistic.Statistic) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path

			timer := prometheus.NewTimer(stat.HTTPDuration.WithLabelValues(path))

			rw := newResponseWriter(w)
			next.ServeHTTP(rw, r)

			statusCode := rw.statusCode

			stat.ResponseStatus.WithLabelValues(strconv.Itoa(statusCode)).Inc()

			stat.RequestCount.WithLabelValues(r.Method, path).Inc()

			timer.ObserveDuration()
		}

		return http.HandlerFunc(fn)
	}
}
