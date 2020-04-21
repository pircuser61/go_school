package handlers

import (
	"context"
	"gitlab.services.mts.ru/erius/pipeliner/internal/script"
	"go.opencensus.io/trace"
	"net/http"
)

func (ae ApiEnv) GetModules(w http.ResponseWriter, req *http.Request){
	_, s := trace.StartSpan(context.Background(), "list_pipelines")
	defer s.End()

	eriusFunctions, err := script.GetReadyFuncs(ae.ScriptManager)
	if err != nil  {
		ae.Logger.WithError(err).Error("can't get erius functions from script manager")
		return
	}

	err = sendResponse(w, http.StatusOK, EriusFunctionList{Functions: eriusFunctions})
	if err != nil {
		ae.Logger.Error("can't send response", err)
		return
	}
}