package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/gommon/log"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/user"
)

const statusRunned = "runned"
const copyPostfix = "копия"

const (
	ValidateParallelNodeReturnCycle       = "ParallelNodeReturnCycle"
	ValidateParallelNodeExitsNotConnected = "ParallelNodeExitsNotConnected"
	ValidateOutOfParallelNodesConnection  = "OutOfParallelNodesConnection"
	ValidateParallelOutOfStartInsert      = "ParallelOutOfStartInsert"
	ValidateParallelPathIntersected       = "ParallelPathIntersected"
)

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

	if len(p.Pipeline.Blocks) == 0 {
		p.Pipeline.FillEmptyPipeline()
		b, _ = json.Marshal(&p) // nolint // already unmarshalling that struct
	}
	ok, valErr := p.Pipeline.Blocks.Validate(ctx, ae.ServiceDesc)
	if p.Status == db.StatusApproved && !ok {
		var e Err

		switch valErr {
		case ValidateParallelNodeReturnCycle:
			e = ParallelNodeReturnCycle
		case ValidateParallelNodeExitsNotConnected:
			e = ParallelNodeExitsNotConnected
		case ValidateOutOfParallelNodesConnection:
			e = OutOfParallelNodesConnection
		case ValidateParallelOutOfStartInsert:
			e = ParallelOutOfStartInsert
		case ValidateParallelPathIntersected:
			e = ParallelPathIntersected
		default:
			e = PipelineValidateError
		}
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}

	executableFunctions, err := p.Pipeline.Blocks.GetExecutableFunctions()
	if err != nil {
		e := GetExecutableFunctionIDsError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	hasPrivateFunction := false
	for _, fn := range executableFunctions {
		function, getFunctionErr := ae.FunctionStore.GetFunctionVersion(ctx, fn.FunctionId, fn.VersionId)
		if getFunctionErr != nil {
			e := GetFunctionError
			log.Error(e.errorMessage(getFunctionErr))
			_ = e.sendError(w)

			return
		}

		hasPrivateFunction = function.Options.Private
		if hasPrivateFunction {
			break
		}
	}

	err = ae.DB.CreatePipeline(ctx, &p, userFromContext.Username, b, uuid.Nil, hasPrivateFunction)
	if err != nil {
		e := PipelineCreateError
		if db.IsUniqueConstraintError(err) {
			e = PipelineNameUsed
		}
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	created, err := ae.DB.GetPipelineVersion(ctx, p.VersionID, true)
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

	oldVersionID := p.VersionID

	p.Name = fmt.Sprintf("%s - %s", p.Name, copyPostfix)

	updated, err := json.Marshal(p)
	if err != nil {
		e := PipelineParseError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	p.ID = uuid.New()
	p.VersionID = uuid.New()

	executableFunctions, err := p.Pipeline.Blocks.GetExecutableFunctions()
	if err != nil {
		e := GetExecutableFunctionIDsError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	hasPrivateFunction := false
	for _, fn := range executableFunctions {
		function, getFunctionErr := ae.FunctionStore.GetFunctionVersion(ctx, fn.FunctionId, fn.VersionId)
		if getFunctionErr != nil {
			e := GetFunctionError
			log.Error(e.errorMessage(getFunctionErr))
			_ = e.sendError(w)

			return
		}

		hasPrivateFunction = function.Options.Private
		if hasPrivateFunction {
			break
		}
	}

	err = ae.DB.CreatePipeline(ctx, &p, userFromContext.Username, updated, oldVersionID, hasPrivateFunction)
	if err != nil {
		e := PipelineCreateError
		if db.IsUniqueConstraintError(err) {
			e = PipelineNameUsed
		}
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	created, err := ae.DB.GetPipelineVersion(ctx, p.VersionID, true)
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

	if err = sendResponse(w, http.StatusOK, p); err != nil {
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
	publishedPipelines := params.IsPublished != nil && *params.IsPublished
	page := 1
	perPage := 10
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

	if err = ae.DB.DeletePipeline(ctx, id); err != nil {
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
		if err = ae.DB.DeletePipeline(ctx, p.ID); err != nil {
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

// listPipelines выбирает версии сценария с признаком Draft,
// разрешенные для данного пользователя
//
//nolint:dupl //diff logic
func (ae *APIEnv) listPipelines(ctx context.Context,
	myPipelines,
	publishedPipelines bool,
	page, perPage int,
	filter string) ([]entity.EriusScenarioInfo, *PipelinerError) {
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

func (ae *APIEnv) PipelineNameExists(w http.ResponseWriter, r *http.Request, params PipelineNameExistsParams) {
	ctx, span := trace.StartSpan(r.Context(), "pipeline_name_exists")
	defer span.End()

	nameExists, checkNameExistsErr := ae.DB.CheckPipelineNameExists(ctx, params.Name, params.CheckNotDeleted)

	if checkNameExistsErr != nil {
		e := UnknownError
		log.Error(e.errorMessage(checkNameExistsErr))
		_ = e.sendError(w)

		return
	}

	sendResponseErr := sendResponse(w, http.StatusOK, NameExists{
		Exists: *nameExists,
	})
	if sendResponseErr != nil {
		e := UnknownError
		log.Error(e.errorMessage(sendResponseErr))
		_ = e.sendError(w)

		return
	}
}
