package api

import (
	"encoding/json"
	"net/http"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/metrics"
)

type httpErrorHandler struct {
	log logger.Logger
	w   http.ResponseWriter

	metricsRequestInfo *metrics.RequestInfo
}

func newHTTPErrorHandler(log logger.Logger, w http.ResponseWriter) httpErrorHandler {
	return httpErrorHandler{log: log, w: w}
}

func (h *httpErrorHandler) setMetricsRequestInfo(req *metrics.RequestInfo) {
	h.metricsRequestInfo = req
}

func (h *httpErrorHandler) handleError(httpErr Err, err error) {
	h.log.Error(httpErr.errorMessage(err))
	h.sendError(httpErr)
}

func (h *httpErrorHandler) sendError(httpErr Err) {
	_ = httpErr.sendError(h.w)
	h.setMetricsRequestInfoStatus(httpErr)
}

func (h *httpErrorHandler) handleErrorStatusCode(httpErr Err, err error, statusCode int) {
	h.log.Error(httpErr.errorMessage(err))
	h.sendErrorStatusCode(httpErr, statusCode)
}

func (h *httpErrorHandler) sendErrorStatusCode(httpErr Err, statusCode int) {
	resp := httpError{
		StatusCode:  statusCode,
		Error:       httpErr.error(),
		Description: httpErr.description(),
	}

	h.w.WriteHeader(statusCode)

	_ = json.NewEncoder(h.w).Encode(resp)

	h.setMetricsRequestInfoStatus(httpErr)
}

func (h *httpErrorHandler) setMetricsRequestInfoStatus(httpErr Err) {
	if h.metricsRequestInfo == nil {
		return
	}

	h.metricsRequestInfo.Status = httpErr.Status()
}
