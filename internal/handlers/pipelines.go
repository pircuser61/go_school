package handlers

import (
	"context"
	"encoding/json"
	"go.opencensus.io/trace"
	"io/ioutil"
	"net/http"
)

func (ae ApiEnv) ListPipelines(w http.ResponseWriter, req *http.Request){
	_, s := trace.StartSpan(context.Background(), "list_pipelines")
	defer s.End()
	err := sendResponse(w, http.StatusOK, nil)
	if err != nil {
		ae.Logger.Error("can't send response", err)
		return
	}
}


func (ae ApiEnv) GetPipeline(w http.ResponseWriter, req *http.Request){
	_, s := trace.StartSpan(context.Background(), "list_pipelines")
	defer s.End()
	err := sendResponse(w, http.StatusOK, nil)
	if err != nil {
		ae.Logger.Error("can't send response", err)
		return
	}
}


func (ae ApiEnv) GetPipelineVersion(w http.ResponseWriter, req *http.Request){
	_, s := trace.StartSpan(context.Background(), "list_pipelines")
	defer s.End()
	err := sendResponse(w, http.StatusOK, nil)
	if err != nil {
		ae.Logger.Error("can't send response", err)
		return
	}
}


func (ae ApiEnv) CreateDraft(w http.ResponseWriter, req *http.Request){
	_, s := trace.StartSpan(context.Background(), "list_pipelines")
	defer s.End()
	err := sendResponse(w, http.StatusOK, nil)
	if err != nil {
		ae.Logger.Error("can't send response", err)
		return
	}
}


func (ae ApiEnv) EditDraft(w http.ResponseWriter, req *http.Request){
	_, s := trace.StartSpan(context.Background(), "list_pipelines")
	defer s.End()
	err := sendResponse(w, http.StatusOK, nil)
	if err != nil {
		ae.Logger.Error("can't send response", err)
		return
	}
}


func (ae ApiEnv) DeleteDraft(w http.ResponseWriter, req *http.Request){
	_, s := trace.StartSpan(context.Background(), "list_pipelines")
	defer s.End()
	err := sendResponse(w, http.StatusOK, nil)
	if err != nil {
		ae.Logger.Error("can't send response", err)
		return
	}
}


func (ae ApiEnv) DeletePipeline(w http.ResponseWriter, req *http.Request){
	_, s := trace.StartSpan(context.Background(), "list_pipelines")
	defer s.End()
	err := sendResponse(w, http.StatusOK, nil)
	if err != nil {
		ae.Logger.Error("can't send response", err)
		return
	}
}

func (ae ApiEnv) CreatePipeline(w http.ResponseWriter, req *http.Request){
	_, s := trace.StartSpan(context.Background(), "list_pipelines")
	defer s.End()

	b, err := ioutil.ReadAll(req.Body)
	defer req.Body.Close()
	if err != nil {
		ae.Logger.Error("can't get pipeline from request body ", err)
		return
	}

	p := Pipeline{}
	err = json.Unmarshal(b, &p)
	if err != nil {
		ae.Logger.Error("can't unmarshal pipeline ", err)
		sendError(w, err)
		return
	}
	err = sendResponse(w, http.StatusOK, nil)
	if err != nil {
		ae.Logger.Error("can't send response", err)
		return
	}
}


func (ae ApiEnv) RunPipeline(w http.ResponseWriter, req *http.Request){
	_, s := trace.StartSpan(context.Background(), "list_pipelines")
	defer s.End()
	err := sendResponse(w, http.StatusOK, nil)
	if err != nil {
		ae.Logger.Error("can't send response", err)
		return
	}
}