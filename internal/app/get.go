package app

import (
	"context"
	"go.opencensus.io/trace"
	"net/http"
)

func (p Pipeliner) GetPipeline(w http.ResponseWriter, req *http.Request) {
	_, s := trace.StartSpan(context.Background(), "get_pipeline")
	defer s.End()
}
