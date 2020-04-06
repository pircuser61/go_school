package app

import (
	"context"
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
		sendError(w, err)
		return
	}
	err = sendResponse(w, http.StatusOK, pipelines)
	if err != nil {
		p.Logger.Error("can't send response", err)
		return
	}

}
