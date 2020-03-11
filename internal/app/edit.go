package app

import (
	"context"
	"fmt"
	"github.com/go-chi/chi"
	"go.opencensus.io/trace"
	"net/http"
)

func (p Pipeliner) EditPipeline(w http.ResponseWriter, req *http.Request) {
	c, s := trace.StartSpan(context.Background(), "edit_pipeline")
	defer s.End()

	id := chi.URLParam(req, "id")
	fmt.Println(c, id)
}
