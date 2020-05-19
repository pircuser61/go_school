package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
	"gitlab.services.mts.ru/erius/pipeliner/internal/db"
	"gitlab.services.mts.ru/erius/pipeliner/internal/entity"
	"go.opencensus.io/trace"
	"io/ioutil"
	"net/http"
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
		ae.Logger.Error("can't get approved versions: ", err)
		sendError(w, err)
		return
	}

	onApprove, err := db.GetOnApproveVersions(c, ae.DBConnection)
	if err != nil {
		ae.Logger.Error("can't get versions on approve: ", err)
		sendError(w, err)
		return
	}
	author := "testuser"
	drafts, err := db.GetDraftVersions(c, ae.DBConnection, author)
	if err != nil {
		ae.Logger.Error("can't get draft versions: ", err)
		sendError(w, err)
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
		ae.Logger.Error("can't send response", err)
		sendError(w, err)
		return
	}

}

func (ae ApiEnv) GetPipeline(w http.ResponseWriter, req *http.Request) {
	c, s := trace.StartSpan(context.Background(), "list_pipelines")
	defer s.End()

	idparam := chi.URLParam(req, "pipelineID")
	id, err := uuid.Parse(idparam)
	if err != nil {
		ae.Logger.Error("can't parse version ID: ", err)
		sendError(w, err)
		return
	}

	pipeline, err := db.GetPipeline(c, ae.DBConnection, id)
	if err != nil {
		ae.Logger.Error("can't get pipeline: ", err)
		sendError(w, err)
		return
	}

	err = sendResponse(w, http.StatusOK, pipeline)
	if err != nil {
		ae.Logger.Error("can't send response", err)
		sendError(w, err)
		return
	}
}

func (ae ApiEnv) GetPipelineVersion(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(context.Background(), "list_pipelines")
	defer s.End()
	idparam := chi.URLParam(req, "versionID")
	id, err := uuid.Parse(idparam)
	if err != nil {
		ae.Logger.Error("can't parse version ID: ", err)
		sendError(w, err)
		return
	}
	version, err := db.GetPipelineVersion(ctx, ae.DBConnection, id)
	if err != nil {
		ae.Logger.Error("can't get pipeline: ", err)
		sendError(w, err)
		return
	}
	err = sendResponse(w, http.StatusOK, version)
	if err != nil {
		ae.Logger.Error("can't send response", err)
		sendError(w, err)
		return
	}
}

func (ae ApiEnv) CreateDraft(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(context.Background(), "create_draft")
	defer s.End()

	b, err := ioutil.ReadAll(req.Body)
	defer req.Body.Close()
	if err != nil {
		ae.Logger.Error("can't get created from request body ", err)
		sendError(w, err)
		return
	}
	author := "testuser"
	p := entity.EriusScenario{}
	err = json.Unmarshal(b, &p)
	if err != nil {
		ae.Logger.Error("can't unmarshal created ", err)
		sendError(w, err)
		return
	}
	p.ID = uuid.New()
	p.VersionID = uuid.New()

	err = db.CreateVersion(ctx, ae.DBConnection, &p, author, b)
	if err != nil {
		ae.Logger.Error("can't write created to database ", err)
		sendError(w, err)
		return
	}
	created, err := db.GetPipelineVersion(ctx, ae.DBConnection, p.VersionID)
	if err != nil {
		ae.Logger.Error("can't get edited version")
		sendError(w, err)
		return
	}

	err = sendResponse(w, http.StatusOK, created)
	if err != nil {
		ae.Logger.Error("can't send response", err)
		sendError(w, err)
		return
	}
}

func (ae ApiEnv) EditDraft(w http.ResponseWriter, req *http.Request) {
	c, s := trace.StartSpan(context.Background(), "list_pipelines")
	defer s.End()
	b, err := ioutil.ReadAll(req.Body)
	defer req.Body.Close()
	if err != nil {
		ae.Logger.Error("can't get pipeline from request body ", err)
		sendError(w, err)
		return
	}

	p := entity.EriusScenario{}
	err = json.Unmarshal(b, &p)
	if err != nil {
		ae.Logger.Error("can't unmarshal pipeline ", err)
		sendError(w, err)
		return
	}

	canEdit, err := db.VersionEditable(c, ae.DBConnection, p.VersionID)
	if err != nil {
		ae.Logger.Error("can't check version status ", err)
		sendError(w, err)
		return
	}
	if !canEdit {
		err = errors.New("can't edit version, please create new draft")
		ae.Logger.Error("can't edit version, please create new draft")
		sendError(w, err)
		return
	}

	err = db.UpdateDraft(c, ae.DBConnection, &p, b)
	if err != nil {
		ae.Logger.Error("can't update draft ", err)
		sendError(w, err)
		return
	}

	if p.Status == db.StatusApproved {
		author := "testuser"
		err = db.SwitchApproved(c, ae.DBConnection, p.ID, p.VersionID, author)
		if err != nil {
			ae.Logger.Error("can't approve pipeline version")
			sendError(w, err)
			return
		}
	}

	edited, err := db.GetPipelineVersion(c, ae.DBConnection, p.VersionID)
	if err != nil {
		ae.Logger.Error("can't get edited version")
		sendError(w, err)
		return
	}

	err = sendResponse(w, http.StatusOK, edited)
	if err != nil {
		ae.Logger.Error("can't send response", err)
		sendError(w, err)
		return
	}
}

func (ae ApiEnv) DeleteVersion(w http.ResponseWriter, req *http.Request) {
	c, s := trace.StartSpan(context.Background(), "list_pipelines")
	defer s.End()
	idparam := chi.URLParam(req, "versionID")
	versionID, err := uuid.Parse(idparam)
	if err != nil {
		ae.Logger.Error("can't parse version ID: ", err)
		sendError(w, err)
		return
	}

	err = db.DeleteVersion(c, ae.DBConnection, versionID)
	if err != nil {
		ae.Logger.Error("can't delete version: ", err)
		sendError(w, err)
		return
	}

	err = sendResponse(w, http.StatusOK, nil)
	if err != nil {
		ae.Logger.Error("can't send response", err)
		sendError(w, err)
		return
	}
}

func (ae ApiEnv) DeletePipeline(w http.ResponseWriter, req *http.Request) {
	c, s := trace.StartSpan(context.Background(), "list_pipelines")
	defer s.End()
	idparam := chi.URLParam(req, "pipelineID")
	id, err := uuid.Parse(idparam)
	if err != nil {
		ae.Logger.Error("can't parse version ID: ", err)
		sendError(w, err)
		return
	}

	err = db.DeletePipeline(c, ae.DBConnection, id)
	if err != nil {
		ae.Logger.Error("can't delete version: ", err)
		sendError(w, err)
		return
	}

	err = sendResponse(w, http.StatusOK, id)
	if err != nil {
		ae.Logger.Error("can't send response", err)
		return
	}

}

func (ae ApiEnv) CreatePipeline(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(context.Background(), "list_pipelines")
	defer s.End()

	b, err := ioutil.ReadAll(req.Body)
	defer req.Body.Close()
	if err != nil {
		ae.Logger.Error("can't get created from request body ", err)
		sendError(w, err)
		return
	}
	author := "testuser"
	p := entity.EriusScenario{}
	err = json.Unmarshal(b, &p)
	if err != nil {
		ae.Logger.Error("can't unmarshal created ", err)
		sendError(w, err)
		return
	}
	p.ID = uuid.New()
	p.VersionID = uuid.New()

	err = db.CreatePipeline(ctx, ae.DBConnection, &p, author, b)
	if err != nil {
		ae.Logger.Error("can't write created to database ", err)
		sendError(w, err)
		return
	}
	created, err := db.GetPipelineVersion(ctx, ae.DBConnection, p.VersionID)
	if err != nil {
		ae.Logger.Error("can't get edited version")
		sendError(w, err)
		return
	}

	err = sendResponse(w, http.StatusOK, created)
	if err != nil {
		ae.Logger.Error("can't send response", err)
		return
	}
}

func (ae ApiEnv) RunPipeline(w http.ResponseWriter, req *http.Request) {
	_, s := trace.StartSpan(context.Background(), "list_pipelines")
	defer s.End()
	err := sendResponse(w, http.StatusOK, nil)
	if err != nil {
		ae.Logger.Error("can't send response", err)
		sendError(w, err)
		return
	}
}

func (ae ApiEnv) ModuleUsage(w http.ResponseWriter, req *http.Request) {
	_, s := trace.StartSpan(context.Background(), "list_usage")
	defer s.End()
	name := chi.URLParam(req, "moduleName")

	r := entity.UsageResponse{
		Name: name,
	}




	err := sendResponse(w, http.StatusOK, r)
	if err != nil {
		ae.Logger.Error("can't send response", err)
		sendError(w, err)
		return
	}
}
