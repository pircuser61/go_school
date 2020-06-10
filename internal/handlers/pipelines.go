package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"sync"

	"github.com/go-chi/chi"
	"github.com/google/uuid"
	"gitlab.services.mts.ru/erius/pipeliner/internal/db"
	"gitlab.services.mts.ru/erius/pipeliner/internal/entity"
	"gitlab.services.mts.ru/erius/pipeliner/internal/pipeline"
	"go.opencensus.io/trace"
)

type RunContext struct {
	ID         string            `json:"id"`
	Parameters map[string]string `json:"parameters"`
}

func (ae ApiEnv) ListPipelines(w http.ResponseWriter, req *http.Request) {
	c, s := trace.StartSpan(context.Background(), "list_pipelines")
	defer s.End()
	approved, err := db.GetApprovedVersions(c, ae.DBConnection)
	if err != nil {
		e := GetAllApprovedError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}

	onApprove, err := db.GetOnApproveVersions(c, ae.DBConnection)
	if err != nil {
		e := GetAllOnApproveError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}
	author := "testuser"
	drafts, err := db.GetDraftVersions(c, ae.DBConnection, author)
	if err != nil {
		e := GetAllDraftsError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}

	resp := entity.EriusScenarioList{
		Pipelines: approved,
		Drafts:    drafts,
		OnApprove: onApprove,
		Tags:      nil,
	}

	err = sendResponse(w, http.StatusOK, resp)
	if err != nil {
		e := UnknownError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}

}

func (ae ApiEnv) GetPipeline(w http.ResponseWriter, req *http.Request) {
	c, s := trace.StartSpan(context.Background(), "list_pipelines")
	defer s.End()

	idparam := chi.URLParam(req, "pipelineID")
	id, err := uuid.Parse(idparam)
	if err != nil {
		e := UUIDParsingError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}

	p, err := db.GetPipeline(c, ae.DBConnection, id)
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

func (ae ApiEnv) GetPipelineVersion(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(context.Background(), "list_pipelines")
	defer s.End()
	idparam := chi.URLParam(req, "versionID")
	id, err := uuid.Parse(idparam)
	if err != nil {
		e := UUIDParsingError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}
	version, err := db.GetPipelineVersion(ctx, ae.DBConnection, id)
	if err != nil {
		e := GetVersionError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}
	err = sendResponse(w, http.StatusOK, version)
	if err != nil {
		e := UnknownError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}
}

func (ae ApiEnv) CreateDraft(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(context.Background(), "create_draft")
	defer s.End()

	b, err := ioutil.ReadAll(req.Body)
	defer req.Body.Close()
	if err != nil {
		e := RequestReadError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}
	author := "testuser"
	p := entity.EriusScenario{}
	err = json.Unmarshal(b, &p)
	if err != nil {
		e := PipelineParseError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}
	p.ID = uuid.New()
	p.VersionID = uuid.New()

	err = db.CreateVersion(ctx, ae.DBConnection, &p, author, b)
	if err != nil {
		e := PipelineWriteError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}
	created, err := db.GetPipelineVersion(ctx, ae.DBConnection, p.VersionID)
	if err != nil {
		e := PipelineReadError
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

func (ae ApiEnv) EditDraft(w http.ResponseWriter, req *http.Request) {
	c, s := trace.StartSpan(context.Background(), "list_pipelines")
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

	canEdit, err := db.VersionEditable(c, ae.DBConnection, p.VersionID)
	if err != nil {
		e := UnknownError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}
	if !canEdit {
		e := PipelineIsDraft
		errors.New("pipeline is not editable")
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}

	err = db.UpdateDraft(c, ae.DBConnection, &p, b)
	if err != nil {
		e := PipelineWriteError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}

	if p.Status == db.StatusApproved {
		author := "testuser"
		err = db.SwitchApproved(c, ae.DBConnection, p.ID, p.VersionID, author)
		if err != nil {
			e := ApproveError
			ae.Logger.Error(e.errorMessage(err))
			_ = e.sendError(w)
			return
		}
	}

	edited, err := db.GetPipelineVersion(c, ae.DBConnection, p.VersionID)
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

func (ae ApiEnv) DeleteVersion(w http.ResponseWriter, req *http.Request) {
	c, s := trace.StartSpan(context.Background(), "list_pipelines")
	defer s.End()
	idparam := chi.URLParam(req, "versionID")
	versionID, err := uuid.Parse(idparam)
	if err != nil {
		e := UUIDParsingError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}

	canEdit, err := db.VersionEditable(c, ae.DBConnection, versionID)
	if err != nil {
		e := UnknownError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}
	if !canEdit {
		err := errors.New("pipeline is not editable")
		e := PipelineIsDraft
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}

	err = db.DeleteVersion(c, ae.DBConnection, versionID)
	if err != nil {
		e := PipelineDeleteError
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

func (ae ApiEnv) DeletePipeline(w http.ResponseWriter, req *http.Request) {
	c, s := trace.StartSpan(context.Background(), "list_pipelines")
	defer s.End()
	idparam := chi.URLParam(req, "pipelineID")
	id, err := uuid.Parse(idparam)
	if err != nil {
		e := UUIDParsingError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}

	err = db.DeletePipeline(c, ae.DBConnection, id)
	if err != nil {
		e := PipelineDeleteError
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

func (ae ApiEnv) CreatePipeline(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(context.Background(), "list_pipelines")
	defer s.End()

	b, err := ioutil.ReadAll(req.Body)
	defer req.Body.Close()
	if err != nil {
		e := RequestReadError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}
	author := "testuser"
	p := entity.EriusScenario{}
	err = json.Unmarshal(b, &p)
	if err != nil {
		e := PipelineParseError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}
	p.ID = uuid.New()
	p.VersionID = uuid.New()

	err = db.CreatePipeline(ctx, ae.DBConnection, &p, author, b)
	if err != nil {
		e := PipelineCreateError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}
	created, err := db.GetPipelineVersion(ctx, ae.DBConnection, p.VersionID)
	if err != nil {
		e := PipelineReadError
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
func (ae ApiEnv) RunPipeline(w http.ResponseWriter, req *http.Request) {
	c, s := trace.StartSpan(context.Background(), "run_pipeline")
	defer s.End()
	idparam := chi.URLParam(req, "pipelineID")

	id, err := uuid.Parse(idparam)
	if err != nil {
		e := UUIDParsingError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}

	p, err := db.GetPipeline(c, ae.DBConnection, id)
	if err != nil {
		e := GetPipelineError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}
	ae.execVersion(c, w, req, p)
}

func (ae ApiEnv) RunVersion(w http.ResponseWriter, req *http.Request)  {
	c, s := trace.StartSpan(context.Background(), "run_version")
	defer s.End()
	idparam := chi.URLParam(req, "pipelineID")

	id, err := uuid.Parse(idparam)
	if err != nil {
		e := UUIDParsingError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}

	p, err := db.GetPipeline(c, ae.DBConnection, id)
	if err != nil {
		e := GetPipelineError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}
	ae.execVersion(c, w, req, p)
}

func (ae ApiEnv) execVersion(c context.Context, w http.ResponseWriter, req *http.Request, p *entity.EriusScenario) {
	c, s := trace.StartSpan(c, "exec_version")
	defer s.End()
	status:= "prepairing"
	testuser := "testuser"

	ep := pipeline.ExecutablePipeline{}
	ep.PipelineID = p.ID
	ep.VersionID = p.VersionID
	ep.Storage = ae.DBConnection
	ep.Entrypoint = p.Pipeline.Entrypoint
	ep.Logger = ae.Logger
	err := ep.CreateBlocks(p.Pipeline.Blocks)
	if err != nil {
		e := GetPipelineError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}
	status = "loaded"

	err = ep.CreateWork(c, testuser)
	if err != nil {
		e := PipelineRunError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return

	}
	wg := sync.WaitGroup{}
	wg.Add(1)
	vs := pipeline.NewStore()
	b, err := ioutil.ReadAll(req.Body)
	defer req.Body.Close()
	if err != nil {
		e := RequestReadError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}
	vars := make(map[string]interface{})
	if len(b) != 0 {
		err = json.Unmarshal(b, &vars)
		if err != nil {
			e := PipelineRunError
			ae.Logger.Error(e.errorMessage(err))
			_ = e.sendError(w)
			return
		}
		for key, value := range vars {
			vs.SetValue("input_0."+key, value)
		}
	}
	status = "input readed"

	//go func() {
	status = "runned"
	err = ep.Run(c, &vs)
	if err != nil {
		ae.Logger.Error(PipelineExecutionError.errorMessage(err))
		vs.AddError(err)
	}
	wg.Done()
	//}()
	out, err := vs.GrabOutput()
	if err != nil {
		e := UnknownError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}
	status = "completed"
	steps, err := vs.GrabSteps()
	if err != nil {
		e := UnknownError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}
	errs, err := vs.GrabErrors()
	if err != nil {
		e := UnknownError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}
	if len(errs) != 0 {
		status = "error"
	}
	err = sendResponse(w, http.StatusOK, entity.RunResponse{PipelineID: ep.PipelineID, TaskID: ep.WorkId,
		Status: status, Output: out, Steps: steps, Errors: errs})
	if err != nil {
		e := UnknownError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}
	wg.Wait()
}
