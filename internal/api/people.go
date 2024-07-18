package api

import (
	"net/http"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"

	"gitlab.services.mts.ru/abp/myosotis/logger"
)

func (ae *Env) FindPerson(w http.ResponseWriter, r *http.Request, params FindPersonParams) {
	ctx, s := trace.StartSpan(r.Context(), "find_person")
	defer s.End()

	log := logger.GetLogger(ctx).
		WithField("mainFuncName", "FindPerson").
		WithField("method", "get").
		WithField("transport", "rest").
		WithField("traceID", s.SpanContext().TraceID.String()).
		WithField("logVersion", "v1")
	errorHandler := newHTTPErrorHandler(log, w)

	search := ""
	if params.Search != nil {
		search = *params.Search
	}

	enabled := true
	if params.Enabled != nil {
		enabled = *params.Enabled
	}

	ctx = logger.WithLogger(ctx, log)

	user, err := ae.People.GetUser(ctx, search, enabled)
	if err != nil {
		e := GetUserError

		log.WithField("search", search).
			Error(e.errorMessage(err))
		errorHandler.sendError(e)

		return
	}

	ui, err := user.ToSSOUserTyped()
	if err != nil {
		e := GetUserError

		log.WithField("search", search).
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

func (ae *Env) SearchPeople(w http.ResponseWriter, r *http.Request, params SearchPeopleParams) {
	ctx, s := trace.StartSpan(r.Context(), "search_people")
	defer s.End()

	log := logger.GetLogger(ctx).WithField("mainFuncName", "SearchPeople").
		WithField("method", "get").
		WithField("transport", "rest").
		WithField("traceID", s.SpanContext().TraceID.String()).
		WithField("logVersion", "v1")
	errorHandler := newHTTPErrorHandler(log, w)

	enabled := true
	if params.Enabled != nil {
		enabled = *params.Enabled
	}

	ctx = logger.WithLogger(ctx, log)

	users, err := ae.People.GetUsers(ctx, params.Search, params.Limit, []string{}, enabled)
	if err != nil {
		e := GetUserError

		log.WithField("search", params.Search).
			Error(e.errorMessage(err))
		errorHandler.sendError(e)

		return
	}

	var ui *people.SSOUserTyped

	pls := make([]UserInfo, 0, len(users))

	for i := range users {
		ui, err = users[i].ToSSOUserTyped()
		if err != nil {
			e := GetUserError

			log.WithField("search", params.Search).
				Error(e.errorMessage(err))
			errorHandler.sendError(e)

			return
		}

		pls = append(pls, UserInfo{
			Email:       ui.Email,
			FullOrgUnit: ui.Attributes.OrgUnit,
			Fullname:    ui.Attributes.FullName,
			Phone:       ui.Attributes.Phone,
			Mobile:      ui.Attributes.Mobile,
			Position:    ui.Attributes.Title,
			Tabnum:      ui.Tabnum,
			Username:    ui.Username,
		})
	}

	res := PeopleResp{
		People: &pls,
	}

	if err = sendResponse(w, http.StatusOK, res); err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}
