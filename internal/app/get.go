package app

import (
	"context"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
	"gitlab.services.mts.ru/erius/pipeliner/internal/db"
	"go.opencensus.io/trace"
	"net/http"
)

func (p Pipeliner) GetPipeline(w http.ResponseWriter, req *http.Request) {
	c, s := trace.StartSpan(context.Background(), "get_pipeline")
	defer s.End()

	idparam := chi.URLParam(req, "id")
	id, err := uuid.Parse(idparam)
	if err != nil {
		p.Logger.Error("can't parse id", err)
		sendError(w, err)
		return
	}
	pipe, err := db.GetPipeline(c, p.DBConnection, id)
	if err != nil {
		p.Logger.Error("can't add pipeline to db", err)
		sendError(w, err)
		return
	}

	err = sendResponse(w, 200, pipe)
}
