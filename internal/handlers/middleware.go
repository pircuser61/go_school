package handlers

import (
	"net/http"

	"gitlab.services.mts.ru/abp/myosotis/logger"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/trace"

	"github.com/google/uuid"
)

const XRequestIDHeader = "X-Request-Id"

func RequestIDMiddleware(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get(XRequestIDHeader)
		if reqID == "" {
			reqID = uuid.New().String()
			w.Header().Set(XRequestIDHeader, reqID)
			r.Header.Set(XRequestIDHeader, reqID)
		}

		next.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}

func LoggerMiddleware(log logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return ochttp.Handler{
			Handler: http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
				ctxLocal, span := trace.StartSpan(req.Context(), req.Method+" "+req.URL.String())
				defer span.End()

				newLogger := log.WithFields(map[string]interface{}{
					"method": req.Method,
					"url":    req.URL.String(),
					"host":   req.Host,
				})

				newLogger.Info("request")
				ctx := logger.WithLogger(ctxLocal, newLogger)

				next.ServeHTTP(res, req.WithContext(ctx))
			}),
		}.Handler
	}
}
