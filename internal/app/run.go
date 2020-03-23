package app

import (
	"context"
	"go.opencensus.io/trace"
	"net/http"
)

func (p Pipeliner) RunPipeline(w http.ResponseWriter, req *http.Request) {
	_, s := trace.StartSpan(context.Background(), "add_pipeline")
	defer s.End()


}
