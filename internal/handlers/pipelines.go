package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
	"gitlab.services.mts.ru/erius/pipeliner/internal/db"
	"gitlab.services.mts.ru/erius/pipeliner/internal/entity"
	"gitlab.services.mts.ru/erius/pipeliner/internal/pipeline"
	"go.opencensus.io/trace"
	"io/ioutil"
	"net/http"
	"sync"
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
		fmt.Println(string(b))
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
		fmt.Println(string(b))
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
		fmt.Println(string(b))
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
	ep := pipeline.ExecutablePipeline{
		PipelineID: p.ID,
		Storage:    ae.DBConnection,
	}

	err = ep.CreateWork(c)
	if err != nil {
		e := PipelineRunError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		vs := pipeline.VariableStore{}
		err := ep.Run(c, &vs)
		if err != nil {
			ae.Logger.Error(PipelineExecutionError.errorMessage(err))
		}
		wg.Done()
	}()

	err = sendResponse(w, http.StatusOK, entity.RunResponse{PipelineID: id, TaskID: ep.WorkId, Status: "work"})
	if err != nil {
		e := UnknownError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}
	wg.Wait()
}
