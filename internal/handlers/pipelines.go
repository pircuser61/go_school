package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"gitlab.services.mts.ru/erius/admin/pkg/auth"

	"gitlab.services.mts.ru/erius/monitoring/pkg/monitor"

	"gitlab.services.mts.ru/erius/monitoring/pkg/pipeliner/monitoring"

	"github.com/go-chi/chi"
	"github.com/google/uuid"
	"gitlab.services.mts.ru/erius/pipeliner/internal/db"
	"gitlab.services.mts.ru/erius/pipeliner/internal/entity"
	"gitlab.services.mts.ru/erius/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/erius/pipeliner/internal/store"
	"go.opencensus.io/trace"
)

var errPipelineNotEditable = errors.New("pipeline is not editable")

const (
	testAuthor = "testUser"
	testUser   = "testUser"
)

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
// @Failure 400 {object} httpError
// @Failure 500 {object} httpError
// @Router /pipelines/ [get]
func (ae *APIEnv) ListPipelines(w http.ResponseWriter, req *http.Request) {
	c, s := trace.StartSpan(context.Background(), "list_pipelines")
	defer s.End()

	user, err := auth.UserFromContext(c)
	if err != nil {
		ae.Logger.Errorf("user failed: %s", err.Error())
	}

	ae.Logger.Errorf("user: %s", user.UserName())

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

	drafts, err := ae.DB.GetDraftVersions(c, user.UserName())
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
// @Summary Get pipeline version
// @Description Получить версию сценария по ID
// @Tags pipeline, version
// @ID      get-version
// @Produce json
// @Param versionID path string true "Version ID"
// @success 200 {object} httpResponse{data=entity.EriusScenario}
// @Failure 400 {object} httpError
// @Failure 500 {object} httpError
// @Router /pipelines/version/{versionID} [get]
func (ae *APIEnv) GetPipeline(isVersion bool) func(w http.ResponseWriter, req *http.Request) {
	spanName := "get_pipeline"

	paramKey := "pipelineID"

	getPipelineFunction := ae.DB.GetPipeline

	pipelineError := GetPipelineError

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

// @Summary Create pipeline version
// @Description Создать новую версию сценария
// @Tags pipeline, version
// @ID      create-version
// @Accept json
// @Produce json
// @Param pipeline body entity.EriusScenario true "New version"
// @Param pipelineID path string true "Pipeline ID"
// @success 200 {object} httpResponse{data=entity.EriusScenario}
// @Failure 400 {object} httpError
// @Failure 500 {object} httpError
// @Router /pipelines/version/{pipelineID} [post]
//nolint:gocritic,deadcode,unused // need for swagger codegen
func postVersion() {}

// PostPipeline returns handler for POST pipelines
// if isDraft is True - returns handler for POST pipelines/version.
// @Summary Create pipeline
// @Description Создать новый сценарий
// @Tags pipeline
// @ID      create-pipeline
// @Accept json
// @Produce json
// @Param pipeline body entity.EriusScenario true "New scenario"
// @Success 200 {object} httpResponse{data=entity.EriusScenario}
// @Failure 400 {object} httpError
// @Failure 500 {object} httpError
// @Router /pipelines/ [post]
func (ae *APIEnv) PostPipeline(isDraft bool) func(w http.ResponseWriter, req *http.Request) {
	spanName := "create_pipeline"

	createFunction := ae.DB.CreatePipeline

	pipelineError := PipelineCreateError

	if isDraft {
		spanName = "create_draft"
		createFunction = ae.DB.CreateVersion
		pipelineError = PipelineWriteError
	}

	return func(w http.ResponseWriter, req *http.Request) {
		ctx, s := trace.StartSpan(context.Background(), spanName)
		user, err := auth.UserFromContext(ctx)
		if err != nil {
			ae.Logger.Errorf("user failed: %s", err.Error())
		}
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

		err = createFunction(ctx, &p, user.UserName(), b)
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

// @Summary Edit Draft
// @Description Изменить черновик
// @Tags pipeline
// @ID      edit-draft
// @Accept json
// @Produce json
// @Param draft body entity.EriusScenario true "New draft"
// @Success 200 {object} httpResponse{data=entity.EriusScenario}
// @Failure 400 {object} httpError
// @Failure 500 {object} httpError
// @Router /pipelines/version [put]
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

// @Summary Delete Version
// @Description Удалить версию
// @Tags version
// @ID      delete-version
// @Produce json
// @Param versionID path string true "Version ID"
// @Success 200 {object} httpResponse
// @Failure 400 {object} httpError
// @Failure 500 {object} httpError
// @Router /pipelines/version/{versionID} [delete]
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

// @Summary Delete Pipeline
// @Description Удалить сценарий
// @Tags pipeline
// @ID      delete-pipeline
// @Produce json
// @Param pipelineID path string true "Pipeline ID"
// @Success 200 {object} httpResponse
// @Failure 400 {object} httpError
// @Failure 500 {object} httpError
// @Router /pipelines/version/{pipelineID} [delete]
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
// @Failure 500 {object} httpError
// @Router /run/{pipelineID} [post]
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
// @Failure 500 {object} httpError
// @Router /run/version/{versionID} [post]
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

	reqID := req.Header.Get(XRequestIDHeader)

	mon := monitoring.Copy(monitoring.Pipeliner)
	mon.Set(reqID, monitor.PipelinerData{
		PipelineUUID: p.ID.String(),
		VersionUUID:  p.VersionID.String(),
		Name:         p.Name,
	})

	monCtx, _ := context.WithCancel(c)

	c = context.WithValue(c, XRequestIDHeader, reqID)

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

		err = mon.Fatal(monCtx)
		if err != nil {
			ae.Logger.WithError(err).Error("can't send data to monitoring")
		}

		return
	}

	ae.Logger.Println("--- running pipeline:", p.Name)

	err = ep.CreateWork(c, testUser)
	if err != nil {
		e := PipelineRunError
		ae.Logger.Error(e.errorMessage(err))
		_ = e.sendError(w)

		err = mon.Fatal(monCtx)
		if err != nil {
			ae.Logger.WithError(err).Error("can't send data to monitoring")
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

		err = mon.Fatal(monCtx)
		if err != nil {
			ae.Logger.WithError(err).Error("can't send data to monitoring")
		}

		return
	}

	vars := make(map[string]interface{})

	if len(b) != 0 {
		err = json.Unmarshal(b, &vars)
		if err != nil {
			e := PipelineRunError
			ae.Logger.Error(e.errorMessage(err))
			_ = e.sendError(w)

			err = mon.Fatal(monCtx)
			if err != nil {
				ae.Logger.WithError(err).Error("can't send data to monitoring")
			}

			return
		}

		for key, value := range vars {
			vs.SetValue(p.Name+"."+key, value)
			fmt.Println(vs)
		}
	}

	if withStop {
		err = ep.DebugRun(c, vs)
		if err != nil {
			ae.Logger.Error(PipelineExecutionError.errorMessage(err))
			vs.AddError(err)
		}

		err = sendResponse(w, http.StatusOK, entity.RunResponse{
			PipelineID: ep.PipelineID, TaskID: ep.WorkID,
			Status: "runned",
		})
		if err != nil {
			e := UnknownError
			ae.Logger.Error(e.errorMessage(err))
			_ = e.sendError(w)

			return
		}
	} else {
		go func() {
			err = mon.Run(monCtx)
			if err != nil {
				ae.Logger.WithError(err).Error("can't send data to monitoring")
			}

			err = ep.DebugRun(c, vs)
			if err != nil {
				ae.Logger.Error(PipelineExecutionError.errorMessage(err))
				vs.AddError(err)

				err = mon.Error(monCtx)
				if err != nil {
					ae.Logger.WithError(err).Error("can't send data to monitoring")
				}
			}

			err = mon.Done(monCtx)
			if err != nil {
				ae.Logger.WithError(err).Error("can't send data to monitoring")
			}
		}()

		status := "runned"

		err = sendResponse(w, http.StatusOK, entity.RunResponse{
			PipelineID: ep.PipelineID, TaskID: ep.WorkID,
			Status: status,
		})
		if err != nil {
			e := UnknownError
			ae.Logger.Error(e.errorMessage(err))
			_ = e.sendError(w)

			return
		}
	}
}
