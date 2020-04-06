package app

import (
	"context"
	"gitlab.services.mts.ru/erius/pipeliner/internal/db"
	"gitlab.services.mts.ru/erius/pipeliner/internal/model"
	"go.opencensus.io/trace"
	"io/ioutil"
	"net/http"
)

func (p Pipeliner) AddPipeline(w http.ResponseWriter, req *http.Request) {
	c, s := trace.StartSpan(context.Background(), "add_pipeline")
	defer s.End()
	b, err := ioutil.ReadAll(req.Body)
	defer req.Body.Close()
	if err != nil {
		p.Logger.Error("can't get pipeline from request body", err)
		return
	}

	pipe, err := model.NewPipeline(db.PipelineStorageModel{Pipeline: string(b)}, p.DBConnection)
	if err != nil {
		p.Logger.Error("can't unmarshal pipeline: ", err)
		sendError(w, err)
		return
	}

	err = db.AddPipeline(c, p.DBConnection, pipe.Name, b)
	if err != nil {
		p.Logger.Error("can't add pipeline to db: ", err)
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
