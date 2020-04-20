package handlers

import (
	"context"
	"go.opencensus.io/trace"
	"net/http"
)

func (ae ApiEnv) GetModules(w http.ResponseWriter, req *http.Request){
	_, s := trace.StartSpan(context.Background(), "list_pipelines")
	defer s.End()
	err := sendResponse(w, http.StatusOK, nil)
	if err != nil {
		ae.Logger.Error("can't send response", err)
		return
	}
}