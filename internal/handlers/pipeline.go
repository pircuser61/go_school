package handlers

import (
	"context"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/erius/admin/pkg/auth"
	"gitlab.services.mts.ru/erius/admin/pkg/vars"
	"gitlab.services.mts.ru/erius/pipeliner/internal/db"
	"gitlab.services.mts.ru/erius/pipeliner/internal/entity"
	"gitlab.services.mts.ru/erius/pipeliner/internal/script"
	"go.opencensus.io/trace"
	"io/ioutil"
	"net/http"
	"strconv"
)

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
