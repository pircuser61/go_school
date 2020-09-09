package handlers

import (
	"net/http"

	"github.com/google/uuid"
)

const XRequestIDHeader = "X-Request-Id"

func SetRequestID(next http.Handler) http.Handler {
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
