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

func (p Pipeliner) AddPipeline(w http.ResponseWriter, req *http.Request) {
	c, s := trace.StartSpan(context.Background(), "add_pipeline")
	defer s.End()
	b, err := ioutil.ReadAll(req.Body)
	defer req.Body.Close()
	if err != nil {
		p.Logger.Error("can't get pipeline from request body", err)
		return
	}
	pipe := model.Pipeline{}
	err = json.Unmarshal(b, &pipe)
	if err != nil {
		p.Logger.Error("can't unmarshal pipeline", err)
		return
	}
	err = db.AddPipeline(c, p.DBConnection,  b,pipe.Name)
	if err != nil {
		p.Logger.Error("can't add pipeline to db", err)
		return
	}
}
