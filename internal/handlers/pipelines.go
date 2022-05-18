package handlers

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/pkg/errors"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/erius/admin/pkg/auth"
	"gitlab.services.mts.ru/erius/admin/pkg/vars"
	"gitlab.services.mts.ru/erius/monitoring/pkg/monitor"
	"gitlab.services.mts.ru/erius/monitoring/pkg/pipeliner/monitoring"
	"gitlab.services.mts.ru/erius/pipeliner/internal/script"

	"gitlab.services.mts.ru/erius/pipeliner/internal/db"
	"gitlab.services.mts.ru/erius/pipeliner/internal/entity"
	"gitlab.services.mts.ru/erius/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/erius/pipeliner/internal/store"
)

const (
	statusRunned   = "runned"
	statusFinished = "finished"
	statusError    = "error"
)

type RunContext struct {
	ID         string            `json:"id"`
	Parameters map[string]string `json:"parameters"`
}

// ListPipelines godoc
// @Summary Get list of pipelines
// @Description Список сценариев
// @Tags pipeline
// @ID      list-pipelines
// @Produce json
// @success 200 {object} httpResponse{data=entity.EriusScenarioList}
// @success 401 {object} httpError
// @Failure 500 {object} httpError
// @Router /pipelines/ [get]
func (ae *APIEnv) ListPipelines(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "list_pipelines")
	defer s.End()

	log := logger.GetLogger(ctx)

	drafts, err := ae.draftVersions(ctx)
	if err != nil {
		_ = err.sendError(w)

		return
	}

	approved, err := ae.approvedVersions(ctx)
	if err != nil {
		_ = err.sendError(w)

		return
	}

	onApprove, perr := ae.onApprovedVersions(ctx)
	if perr != nil {
		_ = perr.sendError(w)

		return
	}

	tags, err := ae.tags(ctx)
	if err != nil {
		_ = err.sendError(w)

		return
	}

	resp := entity.EriusScenarioList{
		Pipelines: approved,
		OnApprove: onApprove,
		Drafts:    drafts,
		Tags:      tags,
	}

	if err := sendResponse(w, http.StatusOK, resp); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

// draftVersions выбирает версии сценария с признаком Draft,
// разрешенные для данного пользователя
//nolint:dupl //diff logic
func (ae *APIEnv) draftVersions(ctx context.Context) ([]entity.EriusScenarioInfo, *PipelinerError) {
	ctx, s := trace.StartSpan(ctx, "list_drafts")
	defer s.End()

	log := logger.GetLogger(ctx)

	grants, err := ae.AuthClient.CheckGrants(ctx, vars.PipelineVersion, vars.Own)
	if err != nil {
		log.Error(AuthServiceError.errorMessage(err))

		return []entity.EriusScenarioInfo{}, &PipelinerError{AuthServiceError}
	}

	if !grants.Allow {
		log.Error(UnauthError.errorMessage(err))

		return []entity.EriusScenarioInfo{}, nil
	}

	drafts, err := ae.DB.GetDraftVersions(ctx)
	if err != nil {
		return []entity.EriusScenarioInfo{}, &PipelinerError{GetAllDraftsError}
	}

	onapprove, err := ae.DB.GetOnApproveVersions(ctx)
	if err != nil {
		return []entity.EriusScenarioInfo{}, &PipelinerError{GetAllOnApproveError}
	}

	rejected, err := ae.DB.GetRejectedVersions(ctx)
	if err != nil {
		return []entity.EriusScenarioInfo{}, &PipelinerError{GetAllRejectedError}
	}

	drafts = append(drafts, onapprove...)
	drafts = append(drafts, rejected...)

	return filterVersionsByID(drafts, grants.All, grants.Items), nil
}

// onApprovedVersions выбирает версии сценариев с признаком OnApprove,
// разрешенные для данного пользователя
//nolint:dupl //different logic
func (ae *APIEnv) onApprovedVersions(ctx context.Context) ([]entity.EriusScenarioInfo, *PipelinerError) {
	ctx, s := trace.StartSpan(ctx, "list_on_approve_versions")
	defer s.End()

	log := logger.GetLogger(ctx)

	grants, err := ae.AuthClient.CheckGrants(ctx, vars.Pipeline, vars.Approve)
	if err != nil {
		log.Error(AuthServiceError.errorMessage(err))

		return []entity.EriusScenarioInfo{}, &PipelinerError{AuthServiceError}
	}

	if !grants.Allow {
		log.Error(UnauthError.errorMessage(err))

		return []entity.EriusScenarioInfo{}, nil
	}

	onApprove, err := ae.DB.GetOnApproveVersions(ctx)
	if err != nil {
		log.Error(GetAllOnApproveError.errorMessage(err))

		return []entity.EriusScenarioInfo{}, &PipelinerError{GetAllOnApproveError}
	}

	return onApprove, nil
}

// approvedVersions выбирает последние рабочие версии сценариев,
// разрешенные для данного пользователя
//nolint:dupl //different logic
func (ae *APIEnv) approvedVersions(ctx context.Context) ([]entity.EriusScenarioInfo, *PipelinerError) {
	ctx, s := trace.StartSpan(ctx, "list_approved_versions")
	defer s.End()

	log := logger.GetLogger(ctx)

	grants, err := ae.AuthClient.CheckGrants(ctx, vars.Pipeline, vars.Read)
	if err != nil {
		log.Error(AuthServiceError.errorMessage(err))

		return []entity.EriusScenarioInfo{}, &PipelinerError{AuthServiceError}
	}

	if !grants.Allow {
		log.Error(UnauthError.errorMessage(err))

		return []entity.EriusScenarioInfo{}, nil
	}

	approved, err := ae.DB.GetApprovedVersions(ctx)
	if err != nil {
		log.Error(GetAllApprovedError.errorMessage(err))

		return []entity.EriusScenarioInfo{}, &PipelinerError{GetAllApprovedError}
	}

	return filterPipelinesByID(approved, grants.All, grants.Items), nil
}

// nolint:dupl // original code
func (ae *APIEnv) tags(ctx context.Context) ([]entity.EriusTagInfo, *PipelinerError) {
	ctx, s := trace.StartSpan(ctx, "list_tags")
	defer s.End()

	log := logger.GetLogger(ctx)

	grants, err := ae.AuthClient.CheckGrants(ctx, vars.PipelineTag, vars.Read)
	if err != nil {
		log.Error(AuthServiceError.errorMessage(err))

		return []entity.EriusTagInfo{}, &PipelinerError{AuthServiceError}
	}

	if !grants.Allow {
		log.Error(UnauthError.errorMessage(err))

		return []entity.EriusTagInfo{}, nil
	}

	tags, err := ae.DB.GetAllTags(ctx)
	if err != nil {
		log.Error(GetAllTagsError.errorMessage(err))

		return []entity.EriusTagInfo{}, &PipelinerError{GetAllTagsError}
	}

	return tags, nil
}

// GetPipelineVersion
// @Summary Get pipeline version
// @Description Получить версию сценария по ID
// @Tags version
// @ID      get-version
// @Produce json
// @Param versionID path string true "Version ID"
// @success 200 {object} httpResponse{data=entity.EriusScenario}
// @Failure 400 {object} httpError
// @Failure 401 {object} httpError
// @Failure 500 {object} httpError
// @Router /pipelines/version/{versionID} [get]
func (ae *APIEnv) GetPipelineVersion(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "get_version")
	defer s.End()

	log := logger.GetLogger(ctx)

	versionID := chi.URLParam(req, "versionID")

	versionUUID, err := uuid.Parse(versionID)
	if err != nil {
		e := UUIDParsingError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	p, err := ae.DB.GetPipelineVersion(ctx, versionUUID)
	if err != nil {
		e := GetVersionError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	tags, err := ae.DB.GetPipelineTag(ctx, p.ID)
	if err != nil {
		e := GetPipelineTagsError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
	}

	p.Tags = tags

	grants, err := ae.AuthClient.CheckGrants(ctx, vars.Pipeline, vars.Read)
	if err != nil {
		e := AuthServiceError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	// проверяем доступ к сценарию запрошенной версии
	if !(grants.Allow && grants.Contains(p.ID.String())) {
		e := UnauthError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = sendResponse(w, http.StatusOK, p)
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

// GetPipeline
// @Summary Get pipeline
// @Description Получить сценарий по ID
// @Tags pipeline
// @ID      get-pipeline
// @Produce json
// @Param pipelineID path string true "Pipeline ID"
// @success 200 {object} httpResponse{data=entity.EriusScenario}
// @Failure 400 {object} httpError
// @Failure 401 {object} httpError
// @Failure 500 {object} httpError
// @Router /pipelines/pipeline/{pipelineID} [get]
func (ae *APIEnv) GetPipeline(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "get_pipeline")
	defer s.End()

	log := logger.GetLogger(ctx)

	idParam := chi.URLParam(req, "pipelineID")

	id, err := uuid.Parse(idParam)
	if err != nil {
		e := UUIDParsingError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	grants, err := ae.AuthClient.CheckGrants(ctx, vars.Pipeline, vars.Read)
	if err != nil {
		e := AuthServiceError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	// проверяем доступ на чтение запрошенного сценария
	if !(grants.Allow && grants.Contains(idParam)) {
		e := UnauthError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	p, err := ae.DB.GetPipeline(ctx, id)
	if err != nil {
		e := GetPipelineError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	tags, err := ae.DB.GetPipelineTag(ctx, p.ID)
	if err != nil {
		e := GetPipelineTagsError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
	}

	p.Tags = tags

	err = sendResponse(w, http.StatusOK, p)
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

// @Summary Create pipeline version
// @Description Создать новую версию сценария
// @Tags version
// @ID      create-version
// @Accept json
// @Produce json
// @Param pipeline   body entity.EriusScenario  true "New version"
// @Param pipelineID path string 				true "Pipeline ID"
// @success 200 {object} httpResponse{data=entity.EriusScenario}
// @Failure 400 {object} httpError
// @Failure 401 {object} httpError
// @Failure 500 {object} httpError
// @Router /pipelines/version/{pipelineID} [post]
func (ae *APIEnv) CreatePipelineVersion(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "create_draft")
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

	p := entity.EriusScenario{}

	err = json.Unmarshal(b, &p)
	if err != nil {
		e := PipelineParseError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	pipelineID := chi.URLParam(req, "pipelineID")

	p.ID, err = uuid.Parse(pipelineID)
	if err != nil {
		e := VersionCreateError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	p.VersionID = uuid.New()

	grants, err := ae.AuthClient.CheckGrants(ctx, vars.Pipeline, vars.Create)
	if err != nil {
		e := AuthServiceError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if !grants.Allow {
		e := UnauthError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	user, err := auth.UserFromContext(ctx)
	if err != nil {
		log.WithError(err).Error("user failed")
	}
	//nolint:govet //it doesn't shadow
	canCreate, err := ae.DB.DraftPipelineCreatable(ctx, p.ID, user.UserName())
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if !canCreate {
		e := PipelineHasDraft
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = ae.DB.CreateVersion(ctx, &p, user.UserName(), b)
	if err != nil {
		e := PipelineWriteError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	created, err := ae.DB.GetPipelineVersion(ctx, p.VersionID)
	if err != nil {
		e := PipelineReadError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if err = ae.AuthClient.Notice(ctx, &auth.Notice{
		NoticeType:   vars.CreateNotice,
		ResourceType: vars.PipelineVersion,
		ResourceID:   created.VersionID.String(),
	}); err != nil {
		e := AuthServiceError
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

// @Summary Create pipeline
// @Description Создать новый сценарий
// @Tags pipeline
// @ID      create-pipeline
// @Accept json
// @Produce json
// @Param pipeline body entity.EriusScenario true "New scenario"
// @Success 200 {object} httpResponse{data=entity.EriusScenario}
// @Failure 400 {object} httpError
// @Failure 401 {object} httpError
// @Failure 500 {object} httpError
// @Router /pipelines/ [post]
//nolint:dupl //diff logic
func (ae *APIEnv) CreatePipeline(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "create_pipeline")
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

	p := entity.EriusScenario{}

	err = json.Unmarshal(b, &p)
	if err != nil {
		e := PipelineParseError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	grants, err := ae.AuthClient.CheckGrants(ctx, vars.Pipeline, vars.Create)
	if err != nil {
		e := AuthServiceError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if !grants.Allow {
		e := UnauthError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	user, err := auth.UserFromContext(ctx)
	if err != nil {
		log.Error("user failed: ", err.Error())
	}

	p.ID = uuid.New()
	p.VersionID = uuid.New()

	canCreate, err := ae.DB.PipelineNameCreatable(ctx, p.Name)
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if !canCreate {
		e := PipelineNameUsed
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = ae.DB.CreatePipeline(ctx, &p, user.UserName(), b)
	if err != nil {
		e := PipelineCreateError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	created, err := ae.DB.GetPipelineVersion(ctx, p.VersionID)
	if err != nil {
		e := PipelineReadError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if err = ae.AuthClient.Notice(ctx, &auth.Notice{
		NoticeType:   vars.CreateNotice,
		ResourceType: vars.PipelineVersion,
		ResourceID:   created.VersionID.String(),
	}); err != nil {
		e := AuthServiceError
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

// @Summary Edit Draft
// @Description Изменить черновик
// @Tags pipeline
// @ID      edit-draft
// @Accept json
// @Produce json
// @Param draft body entity.EriusScenario true "New draft"
// @Success 200 {object} httpResponse{data=entity.EriusScenario}
// @Failure 400 {object} httpError
// @Failure 401 {object} httpError
// @Failure 500 {object} httpError
// @Router /pipelines/version [put]
//nolint:gocyclo //its  necessary
func (ae *APIEnv) EditVersion(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "edit_draft")
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

	p := entity.EriusScenario{}

	err = json.Unmarshal(b, &p)
	if err != nil {
		e := PipelineParseError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	resource, action, id := authUpdateParametersByPipelineStatus(&p)

	grants, err := ae.AuthClient.CheckGrants(ctx, resource, action)
	if err != nil {
		e := AuthServiceError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if !(grants.Allow && grants.Contains(id)) {
		e := UnauthError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	canEdit, err := ae.DB.VersionEditable(ctx, p.VersionID)
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if !canEdit {
		err = ae.DB.RollbackVersion(ctx, p.ID, p.VersionID)
		if err != nil {
			e := ApproveError
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

		return
	}

	err = ae.DB.UpdateDraft(ctx, &p, b)
	if err != nil {
		e := PipelineWriteError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	user, err := auth.UserFromContext(ctx)
	if err != nil {
		log.Error(err.Error())
	}

	if p.Status == db.StatusApproved {
		err = ae.DB.SwitchApproved(ctx, p.ID, p.VersionID, user.UserName())
		if err != nil {
			e := ApproveError
			log.Error(e.errorMessage(err))
			_ = e.sendError(w)

			return
		}
	}

	if p.Status == db.StatusRejected {
		err = ae.DB.SwitchRejected(ctx, p.VersionID, p.CommentRejected, user.UserName())
		if err != nil {
			e := ApproveError
			log.Error(e.errorMessage(err))
			_ = e.sendError(w)

			return
		}
	}

	edited, err := ae.DB.GetPipelineVersion(ctx, p.VersionID)
	if err != nil {
		e := PipelineReadError
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

func authUpdateParametersByPipelineStatus(p *entity.EriusScenario) (resource vars.ResourceType, action vars.ActionType, id string) {
	switch p.Status {
	case db.StatusDraft, db.StatusOnApprove:
		resource = vars.PipelineVersion
		action = vars.Own
		id = p.VersionID.String()
	case db.StatusApproved, db.StatusRejected:
		resource = vars.Pipeline
		action = vars.Approve
		id = p.ID.String() // pipeline id
	default:
		resource = vars.Pipeline
		action = vars.Update
		id = p.ID.String() // pipeline id
	}

	return resource, action, id
}

func authDeleteParametersByPipelineStatus(p *entity.EriusScenario) (resource vars.ResourceType, action vars.ActionType, id string) {
	switch p.Status {
	case db.StatusDraft:
		resource = vars.PipelineVersion
		action = vars.Own
		id = p.VersionID.String()
	default:
		resource = vars.Pipeline
		action = vars.Delete
		id = p.ID.String() // pipeline id
	}

	return resource, action, id
}

// @Summary Delete Version
// @Description Удалить версию
// @Tags version
// @ID      delete-version
// @Produce json
// @Param versionID path string true "Version ID"
// @Success 200 {object} httpResponse
// @Failure 400 {object} httpError
// @Failure 401 {object} httpError
// @Failure 500 {object} httpError
// @Router /pipelines/version/{versionID} [delete]
func (ae *APIEnv) DeleteVersion(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "delete_version")
	defer s.End()

	log := logger.GetLogger(ctx)

	idParam := chi.URLParam(req, "versionID")

	versionID, err := uuid.Parse(idParam)
	if err != nil {
		e := UUIDParsingError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	p, err := ae.DB.GetPipelineVersion(ctx, versionID)
	if err != nil {
		e := PipelineDeleteError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	resource, action, id := authDeleteParametersByPipelineStatus(p)

	grants, err := ae.AuthClient.CheckGrants(ctx, resource, action)
	if err != nil {
		e := AuthServiceError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if !(grants.Allow && grants.Contains(id)) {
		e := UnauthError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if p.Status == db.StatusDraft {
		err = ae.DeleteDraftPipeline(ctx, w, p)
		if err != nil {
			e := PipelineDeleteError
			log.Error(e.errorMessage(err))
			_ = e.sendError(w)

			return
		}
	}

	err = ae.DB.DeleteVersion(ctx, versionID)
	if err != nil {
		e := PipelineDeleteError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if err = ae.AuthClient.Notice(ctx, &auth.Notice{
		NoticeType:   vars.DeleteNotice,
		ResourceType: vars.PipelineVersion,
		ResourceID:   versionID.String(),
	}); err != nil {
		e := AuthServiceError
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

func (ae *APIEnv) DeleteDraftPipeline(ctx context.Context, w http.ResponseWriter, p *entity.EriusScenario) error {
	ctx, s := trace.StartSpan(ctx, "delete_draft_pipeline")
	defer s.End()

	log := logger.GetLogger(ctx)

	canDelete, err := ae.DB.PipelineRemovable(ctx, p.ID)
	if err != nil {
		e := PipelineIsNotDraft
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return err
	}

	if canDelete {
		err = ae.DB.RemovePipelineTags(ctx, p.ID)
		if err != nil {
			e := TagDetachError
			log.Error(e.errorMessage(err))
			_ = e.sendError(w)

			return err
		}

		err = ae.DB.DeletePipeline(ctx, p.ID)
		if err != nil {
			e := PipelineDeleteError
			log.Error(e.errorMessage(err))
			_ = e.sendError(w)

			return err
		}
	}

	return nil
}

// @Summary Delete Pipeline
// @Description Удалить сценарий
// @Tags pipeline
// @ID      delete-pipeline
// @Produce json
// @Param pipelineID path string true "Pipeline ID"
// @Success 200 {object} httpResponse
// @Failure 400 {object} httpError
// @Failure 401 {object} httpError
// @Failure 500 {object} httpError
// @Router /pipelines/{pipelineID} [delete]
func (ae *APIEnv) DeletePipeline(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "delete_pipeline")
	defer s.End()

	log := logger.GetLogger(ctx)

	idParam := chi.URLParam(req, "pipelineID")

	id, err := uuid.Parse(idParam)
	if err != nil {
		e := UUIDParsingError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	grants, err := ae.AuthClient.CheckGrants(ctx, vars.Pipeline, vars.Delete)
	if err != nil {
		e := AuthServiceError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if !(grants.Allow && grants.Contains(id.String())) {
		e := UnauthError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	childPipelines, err := scenarioUsage(ctx, ae.DB, id)
	if len(childPipelines) > 0 {
		e := ScenarioIsUsedInOtherError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = ae.SchedulerClient.DeleteTasksByPipelineID(ctx, id)
	if err != nil {
		e := SchedulerClientFailed
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = ae.DB.RemovePipelineTags(ctx, id)
	if err != nil {
		e := TagDetachError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = ae.NetworkMonitorClient.UnlinkPipelineByID(ctx, id)
	if err != nil {
		e := NetworkMonitorClientFailed
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = ae.DB.DeletePipeline(ctx, id)
	if err != nil {
		e := PipelineDeleteError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if err = ae.AuthClient.Notice(ctx, &auth.Notice{
		NoticeType:   vars.DeleteNotice,
		ResourceType: vars.Pipeline,
		ResourceID:   id.String(),
	}); err != nil {
		e := AuthServiceError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = sendResponse(w, http.StatusOK, id)
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

// @Summary Active scheduler tasks
// @Description Наличие у сценария активных заданий в шедулере
// @Tags pipeline
// @ID pipeline-scheduler-tasks
// @Accept json
// @Produce json
// @Param pipelineID path string true "Pipeline ID"
// @Success 200 {object} httpResponse{data=entity.SchedulerTasksResponse}
// @Failure 400 {object} httpError
// @Failure 500 {object} httpError
// @Router /pipelines/{pipelineID}/scheduler-tasks [post]
func (ae *APIEnv) ListSchedulerTasks(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "scheduler tasks list")
	defer s.End()

	log := logger.GetLogger(ctx)

	idParam := chi.URLParam(req, "pipelineID")

	id, err := uuid.Parse(idParam)
	if err != nil {
		e := UUIDParsingError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	tasks, err := ae.SchedulerClient.GetTasksByPipelineID(ctx, id)
	if err != nil {
		e := SchedulerClientFailed
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	// в текущей реализации возращаем только факт наличия заданий
	result := &entity.SchedulerTasksResponse{
		Result: len(tasks) > 0,
	}

	err = sendResponse(w, http.StatusOK, result)
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

// @Summary Run Pipeline
// @Description Запустить сценарий
// @Tags pipeline, run
// @ID run-pipeline
// @Accept json
// @Produce json
// @Param variables body object false "pipeline input"
// @Param pipelineID path string true "Pipeline ID"
// @Success 200 {object} httpResponse{data=entity.RunResponse}
// @Failure 400 {object} httpError
// @Failure 401 {object} httpError
// @Failure 500 {object} httpError
// @Router /run/{pipelineID} [post]
func (ae *APIEnv) RunPipeline(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "run_pipeline")
	defer s.End()

	log := logger.GetLogger(ctx)

	withStop := false

	if withStopCtx := req.Context().Value("with_stop"); withStopCtx != nil {
		withStop = true
	}

	keys := req.URL.Query()
	if ws, ok := keys["with_stop"]; ok && !withStop {
		if stop, err := strconv.ParseBool(ws[0]); err == nil {
			withStop = stop
		}
	}

	idParam := chi.URLParam(req, "pipelineID")

	id, err := uuid.Parse(idParam)
	if err != nil {
		e := UUIDParsingError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	p, err := ae.DB.GetPipeline(ctx, id)
	if err != nil {
		e := GetPipelineError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	grants, err := ae.AuthClient.CheckGrants(ctx, vars.Pipeline, vars.Run)
	if err != nil {
		e := AuthServiceError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	// проверяем права на запуск пайплайна
	if !(grants.Allow && grants.Contains(p.ID.String())) {
		e := UnauthError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	ae.execVersion(ctx, w, req, p, withStop)
}

// @Summary Run Version
// @Description Запустить версию
// @Tags version, run
// @ID run-version
// @Accept json
// @Produce json
// @Param variables body object false "pipeline input"
// @Param versionID path string true "Version ID"
// @Success 200 {object} httpResponse{data=entity.RunResponse}
// @Failure 400 {object} httpError
// @Failure 401 {object} httpError
// @Failure 500 {object} httpError
// @Router /run/version/{versionID} [post]
func (ae *APIEnv) RunVersion(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "run_pipeline")
	defer s.End()

	log := logger.GetLogger(ctx)

	idParam := chi.URLParam(req, "versionID")

	id, err := uuid.Parse(idParam)
	if err != nil {
		e := UUIDParsingError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	p, err := ae.DB.GetPipelineVersion(ctx, id)
	if err != nil {
		e := GetPipelineError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	grants, err := ae.AuthClient.CheckGrants(ctx, vars.Pipeline, vars.Run)
	if err != nil {
		e := AuthServiceError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	// проверяем права на запуск пайплайна
	if !(grants.Allow && grants.Contains(p.ID.String())) {
		e := UnauthError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	ae.execVersion(ctx, w, req, p, false)
}

//nolint //need big cyclo,need equal string for all usages
func (ae *APIEnv) execVersion(ctx context.Context, w http.ResponseWriter, req *http.Request, p *entity.EriusScenario, withStop bool) {
	ctx, s := trace.StartSpan(ctx, "exec_version")
	defer s.End()

	log := logger.GetLogger(ctx)

	reqID := req.Header.Get(XRequestIDHeader)

	b, err := ioutil.ReadAll(req.Body)
	defer req.Body.Close()

	mon := monitoring.New()
	mon.Set(reqID, monitor.PipelinerData{
		PipelineUUID: p.ID.String(),
		VersionUUID:  p.VersionID.String(),
		Name:         p.Name,
	})

	var pipelineVars map[string]interface{}
	if len(b) != 0 {
		err = json.Unmarshal(b, &pipelineVars)
		if err != nil {
			e := PipelineRunError
			if monErr := mon.RunError(ctx); monErr != nil {
				log.WithError(monErr).Error("can't send data to monitoring")
			}
			log.Error(e.errorMessage(err))
			_ = e.sendError(w)
		}
	}

	log.Info("--- running pipeline:", p.Name)

	ep, e, err := ae.execVersionInternal(ctx, reqID, p, pipelineVars, withStop)
	if err != nil {
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}

	_ = sendResponse(w, http.StatusOK, entity.RunResponse{
		PipelineID: ep.PipelineID, TaskID: ep.TaskID,
		Status: statusRunned,
	})
}

func (ae *APIEnv) execVersionInternal(ctx context.Context, reqID string, p *entity.EriusScenario, vars map[string]interface{}, withStop bool) (*pipeline.ExecutablePipeline, Err, error) {

	log := logger.GetLogger(ctx)

	ctx = context.WithValue(ctx, XRequestIDHeader, reqID)

	ep := pipeline.ExecutablePipeline{}
	ep.PipelineID = p.ID
	ep.VersionID = p.VersionID
	ep.Storage = ae.DB
	ep.EntryPoint = p.Pipeline.Entrypoint
	ep.FaaS = ae.FaaS
	ep.PipelineModel = p
	ep.HTTPClient = ae.HTTPClient
	ep.Remedy = ae.Remedy

	err := ep.CreateBlocks(ctx, p.Pipeline.Blocks)
	if err != nil {
		e := GetPipelineError
		return &ep, e, err
	}

	user, err := auth.UserFromContext(ctx)
	if err != nil {
		return &ep, UnauthError, err
	}

	vs := store.NewStore()

	if err != nil {
		e := RequestReadError
		return &ep, e, err
	}

	pipelineVars := vars

	parameters, err := json.Marshal(pipelineVars)
	if err != nil {
		e := PipelineRunError
		return &ep, e, err
	}

	err = ep.CreateTask(ctx, user.UserName(), false, parameters)
	if err != nil {
		e := PipelineRunError
		return &ep, e, err
	}

	//nolint:nestif //its simple
	if withStop {
		ep.Output = make(map[string]string)

		for _, item := range p.Output {
			ep.Output[item.Global] = ""
		}

		err = ep.DebugRun(ctx, vs)
		if err != nil {
			vs.AddError(err)
			return nil, PipelineExecutionError, err
		}

	} else {
		go func() {
			routineCtx := context.WithValue(context.Background(), XRequestIDHeader, ctx.Value(XRequestIDHeader))

			routineCtx = logger.WithLogger(routineCtx, log)

			err = ep.DebugRun(routineCtx, vs)
			if err != nil {
				vs.AddError(err)
			}
		}()

	}

	return &ep, 0, nil
}

func getOutputValues(ep *pipeline.ExecutablePipeline, s *store.VariableStore) interface{} {
	result := make(map[string]interface{})

	for key := range ep.Outputs() {
		if v, ok := s.Values[key]; ok {
			result[key] = v
		}
	}

	return result
}

func filterVersionsByID(scenarios []entity.EriusScenarioInfo, isAll bool, allowedKeys map[string]struct{}) []entity.EriusScenarioInfo {
	if isAll {
		return scenarios
	}

	if len(allowedKeys) == 0 {
		return []entity.EriusScenarioInfo{}
	}

	res := make([]entity.EriusScenarioInfo, 0)

	for i := range scenarios {
		if _, ok := allowedKeys[scenarios[i].VersionID.String()]; ok {
			res = append(res, scenarios[i])
		}
	}

	return res
}

func filterPipelinesByID(scenarios []entity.EriusScenarioInfo, isAll bool, allowedKeys map[string]struct{}) []entity.EriusScenarioInfo {
	if isAll {
		return scenarios
	}

	if len(allowedKeys) == 0 {
		return []entity.EriusScenarioInfo{}
	}

	res := make([]entity.EriusScenarioInfo, 0)

	for i := range scenarios {
		if _, ok := allowedKeys[scenarios[i].ID.String()]; ok {
			res = append(res, scenarios[i])
		}
	}

	return res
}

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

	grants, err := ae.AuthClient.CheckGrants(ctx, vars.Pipeline, vars.Read)
	if err != nil {
		e := AuthServiceError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if !grants.Allow {
		e := UnauthError
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

	grants, err := ae.AuthClient.CheckGrants(ctx, vars.Pipeline, vars.Update)
	if err != nil {
		e := AuthServiceError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if !grants.Allow {
		e := UnauthError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	id := pID.String()

	if !(grants.Allow && grants.Contains(id)) {
		e := UnauthError
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

	grants, err := ae.AuthClient.CheckGrants(ctx, vars.Pipeline, vars.Update)
	if err != nil {
		e := AuthServiceError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if !grants.Allow {
		e := UnauthError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	id := pID.String()

	if !(grants.Allow && grants.Contains(id)) {
		e := UnauthError
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

func scenarioUsage(ctx context.Context, pipelineStorager db.PipelineStorager, id uuid.UUID) ([]entity.EriusScenario, error) {
	ctx, span := trace.StartSpan(ctx, "scenario usage")
	defer span.End()

	p, err := pipelineStorager.GetPipeline(ctx, id)
	if err != nil {
		return nil, errors.WithMessage(err, "unable to get pipeline")
	}

	workedVersions, err := pipelineStorager.GetWorkedVersions(ctx)
	if err != nil {
		return nil, err
	}

	res := make([]entity.EriusScenario, 0)

	for i := range workedVersions {
		for j := range workedVersions[i].Pipeline.Blocks {
			block := workedVersions[i].Pipeline.Blocks[j]
			if block.BlockType == script.TypeScenario &&
				block.Title == p.Name {
				res = append(res, workedVersions[i])

				break
			}
		}
	}

	return res, nil
}

// Metrics godoc
// @Summary metrics
// @Tags metrics
// @Description Метрики
// @ID metrics
// @Produce plain
// @Success 200 "metrics content"
// @Router /api/pipeliner/v1/metrics [get]
func (ae *APIEnv) ServePrometheus() http.Handler {
	return promhttp.Handler()
}
