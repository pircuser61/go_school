package api

import (
	"net/http"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"
)

func (ae *Env) GetUser(w http.ResponseWriter, r *http.Request, params GetUserParams) {
	ctx, s := trace.StartSpan(r.Context(), "get_user")
	defer s.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	user, err := ae.People.GetUser(ctx, params.Search, false)
	if err != nil {
		e := GetUserError

		log.WithField("username", params.Search).
			Error(e.errorMessage(err))
		errorHandler.sendError(e)

		return
	}

	ui, err := user.ToSSOUserTyped()
	if err != nil {
		e := GetUserError

		log.WithField("username", params.Search).
			Error(e.errorMessage(err))
		errorHandler.sendError(e)

		return
	}

	if err = sendResponse(w, http.StatusOK, UserInfo{
		Email:       ui.Email,
		FullOrgUnit: ui.Attributes.OrgUnit,
		Fullname:    ui.Attributes.FullName,
		Phone:       ui.Attributes.Phone,
		Mobile:      ui.Attributes.Mobile,
		Position:    ui.Attributes.Title,
		Tabnum:      ui.Tabnum,
		Username:    ui.Username,
	}); err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}
