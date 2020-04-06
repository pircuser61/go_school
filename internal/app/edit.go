package app

import (
	"context"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
	"gitlab.services.mts.ru/erius/pipeliner/internal/db"
	"go.opencensus.io/trace"
	"io/ioutil"
	"net/http"
)

func (p Pipeliner) EditPipeline(w http.ResponseWriter, req *http.Request) {
	c, s := trace.StartSpan(context.Background(), "edit_pipeline")
	defer s.End()

	idparam := chi.URLParam(req, "id")
	b, err := ioutil.ReadAll(req.Body)
	defer req.Body.Close()
	if err != nil {
		p.Logger.Error("can't get pipeline from request body", err)
		sendError(w, err)
		return
	}
	id, err := uuid.Parse(idparam)
	if err != nil {
		p.Logger.Error("can't parse id", err)
		sendError(w, err)
		return
	}
	err = db.EditPipeline(c, p.DBConnection, id, b)
	if err != nil {
		p.Logger.Error("can't add pipeline to db", err)
		sendError(w, err)
		return
	}
	err = sendResponse(w, 200, nil)
	if err != nil {
		p.Logger.Error("can't send response", err)
		sendError(w, err)
		return
	}
}
