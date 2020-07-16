package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/google/uuid"
	"gitlab.services.mts.ru/erius/pipeliner/internal/db"
	"gitlab.services.mts.ru/erius/pipeliner/internal/entity"
	"gitlab.services.mts.ru/erius/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/erius/pipeliner/internal/store"
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

func (ae *APIEnv) ListPipelines(w http.ResponseWriter, req *http.Request) {
	c, s := trace.StartSpan(context.Background(), "list_pipelines")
	defer s.End()

	approved, err := ae.DB.GetApprovedVersions(c)
	if err != nil {
		e := GetAllApprovedError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	onApprove, err := ae.DB.GetOnApproveVersions(c)
	if err != nil {
		e := GetAllOnApproveError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	drafts, err := ae.DB.GetDraftVersions(c, testAuthor)
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
func (ae *APIEnv) GetPipeline(isVersion bool) func(w http.ResponseWriter, req *http.Request) {
	var spanName = "get_pipeline"

	var paramKey = "pipelineID"

	var getPipelineFunction = ae.DB.GetPipeline

	var pipelineError = GetPipelineError

	if isVersion {
		spanName = "get_version"
		paramKey = "versionID"
		getPipelineFunction = ae.DB.GetPipelineVersion
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

		p, err := getPipelineFunction(c, id)
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
func (ae *APIEnv) PostPipeline(isDraft bool) func(w http.ResponseWriter, req *http.Request) {
	var spanName = "create_pipeline"

	var createFunction = ae.DB.CreatePipeline

	var pipelineError = PipelineCreateError

	if isDraft {
		spanName = "create_draft"
		createFunction = ae.DB.CreateVersion
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

		pipelineID := chi.URLParam(req, "pipelineID")
		if pipelineID == "" {
			p.ID = uuid.New()
		} else {
			p.ID, err = uuid.Parse(pipelineID)
			if err != nil {
				e := VersionCreateError
				ae.Logger.Error(e.errorMessage(err))
				_ = e.sendError(w)

				return
			}
		}

		p.VersionID = uuid.New()

		err = createFunction(ctx, &p, testAuthor, b)
		if err != nil {
			e := pipelineError
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

		err = sendResponse(w, http.StatusOK, created)
		if err != nil {
			e := UnknownError
			ae.Logger.Error(e.errorMessage(err))
			_ = e.sendError(w)

			return
		}
	}
}

func (ae *APIEnv) EditDraft(w http.ResponseWriter, req *http.Request) {
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

	canEdit, err := ae.DB.VersionEditable(c, p.VersionID)
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

	err = ae.DB.UpdateDraft(c, &p, b)
	if err != nil {
		e := PipelineWriteError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if p.Status == db.StatusApproved {
		err = ae.DB.SwitchApproved(c, p.ID, p.VersionID, testAuthor)
		if err != nil {
			e := ApproveError
			ae.Logger.Error(e.errorMessage(err))
			_ = e.sendError(w)

			return
		}
	}

	edited, err := ae.DB.GetPipelineVersion(c, p.VersionID)
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

func (ae *APIEnv) DeleteVersion(w http.ResponseWriter, req *http.Request) {
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

	canEdit, err := ae.DB.VersionEditable(c, versionID)
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

	err = ae.DB.DeleteVersion(c, versionID)
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

func (ae *APIEnv) DeletePipeline(w http.ResponseWriter, req *http.Request) {
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

	err = ae.DB.DeletePipeline(c, id)
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

func (ae *APIEnv) CreatePipeline(w http.ResponseWriter, req *http.Request) {
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

	err = ae.DB.CreatePipeline(ctx, &p, testAuthor, b)
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

	err = sendResponse(w, http.StatusOK, created)
	if err != nil {
		e := UnknownError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

func (ae *APIEnv) RunPipeline(w http.ResponseWriter, req *http.Request) {
	c, s := trace.StartSpan(context.Background(), "run_pipeline")
	defer s.End()

	withStop := false

	if withStopCtx := req.Context().Value("with_stop"); withStopCtx != nil {
		withStop = true
	}

	idparam := chi.URLParam(req, "pipelineID")

	id, err := uuid.Parse(idparam)
	if err != nil {
		e := UUIDParsingError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	p, err := ae.DB.GetPipeline(c, id)
	if err != nil {
		e := GetPipelineError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	ae.execVersion(c, w, req, p, withStop)
}

func (ae *APIEnv) RunVersion(w http.ResponseWriter, req *http.Request) {
	c, s := trace.StartSpan(context.Background(), "run_pipeline")
	defer s.End()

	idparam := chi.URLParam(req, "versionID")

	id, err := uuid.Parse(idparam)
	if err != nil {
		e := UUIDParsingError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	p, err := ae.DB.GetPipelineVersion(c, id)
	if err != nil {
		e := GetPipelineError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	ae.execVersion(c, w, req, p, false)
}

func (ae *APIEnv) execVersion(c context.Context, w http.ResponseWriter, req *http.Request,
	p *entity.EriusScenario, withStop bool) {
	c, s := trace.StartSpan(c, "exec_version")
	defer s.End()

	ep := pipeline.ExecutablePipeline{}
	ep.PipelineID = p.ID
	ep.VersionID = p.VersionID
	ep.Storage = ae.DB
	ep.Entrypoint = p.Pipeline.Entrypoint
	ep.Logger = ae.Logger
	ep.FaaS = ae.FaaS
	ep.PipelineModel = p

	err := ep.CreateBlocks(c, p.Pipeline.Blocks)
	if err != nil {
		e := GetPipelineError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	ae.Logger.Println("--- running pipeline:", p.Name)

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
			vs.SetValue(p.Name+"."+key, value)
			fmt.Println(vs)
		}
	}

	if withStop {
		err = ep.Run(c, vs)
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
		go func() {
			err = ep.Run(c, vs)
			if err != nil {
				ae.Logger.Error(PipelineExecutionError.errorMessage(err))
				vs.AddError(err)
			}
		}()

		status := "runned"

		err = sendResponse(w, http.StatusOK, entity.RunResponse{PipelineID: ep.PipelineID, TaskID: ep.WorkID,
			Status: status})
		if err != nil {
			e := UnknownError
			ae.Logger.Error(e.errorMessage(err))
			_ = e.sendError(w)

			return
		}
	}
}
