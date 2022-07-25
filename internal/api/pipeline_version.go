package api

import (
	"context"
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

	p.ID, err = uuid.Parse(pipelineID)
	if err != nil {
		e := VersionCreateError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	p.VersionID = uuid.New()

	ui, err := user.GetUserInfoFromCtx(ctx)
	if err != nil {
		log.WithError(err).Error("user failed")
	}
	//nolint:govet //it doesn't shadow
	canCreate, err := ae.DB.DraftPipelineCreatable(ctx, p.ID, ui.Username)
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

	err = ae.DB.CreateVersion(ctx, &p, ui.Username, b)
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

	runResponse, err := ae.execVersion(ctx, w, req, p, false)
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

func (ae *APIEnv) RunVersionsByBlueprintId(w http.ResponseWriter, r *http.Request) {
	ctx, s := trace.StartSpan(r.Context(), "run_versions_by_blueprint_id")
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

	req := &RunVersionsByBlueprintIdRequest{}

	err = json.Unmarshal(body, req)
	if err != nil {
		e := BodyParseError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if req.BlueprintId == "" {
		e := ValidationError
		log.Error(e.errorMessage(errors.New("blueprintID is empty")))
		_ = e.sendError(w)

		return
	}

	versions, err := ae.DB.GetVersionsByBlueprintID(ctx, req.BlueprintId)
	if err != nil {
		e := GetVersionsByBlueprintIdError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	var wg sync.WaitGroup
	wg.Add(len(versions))
	respChan := make(chan *entity.RunResponse, len(versions))

	ctx = context.WithValue(ctx, pipeline.SdApplicationDataCtx{}, pipeline.SdApplicationData{
		BlueprintID:     req.BlueprintId,
		Description:     req.Description,
		ApplicationBody: req.ApplicationBody,
	})

	for i := range versions {
		j := i
		go func(wg *sync.WaitGroup, version entity.EriusScenario, ch chan *entity.RunResponse) {
			defer wg.Done()

			v, execErr := ae.execVersion(ctx, w, r, &version, false)
			if execErr != nil {
				log.Error(execErr)
				return
			}

			if v == nil {
				log.Error(execErr)
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

	if req.BlueprintId == "" {
		e := ValidationError
		log.Error(e.errorMessage(errors.New("blueprintID is empty")))
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

func (ae *APIEnv) GetPipelineVersion(w http.ResponseWriter, req *http.Request, versionID string) {
	ctx, s := trace.StartSpan(req.Context(), "get_version")
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

//nolint //need big cyclo,need equal string for all usages
func (ae *APIEnv) execVersion(ctx context.Context, w http.ResponseWriter, req *http.Request,
	p *entity.EriusScenario, withStop bool) (*entity.RunResponse, error) {

	_, s := trace.StartSpan(ctx, "exec_version")
	defer s.End()

	log := logger.GetLogger(ctx)

	reqID := req.Header.Get(XRequestIDHeader)

	b, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

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

	user, err := user.GetUserInfoFromCtx(ctx)
	if err != nil {
		e := NoUserInContextError
		log.Error(e.errorMessage(err))
		return nil, errors.Wrap(err, e.error())
	}

	arg := &execVersionInternalParams{
		reqID:         reqID,
		p:             p,
		vars:          pipelineVars,
		syncExecution: withStop,
		userName:      user.Username,
	}

	ep, e, err := ae.execVersionInternal(ctx, arg)
	if err != nil {
		log.Error(e.errorMessage(err))
		return nil, errors.Wrap(err, e.error())
	}

	return &entity.RunResponse{
		PipelineID: ep.PipelineID,
		WorkNumber: ep.WorkNumber,
		Status:     statusRunned,
	}, nil
}

type execVersionInternalParams struct {
	reqID         string
	p             *entity.EriusScenario
	vars          map[string]interface{}
	syncExecution bool
	userName      string
}

func (ae *APIEnv) execVersionInternal(ctx context.Context, p *execVersionInternalParams) (*pipeline.ExecutablePipeline, Err, error) {
	log := logger.GetLogger(ctx)

	//nolint:staticcheck // поправить потом
	ctx = context.WithValue(ctx, XRequestIDHeader, p.reqID)

	ep := pipeline.ExecutablePipeline{}
	ep.PipelineID = p.p.ID
	ep.VersionID = p.p.VersionID
	ep.Storage = ae.DB
	ep.EntryPoint = p.p.Pipeline.Entrypoint
	ep.FaaS = ae.FaaS
	ep.PipelineModel = p.p
	ep.HTTPClient = ae.HTTPClient
	ep.Remedy = ae.Remedy
	ep.ActiveBlocks = map[string]struct{}{}
	ep.SkippedBlocks = map[string]struct{}{}
	ep.EntryPoint = pipeline.BlockGoFirstStart
	ep.Sender = ae.Mail
	ep.People = ae.People
	ep.Name = p.p.Name
	ep.Initiator = p.userName

	err := ep.CreateBlocks(ctx, p.p.Pipeline.Blocks)
	if err != nil {
		e := GetPipelineError
		return &ep, e, err
	}

	vs := store.NewStore()

	pipelineVars := p.vars

	parameters, err := json.Marshal(pipelineVars)
	if err != nil {
		e := PipelineRunError
		return &ep, e, err
	}

	err = ep.CreateTask(ctx, p.userName, false, parameters)
	if err != nil {
		e := PipelineRunError
		return &ep, e, err
	}

	//nolint:nestif //its simple
	if p.syncExecution {
		ep.Output = make(map[string]string)

		for _, item := range p.p.Output {
			ep.Output[item.Global] = ""
		}

		err = ep.Run(ctx, vs)
		if err != nil {
			vs.AddError(err)
			return nil, PipelineExecutionError, err
		}
	} else {
		go func() {
			//nolint:staticcheck // поправить потом TODO
			routineCtx := context.WithValue(context.Background(), XRequestIDHeader, ctx.Value(XRequestIDHeader))
			routineCtx = context.WithValue(routineCtx, pipeline.SdApplicationDataCtx{}, ctx.Value(pipeline.SdApplicationDataCtx{}))
			routineCtx = logger.WithLogger(routineCtx, log)
			err = ep.Run(routineCtx, vs)
			if err != nil {
				vs.AddError(err)
			}
		}()
	}
	return &ep, 0, nil
}
