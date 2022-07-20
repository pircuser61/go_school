package api

import (
	"net/http"

	"github.com/google/uuid"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

func (ae *APIEnv) GetPipelineTag(w http.ResponseWriter, req *http.Request, pipelineID PipelineID, tagID TagID) {
	ctx, s := trace.StartSpan(req.Context(), "get_pipeline_tag")
	defer s.End()

	log := logger.GetLogger(ctx)

	pID, err := uuid.Parse(string(pipelineID))
	if err != nil {
		e := UUIDParsingError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	tags, err := ae.DB.GetPipelineTag(ctx, pID)
	if err != nil {
		e := GetPipelineTagsError
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

func (ae *APIEnv) AttachTag(w http.ResponseWriter, req *http.Request, pipelineID PipelineID, tagID TagID) {
	ctx, s := trace.StartSpan(req.Context(), "attach_tag")
	defer s.End()

	log := logger.GetLogger(ctx)

	pID, err := uuid.Parse(string(pipelineID))
	if err != nil {
		e := UUIDParsingError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	tID, err := uuid.Parse(string(tagID))
	if err != nil {
		e := UUIDParsingError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	etag := entity.EriusTagInfo{}

	etag.ID = tID

	attached, err := ae.DB.GetTag(ctx, &etag)
	if err != nil {
		e := GetTagError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = ae.DB.AttachTag(ctx, pID, &etag)
	if err != nil {
		e := TagAttachError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = sendResponse(w, http.StatusOK, attached)
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

func (ae *APIEnv) DetachTag(w http.ResponseWriter, req *http.Request, pipelineID PipelineID, tagID TagID) {
	ctx, s := trace.StartSpan(req.Context(), "remove_pipeline_tag")
	defer s.End()

	log := logger.GetLogger(ctx)

	pID, err := uuid.Parse(string(pipelineID))
	if err != nil {
		e := UUIDParsingError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	tID, err := uuid.Parse(string(tagID))
	if err != nil {
		e := UUIDParsingError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	etag := entity.EriusTagInfo{}

	etag.ID = tID

	_, err = ae.DB.GetTag(ctx, &etag)
	if err != nil {
		e := GetTagError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = ae.DB.DetachTag(ctx, pID, &etag)
	if err != nil {
		e := TagDetachError
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
