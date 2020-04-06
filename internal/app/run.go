package app

import (
	"context"
	"encoding/json"
	"gitlab.services.mts.ru/erius/pipeliner/internal/db"
	"gitlab.services.mts.ru/erius/pipeliner/internal/model"
	"go.opencensus.io/trace"
	"io/ioutil"
	"net/http"
)

type RunContext struct {
	Id         string            `json:"id"`
	Parameters map[string]string `json:"parameters"`
}

func (p Pipeliner) RunPipeline(w http.ResponseWriter, req *http.Request) {
	c, s := trace.StartSpan(context.Background(), "add_pipeline")
	defer s.End()
	b, err := ioutil.ReadAll(req.Body)
	defer req.Body.Close()
	if err != nil {
		p.Logger.Error("can't get pipeline from request body", err)
		return
	}
	rc := RunContext{}
	err = json.Unmarshal(b, &rc)
	if err != nil {
		p.Logger.Error("can't add pipeline to db ", err)
		sendError(w, err)
		return
	}
	pipe, err := db.GetPipelineByName(c, p.DBConnection, rc.Id)
	if err != nil {
		p.Logger.Error("can't get pipeline from db ", err)
		sendError(w, err)
		return
	}
	pipeline, err := model.NewPipeline(*pipe, p.DBConnection)
	if err != nil {
		p.Logger.Error("can't create pipeline ", err)
		sendError(w, err)
		return
	}

	inputVals := model.NewContext()
	for name, value := range rc.Parameters {
		inputVals.SetValue(name, value)
	}
	err = pipeline.Run(c, &inputVals)
	if err != nil {
		p.Logger.Error("can't run pipeline ", err)
		sendError(w, err)
		return
	}
	out, err := pipeline.ReturnOutput()
	if err != nil {
		p.Logger.Error("can't get pipeline result ", err)
		sendError(w, err)
		return
	}
	err = sendResponse(w, http.StatusOK, out)
	if err != nil {
		p.Logger.Error("can't return pipeline result ", err)
		sendError(w, err)
		return
	}
}
