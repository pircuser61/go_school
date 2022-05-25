package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/google/uuid"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

// GetPipelineTags
// @Summary Get Pipeline Tags
// @Description Список тегов сценария
// @Tags pipeline, tags
// @ID      get-pipeline-tags
// @Produce json
// @Param pipelineID path string true "Pipeline ID"
// @success 200 {object} httpResponse{data=[]entity.EriusTagInfo}
// @Failure 400 {object} httpError
// @Failure 401 {object} httpError
// @Failure 500 {object} httpError
// @Router /pipelines/{pipelineID}/tags/ [get]
func (ae *APIEnv) GetPipelineTag(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "get_pipeline_tag")
	defer s.End()

	log := logger.GetLogger(ctx)

	pipelineID := chi.URLParam(req, "pipelineID")

	pID, err := uuid.Parse(pipelineID)
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

// @Summary Attach Tag
// @Description Прикрепить тег к сценарию
// @Tags pipeline, tags
// @ID      attach-tag
// @Produce json
// @Param pipelineID path string true "Pipeline ID"
// @Param ID path string true "Tag ID"
// @Success 200 {object} httpResponse{data=entity.EriusTagInfo}
// @Failure 400 {object} httpError
// @Failure 401 {object} httpError
// @Failure 500 {object} httpError
// @Router /pipelines/{pipelineID}/tags/{ID} [put]
//nolint:dupl //its different function
func (ae *APIEnv) AttachTag(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "attach_tag")
	defer s.End()

	log := logger.GetLogger(ctx)

	pipelineID := chi.URLParam(req, "pipelineID")

	pID, err := uuid.Parse(pipelineID)
	if err != nil {
		e := UUIDParsingError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	tagID := chi.URLParam(req, "ID")

	tID, err := uuid.Parse(tagID)
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

// @Summary Detach Tag
// @Description Открепить тег от сценария
// @Tags pipeline, tags
// @ID      detach-tag
// @Produce json
// @Param pipelineID path string true "Pipeline ID"
// @Param ID path string true "Tag ID"
// @Success 200 {object} httpResponse
// @Failure 400 {object} httpError
// @Failure 401 {object} httpError
// @Failure 500 {object} httpError
// @Router /pipelines/{pipelineID}/tags/{ID} [delete]
//nolint:dupl //its different function
func (ae *APIEnv) DetachTag(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "remove_pipeline_tag")
	defer s.End()

	log := logger.GetLogger(ctx)

	pipelineID := chi.URLParam(req, "pipelineID")

	pID, err := uuid.Parse(pipelineID)
	if err != nil {
		e := UUIDParsingError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	tagID := chi.URLParam(req, "ID")

	tID, err := uuid.Parse(tagID)
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
