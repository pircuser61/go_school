package api

import (
	"encoding/json"
	"github.com/iancoleman/orderedmap"
	"gitlab.services.mts.ru/abp/myosotis/logger"
	"go.opencensus.io/trace"
	"net/http"
)

func (ae *APIEnv) GetApplication(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "get_application")
	defer s.End()

	log := logger.GetLogger(ctx)

	wn := req.URL.Query().Get("workNumber")
	if wn == "" {
		e := BadFiltersError
		log.Error("no work number")
		_ = e.sendError(w)

		return
	}

	data, err := ae.DB.GetApplicationData(wn)
	if err != nil {
		e := UnknownError
		log.Error(err)
		_ = e.sendError(w)

		return
	}

	err = sendResponse(w, http.StatusOK, data)
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

func (ae *APIEnv) SetApplication(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "get_application")
	defer s.End()

	log := logger.GetLogger(ctx)

	wn := req.URL.Query().Get("workNumber")
	if wn == "" {
		e := BadFiltersError
		log.Error("no work number")
		_ = e.sendError(w)

		return
	}

	var data orderedmap.OrderedMap
	defer req.Body.Close()
	if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
		e := BodyParseError
		log.Error(err)
		_ = e.sendError(w)

		return
	}

	err := ae.DB.SetApplicationData(wn, &data)
	if err != nil {
		e := UnknownError
		log.Error(err)
		_ = e.sendError(w)

		return
	}
}
