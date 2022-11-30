package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/pkg/errors"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/user"
)

const copyPostfix = "копия"

func (ae *APIEnv) CreatePipeline(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "create_pipeline")
	defer s.End()

	log := logger.GetLogger(ctx)

	b, err := io.ReadAll(req.Body)
	defer func() {
		_ = req.Body.Close()
	}()

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

	userFromContext, err := user.GetUserInfoFromCtx(ctx)
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

	err = ae.DB.CreatePipeline(ctx, &p, userFromContext.Username, b)
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

	err = sendResponse(w, http.StatusOK, created)
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

//nolint:dupl // different logic (temporary saving old for compatibility)
func (ae *APIEnv) CopyPipeline(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "create_pipeline")
	defer s.End()

	log := logger.GetLogger(ctx)

	b, err := io.ReadAll(req.Body)
	defer func() {
		_ = req.Body.Close()
	}()

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

	userFromContext, err := user.GetUserInfoFromCtx(ctx)
	if err != nil {
		log.Error("user failed: ", err.Error())
	}

	p.ID = uuid.New()
	p.VersionID = uuid.New()
	p.Name = fmt.Sprintf("%s - %s", p.Name, copyPostfix)

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

	err = ae.DB.CreatePipeline(ctx, &p, userFromContext.Username, b)
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

	err = sendResponse(w, http.StatusOK, created)
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

//nolint:dupl //its not duplicate
func (ae *APIEnv) GetPipeline(w http.ResponseWriter, req *http.Request, pipelineID string) {
	ctx, s := trace.StartSpan(req.Context(), "get_pipeline")
	defer s.End()

	log := logger.GetLogger(ctx)

	id, err := uuid.Parse(pipelineID)
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

func (ae *APIEnv) ListPipelines(w http.ResponseWriter, req *http.Request, params ListPipelinesParams) {
	ctx, s := trace.StartSpan(req.Context(), "list_pipelines")
	defer s.End()

	log := logger.GetLogger(ctx)

	myPipelines := params.My != nil && *params.My

	pipelines, err := ae.listPipelines(ctx, myPipelines)
	if err != nil {
		_ = err.sendError(w)

		return
	}

	if err := sendResponse(w, http.StatusOK, pipelines); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

func (ae *APIEnv) DeletePipeline(w http.ResponseWriter, req *http.Request, pipelineID string) {
	ctx, s := trace.StartSpan(req.Context(), "delete_pipeline")
	defer s.End()

	log := logger.GetLogger(ctx)

	id, err := uuid.Parse(pipelineID)
	if err != nil {
		e := UUIDParsingError
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

	err = ae.DB.RemovePipelineTags(ctx, id)
	if err != nil {
		e := TagDetachError
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

	err = sendResponse(w, http.StatusOK, id)
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

func (ae *APIEnv) RunPipeline(w http.ResponseWriter, req *http.Request, pipelineID string) {
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

	id, err := uuid.Parse(pipelineID)
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

	runResponse, err := ae.execVersion(ctx, &execVersionDTO{
		version:  p,
		withStop: withStop,
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

func (ae *APIEnv) GetPipelineVersions(w http.ResponseWriter, req *http.Request, pipelineID string) {
	ctx, span := trace.StartSpan(req.Context(), "get_pipeline_versions")
	defer span.End()

	log := logger.GetLogger(ctx)

	id, err := uuid.Parse(pipelineID)
	if err != nil {
		e := UUIDParsingError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	vv, err := ae.DB.GetPipelineVersions(ctx, id)
	if err != nil {
		e := GetPipelineVersionsError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
	err = sendResponse(w, http.StatusOK, vv)
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

func (ae *APIEnv) RenamePipeline(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "rename_pipeline")
	defer s.End()

	log := logger.GetLogger(ctx)

	b, err := io.ReadAll(req.Body)
	defer func() {
		_ = req.Body.Close()
	}()

	if err != nil {
		e := RequestReadError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	p := PipelineRename{}

	err = json.Unmarshal(b, &p)
	if err != nil {
		e := PipelineRenameParseError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
	id, err := uuid.Parse(p.Id)
	if err != nil {
		e := UUIDParsingError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
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
	err = ae.DB.RenamePipeline(ctx, id, p.Name)
	if err != nil {
		e := PipelineRenameError
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

// listPipelines выбирает версии сценария с признаком Draft,
// разрешенные для данного пользователя
//
//nolint:dupl //diff logic
func (ae *APIEnv) listPipelines(ctx context.Context, myPipelines bool) ([]entity.EriusScenarioInfo, *PipelinerError) {
	ctx, s := trace.StartSpan(ctx, "list_drafts")
	defer s.End()

	authorLogin := ""

	if myPipelines {
		userFromContext, err := user.GetUserInfoFromCtx(ctx)
		if err != nil {
			return []entity.EriusScenarioInfo{}, &PipelinerError{NoUserInContextError}
		}

		authorLogin = userFromContext.Username
	}

	drafts, err := ae.DB.GetPipelinesWithLatestVersion(ctx, authorLogin)
	if err != nil {
		return []entity.EriusScenarioInfo{}, &PipelinerError{GetAllDraftsError}
	}

	return drafts, nil
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
