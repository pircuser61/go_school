package handlers

import (
	"context"
	"encoding/json"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
	"gitlab.services.mts.ru/erius/pipeliner/internal/db"
	"gitlab.services.mts.ru/erius/pipeliner/internal/pipeline"
	"go.opencensus.io/trace"
	"io/ioutil"
	"net/http"
)

type RunContext struct {
	ID         string            `json:"id"`
	Parameters map[string]string `json:"parameters"`
}

func (ae ApiEnv) RunPipelineDepricated(w http.ResponseWriter, req *http.Request) {
	c, s := trace.StartSpan(context.Background(), "add_pipeline")
	defer s.End()
	b, err := ioutil.ReadAll(req.Body)
	defer req.Body.Close()
	if err != nil {
		ae.Logger.Error("can't get pipelineRunneble from request body", err)
		return
	}
	rc := RunContext{}
	err = json.Unmarshal(b, &rc)
	if err != nil {
		ae.Logger.Error("can't add pipelineRunneble to db ", err)
		sendError(w, err)
		return
	}
	pipe, err := db.GetPipelineByName(c, ae.DBConnection, rc.ID)
	if err != nil {
		ae.Logger.Error("can't get pipelineRunneble from db ", err)
		sendError(w, err)
		return
	}
	pipelineRunneble, err := pipeline.NewPipeline(*pipe, ae.DBConnection)
	if err != nil {
		ae.Logger.Error("can't create pipelineRunneble ", err)
		sendError(w, err)
		return
	}

	inputVals := pipeline.NewStore()
	for name, value := range rc.Parameters {
		inputVals.SetValue(name, value)
	}
	err = pipelineRunneble.Run(c, &inputVals)
	if err != nil {
		ae.Logger.Error("can't run pipelineRunneble ", err)
		sendError(w, err)
		return
	}
	out, err := pipelineRunneble.ReturnOutput()
	if err != nil {
		ae.Logger.Error("can't get pipelineRunneble result ", err)
		sendError(w, err)
		return
	}
	err = sendResponse(w, http.StatusOK, out)
	if err != nil {
		ae.Logger.Error("can't return pipelineRunneble result ", err)
		sendError(w, err)
		return
	}
}

func (ae ApiEnv) AddPipelineDepricated(w http.ResponseWriter, req *http.Request) {
	c, s := trace.StartSpan(context.Background(), "add_pipeline")
	defer s.End()
	b, err := ioutil.ReadAll(req.Body)
	defer req.Body.Close()
	if err != nil {
		ae.Logger.Error("can't get pipeline from request body", err)
		return
	}

	pipe, err := pipeline.NewPipeline(db.PipelineStorageModel{Pipeline: string(b)}, ae.DBConnection)
	if err != nil {
		ae.Logger.Error("can't unmarshal pipeline: ", err)
		sendError(w, err)
		return
	}

	err = db.AddPipeline(c, ae.DBConnection, pipe.Name, b)
	if err != nil {
		ae.Logger.Error("can't add pipeline to db: ", err)
		sendError(w, err)
		return
	}
	err = sendResponse(w, 200, nil)
	if err != nil {
		ae.Logger.Error("can't send response", err)
		sendError(w, err)
		return
	}
}

func (ae ApiEnv) EditPipelineDepricated(w http.ResponseWriter, req *http.Request) {
	c, s := trace.StartSpan(context.Background(), "edit_pipeline")
	defer s.End()

	idparam := chi.URLParam(req, "id")
	b, err := ioutil.ReadAll(req.Body)
	defer req.Body.Close()
	if err != nil {
		ae.Logger.Error("can't get pipeline from request body", err)
		sendError(w, err)
		return
	}
	id, err := uuid.Parse(idparam)
	if err != nil {
		ae.Logger.Error("can't parse id", err)
		sendError(w, err)
		return
	}
	err = db.EditPipeline(c, ae.DBConnection, id, b)
	if err != nil {
		ae.Logger.Error("can't add pipeline to db", err)
		sendError(w, err)
		return
	}
	err = sendResponse(w, 200, nil)
	if err != nil {
		ae.Logger.Error("can't send response", err)
		sendError(w, err)
		return
	}
}

func (ae ApiEnv) GetPipelineDepricated(w http.ResponseWriter, req *http.Request) {
	c, s := trace.StartSpan(context.Background(), "get_pipeline")
	defer s.End()

	idparam := chi.URLParam(req, "id")
	id, err := uuid.Parse(idparam)
	if err != nil {
		ae.Logger.Error("can't parse id", err)
		sendError(w, err)
		return
	}
	pipe, err := db.GetPipeline(c, ae.DBConnection, id)
	if err != nil {
		ae.Logger.Error("can't add pipeline to db", err)
		sendError(w, err)
		return
	}

	err = sendResponse(w, 200, pipe)
}

func (ae ApiEnv) ListPipelinesDepricated(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(context.Background(), "list_pipelines")
	defer s.End()
	pipelines, err := db.ListPipelines(ctx, ae.DBConnection)
	if err != nil {
		ae.Logger.Error("can't get pipelines from DB", err)
		sendError(w, err)
		return
	}
	err = sendResponse(w, http.StatusOK, pipelines)
	if err != nil {
		ae.Logger.Error("can't send response", err)
		return
	}

}
