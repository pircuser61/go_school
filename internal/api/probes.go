package api

import (
	"context"
	"net/http"
	"time"
)

func (ae *Env) Alive(_ http.ResponseWriter, _ *http.Request) {}

func (ae *Env) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithDeadline(r.Context(), time.Now().Add(2*time.Second))
	defer cancel()

	if err := ae.DB.Ping(ctx); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
