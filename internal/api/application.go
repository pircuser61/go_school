package api

import (
	"encoding/json"
	"net/http"

	"github.com/iancoleman/orderedmap"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"go.opencensus.io/trace"
)

func (ae *APIEnv) GetApplication(w http.ResponseWriter, req *http.Request, workNumber string) {
	ctx, s := trace.StartSpan(req.Context(), "get_application")
	defer s.End()

	log := logger.GetLogger(ctx)

	data, err := ae.DB.GetApplicationData(workNumber)
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

func (ae *APIEnv) SetApplication(w http.ResponseWriter, req *http.Request, workNumber string) {
	ctx, s := trace.StartSpan(req.Context(), "get_application")
	defer s.End()

	log := logger.GetLogger(ctx)

	var data orderedmap.OrderedMap
	defer req.Body.Close()
	if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
		e := BodyParseError
		log.Error(err)
		_ = e.sendError(w)

		return
	}

	err := ae.DB.SetApplicationData(workNumber, &data)
	if err != nil {
		e := UnknownError
		log.Error(err)
		_ = e.sendError(w)

		return
	}
}
