package handlers

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/erius/admin/pkg/auth"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
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

	log := logger.GetLogger(ctx)

	tags, err := ae.DB.GetAllTags(ctx)
	if err != nil {
		e := GetAllTagsError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
	}

	if err := sendResponse(w, http.StatusOK, tags); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
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

	log := logger.GetLogger(ctx)

	b, err := ioutil.ReadAll(req.Body)
	defer req.Body.Close()

	if err != nil {
		e := RequestReadError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	etag := entity.EriusTagInfo{}

	err = json.Unmarshal(b, &etag)
	if err != nil {
		e := TagParseError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	etag.ID = uuid.New()

	user, err := auth.UserFromContext(ctx)
	if err != nil {
		log.WithError(err).Error("user failed")
	}

	created, err := ae.DB.CreateTag(ctx, &etag, user.UserName())
	if err != nil {
		e := TagCreateError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = sendResponse(w, http.StatusOK, created)
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
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

	log := logger.GetLogger(ctx)

	b, err := ioutil.ReadAll(req.Body)
	defer req.Body.Close()

	if err != nil {
		e := RequestReadError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	etag := entity.EriusTagInfo{}

	err = json.Unmarshal(b, &etag)
	if err != nil {
		e := TagParseError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = ae.DB.EditTag(ctx, &etag)
	if err != nil {
		e := TagEditError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	edited, err := ae.DB.GetTag(ctx, &etag)
	if err != nil {
		e := GetTagError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = sendResponse(w, http.StatusOK, edited)
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
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

	log := logger.GetLogger(ctx)

	tagID := chi.URLParam(req, "ID")

	tID, err := uuid.Parse(tagID)
	if err != nil {
		e := UUIDParsingError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = ae.DB.RemoveTag(ctx, tID)
	if err != nil {
		e := TagDeleteError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = sendResponse(w, http.StatusOK, nil)
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}
