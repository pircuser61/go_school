package handlers

import (
	"context"
	"github.com/go-chi/chi"
	"gitlab.services.mts.ru/erius/pipeliner/internal/db"
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
	c, s := trace.StartSpan(context.Background(), "list_modules")
	defer s.End()
	name := chi.URLParam(req, "moduleName")

	allWorked, err := db.GetWorkedVersions(c, ae.DBConnection)
	if err != nil {
		e := ModuleUsageError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}

	usedBy := make([]entity.UsedBy, 0, 0)
	used := false
	for _, pipe := range allWorked {
		for _, f := range pipe.Pipeline.Blocks {
			if f.Title == name {
				usedBy = append(usedBy, entity.UsedBy{Name:pipe.Name, ID: pipe.ID})
				used = true
				break
			}
		}
	}

	err = sendResponse(w, http.StatusOK, entity.UsageResponse{Name: name, Pipelines: usedBy, Used:used})
	if err != nil {
		e := UnknownError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}
}
