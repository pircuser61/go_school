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

// GetTags
// @Summary Get Tags
// @Description Cписок тегов
// @Tags tags
// @ID      get-tags
// @Produce json
// @success 200 {object} httpResponse{data=[]entity.EriusTagInfo}
// @Failure 400 {object} httpError
// @Failure 401 {object} httpError
// @Failure 500 {object} httpError
// @Router /tags/ [get]
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

// @Summary Create Tag
// @Description Создать новый тег
// @Tags tags
// @ID      create-tag
// @Accept json
// @Produce json
// @Param tag body entity.EriusTagInfo true "New tag"
// @Success 200 {object} httpResponse{data=entity.EriusTagInfo}
// @Failure 400 {object} httpError
// @Failure 401 {object} httpError
// @Failure 500 {object} httpError
// @Router /tags/ [post]
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

// @Summary Edit Tag
// @Description Изменить тег
// @Tags tags
// @ID      edit-tag
// @Accept json
// @Produce json
// @Param tag body entity.EriusTagInfo true "Modified tag"
// @Success 200 {object} httpResponse{data=entity.EriusTagInfo}
// @Failure 400 {object} httpError
// @Failure 401 {object} httpError
// @Failure 500 {object} httpError
// @Router /tags/ [put]
func (ae *APIEnv) EditTag(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "edit_tag")
	defer s.End()

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

	id := etag.ID.String()

	if !(grants.Allow && grants.Contains(id)) {
		e := UnauthError
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

// @Summary Remove Tag
// @Description Удалить тег
// @Tags tags
// @ID      remove-tag
// @Produce json
// @Param ID path string true "Tag ID"
// @Success 200 {object} httpResponse
// @Failure 400 {object} httpError
// @Failure 401 {object} httpError
// @Failure 500 {object} httpError
// @Router /tags/{ID} [delete]
func (ae *APIEnv) RemoveTag(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "remove_tag")
	defer s.End()

	tagID := chi.URLParam(req, "ID")

	tID, err := uuid.Parse(tagID)
	if err != nil {
		e := UUIDParsingError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

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

	id := tID.String()

	if !(grants.Allow && grants.Contains(id)) {
		e := UnauthError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = ae.DB.RemoveTag(ctx, tID)
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
