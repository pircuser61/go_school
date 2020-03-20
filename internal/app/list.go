package app

import (
	"context"
	"encoding/json"
	"gitlab.services.mts.ru/erius/pipeliner/internal/db"
	"go.opencensus.io/trace"
	"net/http"
)

func (p Pipeliner) ListPipelines(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(context.Background(), "list_pipelines")
	defer s.End()
	pipelines, err := db.ListPipelines(ctx, p.DBConnection)
	if err != nil {
		p.Logger.Error("can't get pipelines from DB", err)
		return
	}
	b, err := json.Marshal(pipelines)
	if err != nil {
		p.Logger.Error("can't marshal pipelines", err)
		return
	}
	_, err = w.Write(b)
	if err != nil {
		p.Logger.Error("can't write pipelines to request", err)
		return
	}
}
