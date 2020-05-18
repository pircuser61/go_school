package handlers

import (
	"context"
	"gitlab.services.mts.ru/erius/pipeliner/internal/entity"
	"gitlab.services.mts.ru/erius/pipeliner/internal/script"
	"go.opencensus.io/trace"
	"net/http"
)

func (ae ApiEnv) GetModules(w http.ResponseWriter, req *http.Request){
	ctx, s := trace.StartSpan(context.Background(), "list_modules")
	defer s.End()

	eriusFunctions, err := script.GetReadyFuncs(ctx, ae.ScriptManager)
	if err != nil  {
		ae.Logger.WithError(err).Error("can't get erius functions from script manager")
		return
	}

	eriusShapes, err := script.GetShapes()
	if err != nil  {
		ae.Logger.WithError(err).Error("can't get erius functions from script manager")
		return
	}

	err = sendResponse(w, http.StatusOK, entity.EriusFunctionList{Functions: eriusFunctions, Shapes:eriusShapes})
	if err != nil {
		ae.Logger.Error("can't send response", err)
		return
	}
}