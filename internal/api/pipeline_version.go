package api

import (
	c "context"
	"encoding/json"
	"io"
	"net/http"
	"sync"

	"github.com/google/uuid"

	"github.com/pkg/errors"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/erius/monitoring/pkg/monitor"

	"gitlab.services.mts.ru/erius/monitoring/pkg/pipeliner/monitoring"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/user"
)

const (
	defaultPage    = 1
	defaultPerPage = 10
)

func (ae *APIEnv) CreatePipelineVersion(w http.ResponseWriter, req *http.Request, pipelineID string) {
	ctx, s := trace.StartSpan(req.Context(), "create_draft")
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

	p := entity.EriusScenario{}

	err = json.Unmarshal(b, &p)
	if err != nil {
		e := PipelineParseError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	p.VersionID = uuid.New()
	p.ID, err = uuid.Parse(pipelineID)
	if err != nil {
		e := VersionCreateError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	ui, err := user.GetUserInfoFromCtx(ctx)
	if err != nil {
		log.WithError(err).Error("user failed")
	}

	updated, err := json.Marshal(p)
	if err != nil {
		e := PipelineParseError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = ae.DB.CreateVersion(ctx, &p, ui.Username, updated)
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

	err = sendResponse(w, http.StatusOK, created)
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

func (ae *APIEnv) RunVersion(w http.ResponseWriter, req *http.Request, versionID string) {
	ctx, s := trace.StartSpan(req.Context(), "run_pipeline")
	defer s.End()

	log := logger.GetLogger(ctx)

	id, err := uuid.Parse(versionID)
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

	runResponse, err := ae.execVersion(ctx, &execVersionDTO{
		version:  p,
		withStop: false,
		w:        w,
		req:      req,
	})
	if err != nil {
		e := PipelineExecutionError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	_ = sendResponse(w, http.StatusOK, entity.RunResponse{
		PipelineID: runResponse.PipelineID,
		WorkNumber: runResponse.WorkNumber,
		Status:     statusRunned,
	})
}

func (ae *APIEnv) RunVersionsByPipelineId(w http.ResponseWriter, r *http.Request) {
	ctx, s := trace.StartSpan(r.Context(), "run_versions_by_pipeline_id")
	defer s.End()

	log := logger.GetLogger(ctx)

	body, err := io.ReadAll(r.Body)
	defer r.Body.Close()

	if err != nil {
		e := RequestReadError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	req := &RunVersionsByPipelineIdRequest{}

	err = json.Unmarshal(body, req)
	if err != nil {
		e := BodyParseError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if req.PipelineId == "" {
		e := ValidationError
		log.Error(e.errorMessage(errors.New("PipelineID is empty")))
		_ = e.sendError(w)

		return
	}

	versions, err := ae.DB.GetVersionsByPipelineID(ctx, req.PipelineId)
	if err != nil {
		e := GetVersionsByBlueprintIdError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	var wg sync.WaitGroup
	wg.Add(len(versions))
	respChan := make(chan *entity.RunResponse, len(versions))

	ctx = c.WithValue(ctx, pipeline.SdApplicationDataCtx{}, pipeline.SdApplicationData{
		PipelineID:      req.PipelineId,
		Description:     req.Description,
		ApplicationBody: req.ApplicationBody,
	})

	for i := range versions {
		j := i
		go func(wg *sync.WaitGroup, version entity.EriusScenario, ch chan *entity.RunResponse) {
			defer wg.Done()

			v, execErr := ae.execVersion(ctx, &execVersionDTO{
				version:  &version,
				withStop: false,
				w:        w,
				req:      r,
			})
			if execErr != nil {
				log.Error(execErr)
				return
			}

			if v == nil {
				log.Error("run_versions_by_pipeline_id execution error")
				return
			}
			ch <- v
		}(&wg, versions[j], respChan)
	}

	wg.Wait()
	close(respChan)

	runVersions := make([]*entity.RunResponse, 0, len(versions))
	for i := range respChan {
		v := i
		runVersions = append(runVersions, v)
	}

	err = sendResponse(w, http.StatusOK, runVersions)
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

func (ae *APIEnv) RunNewVersionByPrevVersion(w http.ResponseWriter, r *http.Request) {
	ctx, s := trace.StartSpan(r.Context(), "run_new_version_by_prev_version")
	defer s.End()

	log := logger.GetLogger(ctx)

	body, err := io.ReadAll(r.Body)
	defer r.Body.Close()

	if err != nil {
		e := RequestReadError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	req := &RunNewVersionByPrevVersionRequest{}

	err = json.Unmarshal(body, req)
	if err != nil {
		e := BodyParseError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if req.PipelineId == "" {
		e := ValidationError
		log.Error(e.errorMessage(errors.New("PipelineId is empty")))
		_ = e.sendError(w)

		return
	}

	if req.WorkNumber == "" {
		e := ValidationError
		log.Error(e.errorMessage(errors.New("workNumber is empty")))
		_ = e.sendError(w)

		return
	}

	version, err := ae.DB.GetVersionByWorkNumber(ctx, req.WorkNumber)
	if err != nil {
		e := GetVersionsByWorkNumberError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	ctx = c.WithValue(ctx, pipeline.SdApplicationDataCtx{}, pipeline.SdApplicationData{
		PipelineID:      req.PipelineId,
		Description:     req.Description,
		ApplicationBody: req.ApplicationBody,
	})

	started, execErr := ae.execVersion(ctx, &execVersionDTO{
		version:     version,
		withStop:    false,
		w:           w,
		req:         r,
		makeNewWork: true,
		workNumber:  req.WorkNumber,
	})
	if execErr != nil {
		e := UnknownError
		log.Error(e.errorMessage(execErr))
		_ = e.sendError(w)
		return
	}

	if started == nil {
		e := UnknownError
		log.Error(e.errorMessage(errors.New("no one version was started")))
		_ = e.sendError(w)
		return
	}

	err = sendResponse(w, http.StatusOK, started)
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}
}

func (ae *APIEnv) DeleteVersion(w http.ResponseWriter, req *http.Request, versionID string) {
	ctx, s := trace.StartSpan(req.Context(), "delete_version")
	defer s.End()

	log := logger.GetLogger(ctx)

	vID, err := uuid.Parse(versionID)
	if err != nil {
		e := UUIDParsingError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	p, err := ae.DB.GetPipelineVersion(ctx, vID)
	if err != nil {
		e := PipelineDeleteError
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

	err = ae.DB.DeleteVersion(ctx, vID)
	if err != nil {
		e := PipelineDeleteError
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

//nolint:dupl //its not duplicate
func (ae *APIEnv) GetPipelineVersion(w http.ResponseWriter, req *http.Request, versionID string) {
	ctx, s := trace.StartSpan(req.Context(), "get_pipeline_version")
	defer s.End()

	log := logger.GetLogger(ctx)

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

	err = sendResponse(w, http.StatusOK, p)
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

func (ae *APIEnv) EditVersion(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "edit_draft")
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

	p := entity.EriusScenario{}

	err = json.Unmarshal(b, &p)
	if err != nil {
		e := PipelineParseError
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

	ui, err := user.GetUserInfoFromCtx(ctx)
	if err != nil {
		log.Error(err.Error())
	}

	if p.Status == db.StatusApproved {
		err = ae.DB.SwitchApproved(ctx, p.ID, p.VersionID, ui.Username)
		if err != nil {
			e := ApproveError
			log.Error(e.errorMessage(err))
			_ = e.sendError(w)

			return
		}
	}

	if p.Status == db.StatusRejected {
		err = ae.DB.SwitchRejected(ctx, p.VersionID, p.CommentRejected, ui.Username)
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

type execVersionDTO struct {
	version  *entity.EriusScenario
	withStop bool

	w   http.ResponseWriter
	req *http.Request

	makeNewWork bool
	workNumber  string
}

// nolint //need big cyclo,need equal string for all usages
func (ae *APIEnv) execVersion(ctx c.Context, dto *execVersionDTO) (*entity.RunResponse, error) {
	_, s := trace.StartSpan(ctx, "exec_version")
	defer s.End()

	log := logger.GetLogger(ctx)

	reqID := dto.req.Header.Get(XRequestIDHeader)

	b, err := io.ReadAll(dto.req.Body)
	if err != nil {
		return nil, err
	}

	defer dto.req.Body.Close()

	mon := monitoring.New()
	mon.Set(reqID, monitor.PipelinerData{
		PipelineUUID: dto.version.ID.String(),
		VersionUUID:  dto.version.VersionID.String(),
		Name:         dto.version.Name,
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
			_ = e.sendError(dto.w)
		}
	}

	log.Info("--- running pipeline:", dto.version.Name)

	usr, err := user.GetUserInfoFromCtx(ctx)
	if err != nil {
		e := NoUserInContextError
		log.Error(e.errorMessage(err))
		return nil, errors.Wrap(err, e.error())
	}

	arg := &execVersionInternalDTO{
		reqID:         reqID,
		p:             dto.version,
		vars:          pipelineVars,
		syncExecution: dto.withStop,
		userName:      usr.Username,
		makeNewWork:   dto.makeNewWork,
		workNumber:    dto.workNumber,
	}

	executablePipeline, e, err := ae.execVersionInternal(ctx, arg)
	if err != nil {
		log.Error(e.errorMessage(err))
		return nil, errors.Wrap(err, e.error())
	}

	return &entity.RunResponse{
		PipelineID: executablePipeline.PipelineID,
		WorkNumber: executablePipeline.WorkNumber,
		Status:     statusRunned,
	}, nil
}

type execVersionInternalDTO struct {
	reqID         string
	p             *entity.EriusScenario
	vars          map[string]interface{}
	syncExecution bool
	userName      string
	makeNewWork   bool
	workNumber    string
}

func (ae *APIEnv) execVersionInternal(ctx c.Context, dto *execVersionInternalDTO) (*pipeline.ExecutablePipeline, Err, error) {
	log := logger.GetLogger(ctx)

	//nolint:staticcheck // поправить потом
	ctx = c.WithValue(ctx, XRequestIDHeader, dto.reqID)

	ep := pipeline.ExecutablePipeline{}
	ep.PipelineID = dto.p.ID
	ep.VersionID = dto.p.VersionID
	ep.Storage = ae.DB
	ep.EntryPoint = dto.p.Pipeline.Entrypoint
	ep.FaaS = ae.FaaS
	ep.PipelineModel = dto.p
	ep.HTTPClient = ae.HTTPClient
	ep.Remedy = ae.Remedy
	ep.ActiveBlocks = map[string]struct{}{}
	ep.SkippedBlocks = map[string]struct{}{}
	ep.EntryPoint = pipeline.BlockGoFirstStart
	ep.Sender = ae.Mail
	ep.People = ae.People
	ep.Name = dto.p.Name
	ep.Initiator = dto.userName
	ep.ServiceDesc = ae.ServiceDesc

	if dto.makeNewWork {
		ep.WorkNumber = dto.workNumber
	}

	err := ep.CreateBlocks(ctx, dto.p.Pipeline.Blocks)
	if err != nil {
		e := GetPipelineError
		return &ep, e, err
	}

	variableStorage := store.NewStore()

	pipelineVars := dto.vars

	parameters, err := json.Marshal(pipelineVars)
	if err != nil {
		e := PipelineRunError
		return &ep, e, err
	}

	if err = ep.CreateTask(ctx, &pipeline.CreateTaskDTO{
		Author:     dto.userName,
		IsDebug:    false,
		Params:     parameters,
		WorkNumber: dto.workNumber,
	}); err != nil {
		e := PipelineRunError
		return &ep, e, err
	}

	//nolint:nestif //its simple
	if dto.syncExecution {
		ep.Output = make(map[string]string)

		for _, item := range dto.p.Output {
			ep.Output[item.Global] = ""
		}

		err = ep.Run(ctx, variableStorage)
		if err != nil {
			variableStorage.AddError(err)
			return nil, PipelineExecutionError, err
		}
	} else {
		go func() {
			//nolint:staticcheck // поправить потом TODO
			routineCtx := c.WithValue(c.Background(), XRequestIDHeader, ctx.Value(XRequestIDHeader))
			routineCtx = c.WithValue(routineCtx, pipeline.SdApplicationDataCtx{}, ctx.Value(pipeline.SdApplicationDataCtx{}))
			routineCtx = logger.WithLogger(routineCtx, log)
			err = ep.Run(routineCtx, variableStorage)
			if err != nil {
				variableStorage.AddError(err)
			}
		}()
	}
	return &ep, 0, nil
}

func (ae *APIEnv) SearchPipelines(w http.ResponseWriter, req *http.Request, params SearchPipelinesParams) {
	ctx, s := trace.StartSpan(req.Context(), "search_pipelines")
	defer s.End()

	log := logger.GetLogger(ctx)

	if params.PipelineId == nil && params.PipelineName == nil {
		e := ValidationPipelineSearchError
		log.Error(e.errorMessage(errors.New("name and id are empty")))
		_ = e.sendError(w)

		return
	}

	items, err := ae.DB.GetPipelinesByNameOrId(ctx, toDbSearchPipelinesParams(&params))
	if err != nil {
		e := GetPipelinesSearchError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	res := &ResponsePipelineSearch{}

	for i := range items {
		res.Items = append(res.Items, SearchPipelineItem{
			Name:       &items[i].PipelineName,
			PipelineId: &items[i].PipelineId,
		})
	}

	if len(items) > 0 {
		res.Total = items[0].Total
	}

	err = sendResponse(w, http.StatusOK, res)
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

func toDbSearchPipelinesParams(in *SearchPipelinesParams) (out *db.SearchPipelineRequest) {
	var (
		page    = defaultPage
		perPage = defaultPerPage
	)

	if in.Page == nil {
		in.Page = &page
	}

	if in.PerPage == nil {
		in.PerPage = &perPage
	}

	return &db.SearchPipelineRequest{
		PipelineName: in.PipelineName,
		PipelineId:   in.PipelineId,
		Limit:        *in.PerPage,
		Offset:       (*in.Page * *in.PerPage) - *in.PerPage,
	}
}
