package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/google/uuid"
	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

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

func (ae *APIEnv) CreateTag(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "create_tag")
	defer s.End()

	log := logger.GetLogger(ctx)

	b, err := io.ReadAll(req.Body)
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

	created, err := ae.DB.CreateTag(ctx, &etag, "")
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

func (ae *APIEnv) EditTag(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "edit_tag")
	defer s.End()

	log := logger.GetLogger(ctx)

	b, err := io.ReadAll(req.Body)
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

func (ae *APIEnv) RemoveTag(w http.ResponseWriter, req *http.Request, tagID string) {
	ctx, s := trace.StartSpan(req.Context(), "remove_tag")
	defer s.End()

	log := logger.GetLogger(ctx)

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
