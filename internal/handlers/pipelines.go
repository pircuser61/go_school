package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"go.opencensus.io/trace"

	"github.com/go-chi/chi"
	"github.com/google/uuid"

	"gitlab.services.mts.ru/erius/admin/pkg/auth"
	"gitlab.services.mts.ru/erius/admin/pkg/vars"
	"gitlab.services.mts.ru/erius/monitoring/pkg/monitor"
	"gitlab.services.mts.ru/erius/monitoring/pkg/pipeliner/monitoring"

	"gitlab.services.mts.ru/erius/pipeliner/internal/db"
	"gitlab.services.mts.ru/erius/pipeliner/internal/entity"
	"gitlab.services.mts.ru/erius/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/erius/pipeliner/internal/store"
)

const (
	statusRunned = "runned"
)

var errPipelineNotEditable = errors.New("pipeline is not editable")

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

	resp := entity.EriusScenarioList{
		Pipelines: approved,
		OnApprove: onApprove,
		Drafts:    drafts,
		Tags:      nil,
	}

	if err := sendResponse(w, http.StatusOK, resp); err != nil {
		e := UnknownError
		ae.Logger.Error(e.errorMessage(err))
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

	grants, err := ae.AuthClient.CheckGrants(ctx, vars.PipelineVersion, vars.Own)
	if err != nil {
		ae.Logger.Error(AuthServiceError.errorMessage(err))

		return []entity.EriusScenarioInfo{}, &PipelinerError{AuthServiceError}
	}

	if !grants.Allow {
		ae.Logger.Error(UnauthError.errorMessage(err))

		return []entity.EriusScenarioInfo{}, nil
	}

	drafts, err := ae.DB.GetDraftVersions(ctx)
	if err != nil {
		return []entity.EriusScenarioInfo{}, &PipelinerError{GetAllDraftsError}
	}

	return filterVersionsByID(drafts, grants.All, grants.Items), nil
}

// onApprovedVersions выбирает версии сценариев с признаком OnApprove,
// разрешенные для данного пользователя
//nolint:dupl //different logic
func (ae *APIEnv) onApprovedVersions(ctx context.Context) ([]entity.EriusScenarioInfo, *PipelinerError) {
	ctx, s := trace.StartSpan(ctx, "list_on_approve_versions")
	defer s.End()

	grants, err := ae.AuthClient.CheckGrants(ctx, vars.Pipeline, vars.Approve)
	if err != nil {
		ae.Logger.Error(AuthServiceError.errorMessage(err))

		return []entity.EriusScenarioInfo{}, &PipelinerError{AuthServiceError}
	}

	if !grants.Allow {
		ae.Logger.Error(UnauthError.errorMessage(err))

		return []entity.EriusScenarioInfo{}, nil
	}

	onApprove, err := ae.DB.GetOnApproveVersions(ctx)
	if err != nil {
		ae.Logger.Error(GetAllOnApproveError.errorMessage(err))

		return []entity.EriusScenarioInfo{}, &PipelinerError{GetAllOnApproveError}
	}

	return filterVersionsByID(onApprove, grants.All, grants.Items), nil
}

// approvedVersions выбирает последние рабочие версии сценариев,
// разрешенные для данного пользователя
//nolint:dupl //different logic
func (ae *APIEnv) approvedVersions(ctx context.Context) ([]entity.EriusScenarioInfo, *PipelinerError) {
	ctx, s := trace.StartSpan(ctx, "list_approved_versions")
	defer s.End()

	grants, err := ae.AuthClient.CheckGrants(ctx, vars.Pipeline, vars.Read)
	if err != nil {
		ae.Logger.Error(AuthServiceError.errorMessage(err))

		return []entity.EriusScenarioInfo{}, &PipelinerError{AuthServiceError}
	}

	if !grants.Allow {
		ae.Logger.Error(UnauthError.errorMessage(err))

		return []entity.EriusScenarioInfo{}, nil
	}

	approved, err := ae.DB.GetApprovedVersions(ctx)
	if err != nil {
		ae.Logger.Error(GetAllApprovedError.errorMessage(err))

		return []entity.EriusScenarioInfo{}, &PipelinerError{GetAllApprovedError}
	}

	return filterVersionsByID(approved, grants.All, grants.Items), nil
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

	idParam := chi.URLParam(req, "versionID")

	id, err := uuid.Parse(idParam)
	if err != nil {
		e := UUIDParsingError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	p, err := ae.DB.GetPipelineVersion(ctx, id)
	if err != nil {
		e := GetVersionError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	grants, err := ae.AuthClient.CheckGrants(ctx, vars.Pipeline, vars.Read)
	if err != nil {
		e := AuthServiceError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	// проверяем доступ к сценарию запрошенной версии
	if !(grants.Allow && grants.Contains(p.ID.String())) {
		e := UnauthError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = sendResponse(w, http.StatusOK, p)
	if err != nil {
		e := UnknownError
		ae.Logger.Error(e.errorMessage(err))
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

	idParam := chi.URLParam(req, "pipelineID")

	id, err := uuid.Parse(idParam)
	if err != nil {
		e := UUIDParsingError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	grants, err := ae.AuthClient.CheckGrants(ctx, vars.Pipeline, vars.Read)
	if err != nil {
		e := AuthServiceError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	// проверяем доступ на чтение запрошенного сценария
	if !(grants.Allow && grants.Contains(idParam)) {
		e := UnauthError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	p, err := ae.DB.GetPipeline(ctx, id)
	if err != nil {
		e := GetPipelineError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = sendResponse(w, http.StatusOK, p)
	if err != nil {
		e := UnknownError
		ae.Logger.Error(e.errorMessage(err))
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

	b, err := ioutil.ReadAll(req.Body)
	defer req.Body.Close()

	if err != nil {
		e := RequestReadError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	p := entity.EriusScenario{}

	err = json.Unmarshal(b, &p)
	if err != nil {
		e := PipelineParseError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	pipelineID := chi.URLParam(req, "pipelineID")

	p.ID, err = uuid.Parse(pipelineID)
	if err != nil {
		e := VersionCreateError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	p.VersionID = uuid.New()

	grants, err := ae.AuthClient.CheckGrants(ctx, vars.Pipeline, vars.Create)
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

	user, err := auth.UserFromContext(ctx)
	if err != nil {
		ae.Logger.WithError(err).Error("user failed")
	}

	err = ae.DB.CreateVersion(ctx, &p, user.UserName(), b)
	if err != nil {
		e := PipelineWriteError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	created, err := ae.DB.GetPipelineVersion(ctx, p.VersionID)
	if err != nil {
		e := PipelineReadError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if err = ae.AuthClient.Notice(ctx, &auth.Notice{
		NoticeType:   vars.CreateNotice,
		ResourceType: vars.PipelineVersion,
		ResourceID:   created.VersionID.String(),
	}); err != nil {
		e := AuthServiceError
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

	b, err := ioutil.ReadAll(req.Body)
	defer req.Body.Close()

	if err != nil {
		e := RequestReadError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	p := entity.EriusScenario{}

	err = json.Unmarshal(b, &p)
	if err != nil {
		e := PipelineParseError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	grants, err := ae.AuthClient.CheckGrants(ctx, vars.Pipeline, vars.Create)
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

	user, err := auth.UserFromContext(ctx)
	if err != nil {
		ae.Logger.Error("user failed: ", err.Error())
	}

	p.ID = uuid.New()
	p.VersionID = uuid.New()

	err = ae.DB.CreatePipeline(ctx, &p, user.UserName(), b)
	if err != nil {
		e := PipelineCreateError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	created, err := ae.DB.GetPipelineVersion(ctx, p.VersionID)
	if err != nil {
		e := PipelineReadError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if err = ae.AuthClient.Notice(ctx, &auth.Notice{
		NoticeType:   vars.CreateNotice,
		ResourceType: vars.PipelineVersion,
		ResourceID:   created.VersionID.String(),
	}); err != nil {
		e := AuthServiceError
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
func (ae *APIEnv) EditVersion(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "edit_draft")
	defer s.End()

	b, err := ioutil.ReadAll(req.Body)
	defer req.Body.Close()

	if err != nil {
		e := RequestReadError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	p := entity.EriusScenario{}

	err = json.Unmarshal(b, &p)
	if err != nil {
		e := PipelineParseError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	canEdit, err := ae.DB.VersionEditable(ctx, p.VersionID)
	if err != nil {
		e := UnknownError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if !canEdit {
		e := PipelineIsDraft

		ae.Logger.Error(e.errorMessage(errPipelineNotEditable))
		_ = e.sendError(w)

		return
	}

	resource, action, id := authUpdateParametersByPipelineStatus(&p)

	grants, err := ae.AuthClient.CheckGrants(ctx, resource, action)
	if err != nil {
		e := AuthServiceError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if !(grants.Allow && grants.Contains(id)) {
		e := UnauthError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = ae.DB.UpdateDraft(ctx, &p, b)
	if err != nil {
		e := PipelineWriteError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	user, err := auth.UserFromContext(ctx)
	if err != nil {
		ae.Logger.Error(err.Error())
	}

	if p.Status == db.StatusApproved {
		err = ae.DB.SwitchApproved(ctx, p.ID, p.VersionID, user.UserName())
		if err != nil {
			e := ApproveError
			ae.Logger.Error(e.errorMessage(err))
			_ = e.sendError(w)

			return
		}
	}

	edited, err := ae.DB.GetPipelineVersion(ctx, p.VersionID)
	if err != nil {
		e := PipelineReadError
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

func authUpdateParametersByPipelineStatus(p *entity.EriusScenario) (resource vars.ResourceType, action vars.ActionType, id string) {
	switch p.Status {
	case db.StatusDraft, db.StatusOnApprove:
		resource = vars.PipelineVersion
		action = vars.Own
		id = p.VersionID.String()
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

	idParam := chi.URLParam(req, "versionID")

	versionID, err := uuid.Parse(idParam)
	if err != nil {
		e := UUIDParsingError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	p, err := ae.DB.GetPipelineVersion(ctx, versionID)
	if err != nil {
		e := PipelineDeleteError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	resource, action, id := authDeleteParametersByPipelineStatus(p)

	grants, err := ae.AuthClient.CheckGrants(ctx, resource, action)
	if err != nil {
		e := AuthServiceError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if !(grants.Allow && grants.Contains(id)) {
		e := UnauthError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = ae.DB.DeleteVersion(ctx, versionID)
	if err != nil {
		e := PipelineDeleteError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if err = ae.AuthClient.Notice(ctx, &auth.Notice{
		NoticeType:   vars.DeleteNotice,
		ResourceType: vars.PipelineVersion,
		ResourceID:   versionID.String(),
	}); err != nil {
		e := AuthServiceError
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

	idParam := chi.URLParam(req, "pipelineID")

	id, err := uuid.Parse(idParam)
	if err != nil {
		e := UUIDParsingError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	grants, err := ae.AuthClient.CheckGrants(ctx, vars.Pipeline, vars.Delete)
	if err != nil {
		e := AuthServiceError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if !(grants.Allow && grants.Contains(id.String())) {
		e := UnauthError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = ae.DB.DeletePipeline(ctx, id)
	if err != nil {
		e := PipelineDeleteError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if err = ae.AuthClient.Notice(ctx, &auth.Notice{
		NoticeType:   vars.DeleteNotice,
		ResourceType: vars.Pipeline,
		ResourceID:   id.String(),
	}); err != nil {
		e := AuthServiceError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = sendResponse(w, http.StatusOK, id)
	if err != nil {
		e := UnknownError
		ae.Logger.Error(e.errorMessage(err))
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

	withStop := false

	if withStopCtx := req.Context().Value("with_stop"); withStopCtx != nil {
		withStop = true
	}

	idParam := chi.URLParam(req, "pipelineID")

	id, err := uuid.Parse(idParam)
	if err != nil {
		e := UUIDParsingError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	p, err := ae.DB.GetPipeline(ctx, id)
	if err != nil {
		e := GetPipelineError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	grants, err := ae.AuthClient.CheckGrants(ctx, vars.Pipeline, vars.Run)
	if err != nil {
		e := AuthServiceError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	// проверяем права на запуск пайплайна
	if !(grants.Allow && grants.Contains(p.ID.String())) {
		e := UnauthError
		ae.Logger.Error(e.errorMessage(err))
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

	idParam := chi.URLParam(req, "versionID")

	id, err := uuid.Parse(idParam)
	if err != nil {
		e := UUIDParsingError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	p, err := ae.DB.GetPipelineVersion(ctx, id)
	if err != nil {
		e := GetPipelineError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	grants, err := ae.AuthClient.CheckGrants(ctx, vars.Pipeline, vars.Run)
	if err != nil {
		e := AuthServiceError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	// проверяем права на запуск пайплайна
	if !(grants.Allow && grants.Contains(p.ID.String())) {
		e := UnauthError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	ae.execVersion(ctx, w, req, p, false)
}

// GetPipelineTasks
// @Summary Get Pipeline Tasks
// @Description Получить задачи по сценарию
// @Tags pipeline tasks
// @ID      get-pipeline-tasks
// @Produce json
// @Param pipelineID path string true "Pipeline ID"
// @success 200 {object} httpResponse{data=entity.EriusTasks}
// @Failure 400 {object} httpError
// @Failure 401 {object} httpError
// @Failure 500 {object} httpError
// @Router /tasks/{pipelineID} [get]
//nolint:dupl //diff logic
func (ae *APIEnv) GetPipelineTasks(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "get_pipeline_logs")
	defer s.End()

	idParam := chi.URLParam(req, "pipelineID")

	id, err := uuid.Parse(idParam)
	if err != nil {
		e := UUIDParsingError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	resp, err := ae.DB.GetPipelineTasks(ctx, id)
	if err != nil {
		e := GetTasksError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if err := sendResponse(w, http.StatusOK, resp); err != nil {
		e := UnknownError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

// GetVersionTasks
// @Summary Get Version Tasks
// @Description Получить задачи по версии сценарию
// @Tags version tasks
// @ID      get-version-tasks
// @Produce json
// @Param versionID path string true "Version ID"
// @success 200 {object} httpResponse{data=entity.EriusTasks}
// @Failure 400 {object} httpError
// @Failure 401 {object} httpError
// @Failure 500 {object} httpError
// @Router /tasks/version/{pipelineID} [get]
//nolint:dupl //diff logic
func (ae *APIEnv) GetVersionTasks(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "get_version_logs")
	defer s.End()

	idParam := chi.URLParam(req, "versionID")

	id, err := uuid.Parse(idParam)
	if err != nil {
		e := UUIDParsingError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	resp, err := ae.DB.GetVersionTasks(ctx, id)
	if err != nil {
		e := GetTasksError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if err := sendResponse(w, http.StatusOK, resp); err != nil {
		e := UnknownError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

// GetTaskLog
// @Summary Get Task Log
// @Description Получить логи по задаче
// @Tags tasks log
// @ID      get-task-log
// @Produce json
// @Param versionID path string true "Task ID"
// @success 200 {object} httpResponse{data=entity.EriusLog}
// @Failure 400 {object} httpError
// @Failure 401 {object} httpError
// @Failure 500 {object} httpError
// @Router /logs/version/{pipelineID} [get]
//nolint:dupl //difff logic
func (ae *APIEnv) GetTaskLog(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "get_version_logs")
	defer s.End()

	idParam := chi.URLParam(req, "taskID")

	id, err := uuid.Parse(idParam)
	if err != nil {
		e := UUIDParsingError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	resp, err := ae.DB.GetTaskLog(ctx, id)
	if err != nil {
		e := GetLogError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if err := sendResponse(w, http.StatusOK, resp); err != nil {
		e := UnknownError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

//nolint //need big cyclo,need equal string for all usages
func (ae *APIEnv) execVersion(ctx context.Context, w http.ResponseWriter, req *http.Request,
	p *entity.EriusScenario, withStop bool) {
	ctx, s := trace.StartSpan(ctx, "exec_version")
	defer s.End()

	reqID := req.Header.Get(XRequestIDHeader)

	mon := monitoring.New()
	mon.Set(reqID, monitor.PipelinerData{
		PipelineUUID: p.ID.String(),
		VersionUUID:  p.VersionID.String(),
		Name:         p.Name,
	})

	ctx = context.WithValue(ctx, XRequestIDHeader, reqID)

	ep := pipeline.ExecutablePipeline{}
	ep.PipelineID = p.ID
	ep.VersionID = p.VersionID
	ep.Storage = ae.DB
	ep.EntryPoint = p.Pipeline.Entrypoint
	ep.Logger = ae.Logger
	ep.FaaS = ae.FaaS
	ep.PipelineModel = p
	ep.HTTPClient = ae.HTTPClient
	ep.Remedy = ae.Remedy

	err := ep.CreateBlocks(ctx, p.Pipeline.Blocks)
	if err != nil {
		e := GetPipelineError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		if monErr := mon.RunError(ctx); monErr != nil {
			ae.Logger.WithError(monErr).Error("can't send data to monitoring")
		}

		return
	}

	ae.Logger.Info("--- running pipeline:", p.Name)

	user, err := auth.UserFromContext(ctx)
	if err != nil {
		ae.Logger.Error(err)
	}

	err = ep.CreateWork(ctx, user.UserName())
	if err != nil {
		e := PipelineRunError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		if monErr := mon.RunError(ctx); monErr != nil {
			ae.Logger.WithError(monErr).Error("can't send data to monitoring")
		}

		return
	}

	vs := store.NewStore()

	b, err := ioutil.ReadAll(req.Body)
	defer req.Body.Close()

	if err != nil {
		e := RequestReadError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		if monErr := mon.RunError(ctx); monErr != nil {
			ae.Logger.WithError(monErr).Error("can't send data to monitoring")
		}

		return
	}

	pipelineVars := make(map[string]interface{})

	if len(b) != 0 {
		err = json.Unmarshal(b, &pipelineVars)
		if err != nil {
			e := PipelineRunError
			ae.Logger.Error(e.errorMessage(err))
			_ = e.sendError(w)

			if monErr := mon.RunError(ctx); monErr != nil {
				ae.Logger.WithError(monErr).Error("can't send data to monitoring")
			}

			return
		}

		for key, value := range pipelineVars {
			vs.SetValue(p.Name+"."+key, value)
			fmt.Println(vs)
		}
	}
	//nolint:nestif //its simple
	if withStop {
		err = ep.DebugRun(ctx, vs)
		if err != nil {
			ae.Logger.Error(PipelineExecutionError.errorMessage(err))
			vs.AddError(err)
		}

		err = sendResponse(w, http.StatusOK, entity.RunResponse{
			PipelineID: ep.PipelineID, TaskID: ep.WorkID,
			Status: statusRunned,
		})
		if err != nil {
			e := UnknownError
			ae.Logger.Error(e.errorMessage(err))
			_ = e.sendError(w)

			return
		}
	} else {
		go func() {
			routineCtx := context.WithValue(context.Background(), XRequestIDHeader, ctx.Value(XRequestIDHeader))

			if monErr := mon.Run(routineCtx); monErr != nil {
				ae.Logger.WithError(monErr).Error("can't send data to monitoring")
			}

			err = ep.DebugRun(routineCtx, vs)
			if err != nil {
				ae.Logger.Error(PipelineExecutionError.errorMessage(err))
				vs.AddError(err)

				if monErr := mon.RunError(routineCtx); monErr != nil {
					ae.Logger.WithError(monErr).Error("can't send data to monitoring")
				}
			}

			if monErr := mon.Done(routineCtx); monErr != nil {
				ae.Logger.WithError(monErr).Error("can't send data to monitoring")
			}
		}()

		err = sendResponse(w, http.StatusOK, entity.RunResponse{
			PipelineID: ep.PipelineID, TaskID: ep.WorkID,
			Status: statusRunned,
		})
		if err != nil {
			e := UnknownError
			ae.Logger.Error(e.errorMessage(err))
			_ = e.sendError(w)

			return
		}
	}
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
