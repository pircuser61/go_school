package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"gitlab.services.mts.ru/erius/pipeliner/internal/store"
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

var (
	errPipelineNotEditable = errors.New("pipeline is not editable")
)

const testAuthor = "testUser"
const testUser = "testUser"

type RunContext struct {
	ID         string            `json:"id"`
	Parameters map[string]string `json:"parameters"`
}

func (ae APIEnv) ListPipelines(w http.ResponseWriter, req *http.Request) {
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

	drafts, err := db.GetDraftVersions(c, ae.DBConnection, testAuthor)
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

// GetPipeline returns handler for GET pipelines
// if isVersion is True - returns handler for GET pipelines/version.
func (ae APIEnv) GetPipeline(isVersion bool) func(w http.ResponseWriter, req *http.Request) {
	var spanName = "get_pipeline"

	var paramKey = "pipelineID"

	var getPipelineFunction = db.GetPipeline

	var pipelineError = GetPipelineError

	if isVersion {
		spanName = "get_version"
		paramKey = "versionID"
		getPipelineFunction = db.GetPipelineVersion
		pipelineError = GetVersionError
	}

	return func(w http.ResponseWriter, req *http.Request) {
		c, s := trace.StartSpan(context.Background(), spanName)
		defer s.End()

		idparam := chi.URLParam(req, paramKey)

		id, err := uuid.Parse(idparam)
		if err != nil {
			e := UUIDParsingError
			ae.Logger.Error(e.errorMessage(err))
			_ = e.sendError(w)

			return
		}

		p, err := getPipelineFunction(c, ae.DBConnection, id)
		if err != nil {
			e := pipelineError
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
}

// PostPipeline returns handler for POST pipelines
// if isDraft is True - returns handler for POST pipelines/version.
func (ae APIEnv) PostPipeline(isDraft bool) func(w http.ResponseWriter, req *http.Request) {
	var spanName = "create_pipeline"

	var createFunction = db.CreatePipeline

	var pipelineError = PipelineCreateError

	if isDraft {
		spanName = "create_draft"
		createFunction = db.CreateVersion
		pipelineError = PipelineWriteError
	}

	return func(w http.ResponseWriter, req *http.Request) {
		ctx, s := trace.StartSpan(context.Background(), spanName)
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

		p.ID = uuid.New()
		p.VersionID = uuid.New()

		err = createFunction(ctx, ae.DBConnection, &p, testAuthor, b)
		if err != nil {
			e := pipelineError
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
}

func (ae APIEnv) EditDraft(w http.ResponseWriter, req *http.Request) {
	c, s := trace.StartSpan(context.Background(), "edit_draft")
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

		err = errPipelineNotEditable
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
		err = db.SwitchApproved(c, ae.DBConnection, p.ID, p.VersionID, testAuthor)
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

func (ae APIEnv) DeleteVersion(w http.ResponseWriter, req *http.Request) {
	c, s := trace.StartSpan(context.Background(), "delete_version")
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
		err = errPipelineNotEditable
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

func (ae APIEnv) DeletePipeline(w http.ResponseWriter, req *http.Request) {
	c, s := trace.StartSpan(context.Background(), "delete_pipeline")
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

func (ae APIEnv) CreatePipeline(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(context.Background(), "create_pipeline")
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

	p.ID = uuid.New()
	p.VersionID = uuid.New()

	err = db.CreatePipeline(ctx, ae.DBConnection, &p, testAuthor, b)
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

func (ae APIEnv) RunPipeline(w http.ResponseWriter, req *http.Request) {
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

	ae.execVersion(c, w, req, p, false)
}

func (ae APIEnv) execVersion(c context.Context, w http.ResponseWriter, req *http.Request,
	p *entity.EriusScenario, withStop bool) {
	c, s := trace.StartSpan(c, "exec_version")
	defer s.End()

	ep := pipeline.ExecutablePipeline{}
	ep.PipelineID = p.ID
	ep.VersionID = p.VersionID
	ep.Storage = ae.DBConnection
	ep.Entrypoint = p.Pipeline.Entrypoint
	ep.Logger = ae.Logger
	ep.FaaS = ae.FaaS

	err := ep.CreateBlocks(c, p.Pipeline.Blocks)
	if err != nil {
		e := GetPipelineError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}

	err = ep.CreateWork(c, testUser)
	if err != nil {
		e := PipelineRunError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	vs := store.NewStore()

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
			vs.SetValue(key, value)
		}
	}
	if withStop {
		err = ep.Run(c, &vs)
		if err != nil {
			ae.Logger.Error(PipelineExecutionError.errorMessage(err))
			vs.AddError(err)
		}
		err = sendResponse(w, http.StatusOK, entity.RunResponse{PipelineID: ep.PipelineID, TaskID: ep.WorkID,
			Status: "runned"})
		if err != nil {
			e := UnknownError
			ae.Logger.Error(e.errorMessage(err))
			_ = e.sendError(w)

			return
		}

	} else {
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			err = ep.Run(c, &vs)
			if err != nil {
				ae.Logger.Error(PipelineExecutionError.errorMessage(err))
				vs.AddError(err)
			}

			wg.Done()
		}()
		out, err := vs.GrabOutput()
		if err != nil {
			e := UnknownError
			ae.Logger.Error(e.errorMessage(err))
			_ = e.sendError(w)

			return
		}

		status := "completed"

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

		var stepsResponse = make([]string, 0)

		for _, s := range steps {
			stepsResponse = append(stepsResponse, s)
		}

		err = sendResponse(w, http.StatusOK, entity.RunResponse{PipelineID: ep.PipelineID, TaskID: ep.WorkID,
			Status: status, Output: out, Steps: stepsResponse, Errors: errs})
		if err != nil {
			e := UnknownError
			ae.Logger.Error(e.errorMessage(err))
			_ = e.sendError(w)

			return
		}
		wg.Wait()
	}
}
