package handlers

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/google/uuid"
	"gitlab.services.mts.ru/erius/admin/pkg/auth"
	"gitlab.services.mts.ru/erius/pipeliner/internal/entity"

	"gitlab.services.mts.ru/erius/admin/pkg/vars"

	"go.opencensus.io/trace"
)

func (ae *APIEnv) GetTags(w http.ResponseWriter, req *http.Request) {
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

	tags, err := ae.DB.GetAllTags(ctx)
	if err != nil {
		e := GetAllTagsError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
	}

	if err := sendResponse(w, http.StatusOK, tags); err != nil {
		e := UnknownError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

func (ae *APIEnv) CreateTag(w http.ResponseWriter, req *http.Request) {
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

	b, err := ioutil.ReadAll(req.Body)
	defer req.Body.Close()

	if err != nil {
		e := RequestReadError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	etag := entity.EriusTagInfo{}

	err = json.Unmarshal(b, &etag)
	if err != nil {
		e := TagParseError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if etag.Name == "" {
		e := TagHasZeroLengthError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
	}

	etag.ID = uuid.New()

	user, err := auth.UserFromContext(ctx)
	if err != nil {
		ae.Logger.Error("user failed: ", err.Error())
	}

	created, err := ae.DB.CreateTag(ctx, &etag, user.UserName())
	if err != nil {
		e := TagCreateError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = sendResponse(w, http.StatusOK, created)
	if err != nil {
		e := UnknownError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

func (ae *APIEnv) EditTag(w http.ResponseWriter, req *http.Request) {
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

	b, err := ioutil.ReadAll(req.Body)
	defer req.Body.Close()

	if err != nil {
		e := RequestReadError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	etag := entity.EriusTagInfo{}

	err = json.Unmarshal(b, &etag)
	if err != nil {
		e := TagParseError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = ae.DB.EditTag(ctx, &etag)
	if err != nil {
		e := TagEditError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	edited, err := ae.DB.GetTag(ctx, &etag)
	if err != nil {
		e := GetTagError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = sendResponse(w, http.StatusOK, edited)
	if err != nil {
		e := UnknownError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

func (ae *APIEnv) RemoveTag(w http.ResponseWriter, req *http.Request) {
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

	tagID := chi.URLParam(req, "ID")

	tID, err := uuid.Parse(tagID)
	if err != nil {
		e := UUIDParsingError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	etag := entity.EriusTagInfo{}

	etag.ID = tID

	_, err = ae.DB.GetTag(ctx, &etag)
	if err != nil {
		e := GetTagError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = ae.DB.RemoveTag(ctx, &etag)
	if err != nil {
		e := TagDeleteError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = sendResponse(w, http.StatusOK, nil)
	if err != nil {
		e := UnknownError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}
