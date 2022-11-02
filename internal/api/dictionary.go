package api

import (
	"net/http"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"go.opencensus.io/trace"
)

type GetApproveActionNamesResponse struct {
	Id    string `json:"id"`
	Title string `json:"title"`
}

//nolint:dupl //its not duplicate
func (ae *APIEnv) GetApproveActionNames(w http.ResponseWriter, r *http.Request) {
	ctx, s := trace.StartSpan(r.Context(), "get_approve_action_names")
	defer s.End()

	log := logger.GetLogger(ctx)

	data, err := ae.DB.GetApproveActionNames(ctx)
	if err != nil {
		e := UnknownError
		log.Error(err)
		_ = e.sendError(w)

		return
	}

	res := make([]GetApproveActionNamesResponse, 0, len(data))
	for i := range data {
		res = append(res, GetApproveActionNamesResponse{
			Id:    data[i].Id,
			Title: data[i].Title,
		})
	}

	if err = sendResponse(w, http.StatusOK, res); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

type GetApproveStatusesResponse struct {
	Id    string `json:"id"`
	Title string `json:"title"`
}

//nolint:dupl //its not duplicate
func (ae *APIEnv) GetApproveStatuses(w http.ResponseWriter, r *http.Request) {
	ctx, s := trace.StartSpan(r.Context(), "get_approve_statuses")
	defer s.End()

	log := logger.GetLogger(ctx)

	data, err := ae.DB.GetApproveStatuses(ctx)
	if err != nil {
		e := UnknownError
		log.Error(err)
		_ = e.sendError(w)

		return
	}

	res := make([]GetApproveStatusesResponse, 0, len(data))
	for i := range data {
		res = append(res, GetApproveStatusesResponse{
			Id:    data[i].Id,
			Title: data[i].Title,
		})
	}

	if err = sendResponse(w, http.StatusOK, res); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}
