package api

import (
	"net/http"

	"gitlab.services.mts.ru/abp/myosotis/logger"
)

type httpErrorHandler struct {
	log logger.Logger
	w   http.ResponseWriter
}

func newHttpErrorHandler(log logger.Logger, w http.ResponseWriter) httpErrorHandler {
	return httpErrorHandler{log: log, w: w}
}

func (h *httpErrorHandler) handleError(httpErr Err, err error) {
	h.log.Error(httpErr.errorMessage(err))
	h.sendError(httpErr)
}

func (h *httpErrorHandler) sendError(httpErr Err) {
	_ = httpErr.sendError(h.w)
}
