package handlers

import (
	"net/http"

	"gitlab.services.mts.ru/erius/admin/pkg/vars"

	"go.opencensus.io/trace"
)

// nolint:dupl // mock method
func (ae APIEnv) GetTags(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "get_tags")
	defer s.End()

	grants, err := ae.AuthClient.CheckGrants(ctx, vars.PipelineTag, vars.Read)
	if err != nil {
		e := AuthServiceError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if !grants.Allow {
		e := UnauthError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if err = Teapot.sendError(w); err != nil {
		ae.Logger.Error("can't send response", err)

		return
	}
}

// nolint:dupl // mock method
func (ae APIEnv) CreateTag(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "create_tag")
	defer s.End()

	grants, err := ae.AuthClient.CheckGrants(ctx, vars.PipelineTag, vars.Create)
	if err != nil {
		e := AuthServiceError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if !grants.Allow {
		e := UnauthError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if err := Teapot.sendError(w); err != nil {
		ae.Logger.Error("can't send response", err)

		return
	}
}

// nolint:dupl // mock method
func (ae APIEnv) EditTag(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "edit_tag")
	defer s.End()

	grants, err := ae.AuthClient.CheckGrants(ctx, vars.PipelineTag, vars.Update)
	if err != nil {
		e := AuthServiceError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if !grants.Allow {
		e := UnauthError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if err := Teapot.sendError(w); err != nil {
		ae.Logger.Error("can't send response", err)

		return
	}
}

// nolint:dupl // mock method
func (ae APIEnv) RemoveTag(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "remove_tag")
	defer s.End()

	grants, err := ae.AuthClient.CheckGrants(ctx, vars.PipelineTag, vars.Delete)
	if err != nil {
		e := AuthServiceError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if !grants.Allow {
		e := UnauthError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if err := Teapot.sendError(w); err != nil {
		ae.Logger.Error("can't send response", err)

		return
	}
}
