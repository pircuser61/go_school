package handlers

import (
	"context"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
	"gitlab.services.mts.ru/erius/pipeliner/internal/entity"
	"gitlab.services.mts.ru/erius/pipeliner/internal/script"
	"go.opencensus.io/trace"
	"net/http"
)

func (ae ApiEnv) GetModules(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(context.Background(), "list_modules")
	defer s.End()

	eriusFunctions, err := script.GetReadyFuncs(ctx, ae.ScriptManager)
	if err != nil {
		e := UnknownError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}

	eriusShapes, err := script.GetShapes()
	if err != nil {
		e := UnknownError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}

	err = sendResponse(w, http.StatusOK, entity.EriusFunctionList{Functions: eriusFunctions, Shapes: eriusShapes})
	if err != nil {
		e := UnknownError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}
}

func (ae ApiEnv) ModuleUsage(w http.ResponseWriter, req *http.Request) {
	_, s := trace.StartSpan(context.Background(), "list_modules")
	defer s.End()
	name := chi.URLParam(req, "moduleName")
	usedBy := make([]uuid.UUID, 3)
	for i := 0; i < 3; i++ {
		usedBy[i] = uuid.New()
	}

	err := sendResponse(w, http.StatusOK, entity.UsageResponse{Name: name, UsedBy: usedBy})
	if err != nil {
		e := UnknownError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}
}