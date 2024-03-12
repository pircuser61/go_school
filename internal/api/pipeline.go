package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"

	"github.com/pkg/errors"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/user"
)

const (
	statusRunned = "runned"
	copyPostfix  = "копия"
)

const (
	ValidateParallelNodeReturnCycle       = "ParallelNodeReturnCycle"
	ValidateParallelNodeExitsNotConnected = "ParallelNodeExitsNotConnected"
	ValidateOutOfParallelNodesConnection  = "OutOfParallelNodesConnection"
	ValidateParallelOutOfStartInsert      = "ParallelOutOfStartInsert"
	ValidateParallelPathIntersected       = "ParallelPathIntersected"
)

func (ae *Env) CreatePipeline(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "create_pipeline")
	defer s.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	b, err := io.ReadAll(req.Body)

	defer req.Body.Close()

	if err != nil {
		errorHandler.handleError(RequestReadError, err)

		return
	}

	p := entity.EriusScenario{}

	err = json.Unmarshal(b, &p)
	if err != nil {
		errorHandler.handleError(PipelineParseError, err)

		return
	}

	userFromContext, err := user.GetUserInfoFromCtx(ctx)
	if err != nil {
		log.Error("user failed: ", err.Error())
	}

	p.PipelineID = uuid.New()
	p.VersionID = uuid.New()

	if len(p.Pipeline.Blocks) == 0 {
		p.Pipeline.FillEmptyPipeline()
		b, _ = json.Marshal(&p) // nolint // already unmarshalling that struct
	}

	ok, valErr := ae.validatePipeline(ctx, &p)
	if !ok && p.Status == db.StatusApproved {
		e := validateBlockTypeErrText(valErr)

		errorHandler.handleError(e, errors.New(valErr))

		return
	}

	executableFunctions, err := p.Pipeline.Blocks.GetExecutableFunctions()
	if err != nil {
		errorHandler.handleError(GetExecutableFunctionIDsError, err)

		return
	}

	hasPrivateFunction, err := ae.hasPrivateFunction(ctx, executableFunctions)
	if err != nil {
		errorHandler.handleError(GetFunctionError, err)

		return
	}

	err = ae.DB.CreatePipeline(ctx, &p, userFromContext.Username, b, uuid.Nil, hasPrivateFunction)
	if err != nil {
		if db.IsUniqueConstraintError(err) {
			errorHandler.handleError(PipelineNameUsed, err)
		}

		errorHandler.handleError(PipelineCreateError, err)

		return
	}

	created, err := ae.DB.GetPipelineVersion(ctx, p.VersionID, true)
	if err != nil {
		errorHandler.handleError(PipelineReadError, err)

		return
	}

	err = sendResponse(w, http.StatusOK, created)
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

//nolint:dupl // different logic (temporary saving old for compatibility)
func (ae *Env) CopyPipeline(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "create_pipeline")
	defer s.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	b, err := io.ReadAll(req.Body)
	defer req.Body.Close()

	if err != nil {
		errorHandler.handleError(RequestReadError, err)

		return
	}

	p := entity.EriusScenario{}

	err = json.Unmarshal(b, &p)
	if err != nil {
		errorHandler.handleError(PipelineParseError, err)

		return
	}

	userFromContext, err := user.GetUserInfoFromCtx(ctx)
	if err != nil {
		log.Error("user failed: ", err.Error())
	}

	oldVersionID := p.VersionID
	p.Name = fmt.Sprintf("%s - %s", p.Name, copyPostfix)

	updated, err := json.Marshal(p)
	if err != nil {
		errorHandler.handleError(PipelineParseError, err)

		return
	}

	p.PipelineID = uuid.New()
	p.VersionID = uuid.New()

	executableFunctions, err := p.Pipeline.Blocks.GetExecutableFunctions()
	if err != nil {
		errorHandler.handleError(GetExecutableFunctionIDsError, err)

		return
	}

	hasPrivateFunction, err := ae.hasPrivateFunction(ctx, executableFunctions)
	if err != nil {
		errorHandler.handleError(GetFunctionError, err)

		return
	}

	err = ae.DB.CreatePipeline(ctx, &p, userFromContext.Username, updated, oldVersionID, hasPrivateFunction)
	if err != nil {
		if db.IsUniqueConstraintError(err) {
			errorHandler.handleError(PipelineNameUsed, err)
		}

		errorHandler.handleError(PipelineCreateError, err)

		return
	}

	created, err := ae.DB.GetPipelineVersion(ctx, p.VersionID, true)
	if err != nil {
		errorHandler.handleError(PipelineReadError, err)

		return
	}

	err = sendResponse(w, http.StatusOK, created)
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

//nolint:dupl //its not duplicate
func (ae *Env) GetPipeline(w http.ResponseWriter, req *http.Request, pipelineID string) {
	ctx, s := trace.StartSpan(req.Context(), "get_pipeline")
	defer s.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	id, err := uuid.Parse(pipelineID)
	if err != nil {
		errorHandler.handleError(UUIDParsingError, err)

		return
	}

	p, err := ae.DB.GetPipeline(ctx, id)
	if err != nil {
		errorHandler.handleError(GetPipelineError, err)

		return
	}

	if err = sendResponse(w, http.StatusOK, p); err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

func (ae *Env) ListPipelines(w http.ResponseWriter, req *http.Request, params ListPipelinesParams) {
	ctx, s := trace.StartSpan(req.Context(), "list_pipelines")
	defer s.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	myPipelines := params.My != nil && *params.My
	publishedPipelines := params.IsPublished != nil && *params.IsPublished
	page := defaultPage
	perPage := defaultPerPage
	filter := ""

	if params.Page != nil && *params.Page > 0 {
		page = *params.Page - 1
	}

	if params.PerPage != nil && *params.PerPage > 0 {
		perPage = *params.PerPage
	}

	if params.Filter != nil {
		filter = *params.Filter
	}

	pipelines, err := ae.listPipelines(ctx, myPipelines, publishedPipelines, page, perPage, filter)
	if err != nil {
		errorHandler.sendError(err.Err)

		return
	}

	if err := sendResponse(w, http.StatusOK, pipelines); err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

func (ae *Env) DeletePipeline(w http.ResponseWriter, req *http.Request, pipelineID string) {
	ctx, s := trace.StartSpan(req.Context(), "delete_pipeline")
	defer s.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	id, err := uuid.Parse(pipelineID)
	if err != nil {
		errorHandler.handleError(UUIDParsingError, err)

		return
	}

	if err = ae.DB.DeletePipeline(ctx, id); err != nil {
		errorHandler.handleError(PipelineDeleteError, err)

		return
	}

	err = sendResponse(w, http.StatusOK, id)
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

func (ae *Env) DeleteDraftPipeline(ctx context.Context, w http.ResponseWriter, p *entity.EriusScenario) error {
	ctx, s := trace.StartSpan(ctx, "delete_draft_pipeline")
	defer s.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	canDelete, err := ae.DB.PipelineRemovable(ctx, p.PipelineID)
	if err != nil {
		errorHandler.handleError(PipelineIsNotDraft, err)

		return err
	}

	if canDelete {
		if err = ae.DB.DeletePipeline(ctx, p.PipelineID); err != nil {
			errorHandler.handleError(PipelineDeleteError, err)

			return err
		}
	}

	return nil
}

func (ae *Env) GetPipelineVersions(w http.ResponseWriter, req *http.Request, pipelineID string) {
	ctx, span := trace.StartSpan(req.Context(), "get_pipeline_versions")
	defer span.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	id, err := uuid.Parse(pipelineID)
	if err != nil {
		errorHandler.handleError(UUIDParsingError, err)

		return
	}

	vv, err := ae.DB.GetPipelineVersions(ctx, id)
	if err != nil {
		errorHandler.handleError(GetPipelineVersionsError, err)

		return
	}

	err = sendResponse(w, http.StatusOK, vv)
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

// listPipelines выбирает версии сценария с признаком Draft,
// разрешенные для данного пользователя
//
//nolint:dupl //diff logic
func (ae *Env) listPipelines(ctx context.Context,
	myPipelines,
	publishedPipelines bool,
	page, perPage int,
	filter string,
) ([]entity.EriusScenarioInfo, *PipelinerError) {
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

	drafts, err := ae.DB.GetPipelinesWithLatestVersion(ctx, authorLogin, publishedPipelines, &page, &perPage, filter)
	if err != nil {
		return []entity.EriusScenarioInfo{}, &PipelinerError{GetAllDraftsError}
	}

	return drafts, nil
}

func (ae *Env) PipelineNameExists(w http.ResponseWriter, r *http.Request, params PipelineNameExistsParams) {
	ctx, span := trace.StartSpan(r.Context(), "pipeline_name_exists")
	defer span.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	nameExists, checkNameExistsErr := ae.DB.CheckPipelineNameExists(ctx, params.Name, params.CheckNotDeleted)

	if checkNameExistsErr != nil {
		errorHandler.handleError(UnknownError, checkNameExistsErr)

		return
	}

	sendResponseErr := sendResponse(w, http.StatusOK, NameExists{
		Exists: *nameExists,
	})
	if sendResponseErr != nil {
		errorHandler.handleError(UnknownError, sendResponseErr)

		return
	}
}
