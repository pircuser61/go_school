package api

import (
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

func (h *httpErrorHandler) setMetricsRequestInfoStatus(httpErr Err) {
	if h.metricsRequestInfo == nil {
		return
	}

	h.metricsRequestInfo.Status = httpErr.Status()
}
