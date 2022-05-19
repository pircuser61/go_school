package handlers

import (
	"context"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/erius/admin/pkg/auth"
	"gitlab.services.mts.ru/erius/admin/pkg/vars"
	"gitlab.services.mts.ru/erius/monitoring/pkg/monitor"
	"gitlab.services.mts.ru/erius/monitoring/pkg/pipeliner/monitoring"
	"gitlab.services.mts.ru/erius/pipeliner/internal/db"
	"gitlab.services.mts.ru/erius/pipeliner/internal/entity"
	"gitlab.services.mts.ru/erius/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/erius/pipeliner/internal/store"
	"go.opencensus.io/trace"
	"io/ioutil"
	"net/http"
)

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

	user, err := auth.UserFromContext(ctx)
	if err != nil {
		e := NoUserInContextError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}

	ep, e, err := ae.execVersionInternal(ctx, reqID, p, pipelineVars, withStop, user.UserName())
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

func (ae *APIEnv) execVersionInternal(ctx context.Context, reqID string, p *entity.EriusScenario, vars map[string]interface{}, syncExecution bool, userName string) (*pipeline.ExecutablePipeline, Err, error) {

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

	err = ep.CreateTask(ctx, userName, false, parameters)
	if err != nil {
		e := PipelineRunError
		return &ep, e, err
	}

	//nolint:nestif //its simple
	if syncExecution {
		ep.Output = make(map[string]string)

		for _, item := range p.Output {
			ep.Output[item.Global] = ""
		}

		err = ep.Run(ctx, vs)
		if err != nil {
			vs.AddError(err)
			return nil, PipelineExecutionError, err
		}

	} else {
		go func() {
			routineCtx := context.WithValue(context.Background(), XRequestIDHeader, ctx.Value(XRequestIDHeader))

			routineCtx = logger.WithLogger(routineCtx, log)

			err = ep.Run(routineCtx, vs)
			if err != nil {
				vs.AddError(err)
			}
		}()

	}

	return &ep, 0, nil
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
