package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v4"

	"github.com/google/uuid"

	"github.com/iancoleman/orderedmap"

	"github.com/jackc/pgx/v4"

	"github.com/pkg/errors"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	integration_v1 "gitlab.services.mts.ru/jocasta/integrations/pkg/proto/gen/integration/v1"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/sso"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/user"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

const (
	defaultPage    = 1
	defaultPerPage = 10

	startEntrypoint = "start_0"

	keyWorkNumber      = "workNumber"
	keyInitiator       = "initiator"
	keyApplicationBody = "application_body"
)

func (ae *Env) CreatePipelineVersion(w http.ResponseWriter, req *http.Request, pipelineID string) {
	ctx, s := trace.StartSpan(req.Context(), "create_pipeline_version")
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

	oldVersionID := p.VersionID
	p.VersionID = uuid.New()

	apiErr, err := ae.fillPipeline(&p, pipelineID)
	if err != nil {
		errorHandler.handleError(apiErr, err)

		return
	}

	ui, err := user.GetUserInfoFromCtx(ctx)
	if err != nil {
		log.WithError(err).Error("user failed")
	}

	updated, err := json.Marshal(p)
	if err != nil {
		errorHandler.handleError(PipelineParseError, err)

		return
	}

	updated = []byte(wrapApplicationBody(string(updated)))

	txStorage, transactionErr := ae.DB.StartTransaction(ctx)
	if transactionErr != nil {
		log.WithError(transactionErr).Error("couldn't create pipeline version")

		return
	}

	defer func() {
		if r := recover(); r != nil {
			log = log.WithField("funcName", "CreatePipelineVersion").
				WithField("panic handle", true)
			log.Error(r)

			if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
				log.WithError(errors.New("couldn't rollback tx")).
					Error(txErr)
			}
		}
	}()

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

	err = ae.DB.CreateVersion(ctx, &p, ui.Username, updated, oldVersionID, hasPrivateFunction)
	if err != nil {
		if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
			log.WithField("funcName", "CreateVersion").
				WithError(errors.New("couldn't rollback tx")).
				Error(txErr)
		}

		errorHandler.handleError(PipelineWriteError, err)

		return
	}

	if commitErr := txStorage.CommitTransaction(ctx); commitErr != nil {
		log.WithError(commitErr).Error("couldn't create pipeline version")

		if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
			log.Error(txErr)
		}

		errorHandler.handleError(PipelineReadError, err)

		return
	}

	created, err := ae.DB.GetPipelineVersion(ctx, p.VersionID, true)
	if err != nil {
		errorHandler.handleError(PipelineReadError, err)

		return
	}

	if err = sendResponse(w, http.StatusOK, created); err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

func (ae *Env) hasPrivateFunction(ctx context.Context, executableFunctions []script.FunctionParam) (bool, error) {
	//nolint:gocritic //коллекция без поинтеров
	for _, fn := range executableFunctions {
		function, getFunctionErr := ae.FunctionStore.GetFunctionVersion(ctx, fn.FunctionID, fn.VersionID)
		if getFunctionErr != nil {
			return false, getFunctionErr
		}

		if function.Options.Private {
			return true, nil
		}
	}

	return false, nil
}

func (ae *Env) getExternalSystem(
	ctx context.Context,
	storage db.Database,
	clientID, pipelineID, versionID string,
) (*entity.ExternalSystem, error) {
	system, err := ae.Integrations.RPCIntCli.GetIntegrationByClientId(ctx, &integration_v1.GetIntegrationByClientIdRequest{
		ClientId:   clientID,
		PipelineId: pipelineID,
		VersionId:  versionID,
	})
	if err != nil {
		if strings.Contains(err.Error(), "system not found") { // TODO: delete
			return nil, nil
		}

		return nil, err
	}

	externalSystem, err := storage.GetExternalSystemSettings(ctx, versionID, system.Integration.IntegrationId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) { // TODO: delete
			return nil, nil
		}

		return nil, err
	}

	return &externalSystem, nil
}

func (ae *Env) processMappings(externalSystem *entity.ExternalSystem,
	version *entity.EriusScenario, applicationBody orderedmap.OrderedMap,
) (orderedmap.OrderedMap, error) {
	if externalSystem == nil {
		return applicationBody, nil
	}

	if externalSystem.InputSchema == nil && version.Settings.StartSchema == nil { // TODO: delete
		return applicationBody, nil
	}

	inputSchema, err := json.Marshal(externalSystem.InputSchema)
	if err != nil {
		return orderedmap.OrderedMap{}, err
	}

	// JSON schema of the data that the external system wants to send
	inputSchemaString := string(inputSchema)

	startSchema, err := json.Marshal(version.Settings.StartSchema)
	if err != nil {
		return orderedmap.OrderedMap{}, err
	}

	// JSON schema of the data the process wants to receive
	startSchemaString := string(startSchema)

	err = validateApplicationBody(applicationBody, inputSchemaString)
	if err != nil {
		return orderedmap.OrderedMap{}, err
	}

	var mappedApplicationBody orderedmap.OrderedMap

	if externalSystem.InputMapping == nil || inputSchemaString == startSchemaString {
		// mapping is not needed
		return applicationBody, nil
	}
	// need mapping
	var mappedData map[string]interface{}

	appBody, errMap := script.OrderedMapToMap(applicationBody)
	if errMap != nil {
		return orderedmap.OrderedMap{}, err
	}

	mappedData, err = script.MapData(
		externalSystem.InputMapping.Properties,
		appBody,
		externalSystem.InputMapping.Required,
	)
	if err != nil {
		return orderedmap.OrderedMap{}, err
	}

	mappedApplicationBody, err = script.MapToOrderedMap(mappedData)
	if err != nil {
		return orderedmap.OrderedMap{}, err
	}

	err = validateApplicationBody(mappedApplicationBody, startSchemaString)
	if err != nil {
		return orderedmap.OrderedMap{}, err
	}

	return mappedApplicationBody, nil
}

func (ae *Env) DeleteVersion(w http.ResponseWriter, req *http.Request, versionID string) {
	ctx, s := trace.StartSpan(req.Context(), "delete_version")
	defer s.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	vID, err := uuid.Parse(versionID)
	if err != nil {
		errorHandler.handleError(UUIDParsingError, err)

		return
	}

	p, err := ae.DB.GetPipelineVersion(ctx, vID, true)
	if err != nil {
		errorHandler.handleError(PipelineDeleteError, err)

		return
	}

	if p.Status == db.StatusDraft {
		err = ae.DeleteDraftPipeline(ctx, w, p)
		if err != nil {
			errorHandler.handleError(PipelineDeleteError, err)

			return
		}
	}

	err = ae.DB.DeleteVersion(ctx, vID)
	if err != nil {
		errorHandler.handleError(PipelineDeleteError, err)

		return
	}

	err = sendResponse(w, http.StatusOK, nil)
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

//nolint:dupl //its not duplicate
func (ae *Env) GetPipelineVersion(w http.ResponseWriter, req *http.Request, versionID string) {
	ctx, s := trace.StartSpan(req.Context(), "get_pipeline_version")
	defer s.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	versionUUID, err := uuid.Parse(versionID)
	if err != nil {
		errorHandler.handleError(UUIDParsingError, err)

		return
	}

	p, err := ae.DB.GetPipelineVersion(ctx, versionUUID, true)
	if err != nil {
		errorHandler.handleError(GetVersionError, err)

		return
	}

	err = p.FillEntryPointOutput()
	if err != nil {
		errorHandler.handleError(GetEntryPointOutputError, err)

		return
	}

	if err = sendResponse(w, http.StatusOK, p); err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

// TODO: Убрать нолинт на нижней строчке 15.04.2024

//nolint:gocyclo // Временная проверка, скоро уберем
func (ae *Env) EditVersion(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "edit_draft")
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

	apiErr, err := ae.fillPipeline(&p, "")
	if err != nil {
		errorHandler.handleError(apiErr, err)

		return
	}

	updated, err := json.Marshal(p)
	if err != nil {
		errorHandler.handleError(PipelineParseError, err)

		return
	}

	updated = []byte(wrapApplicationBody(string(updated)))

	ok, valErr := p.Pipeline.Blocks.Validate(ctx, ae.ServiceDesc)
	if p.Status == db.StatusApproved && !ok {
		e := validateBlockTypeErrText(valErr)
		errorHandler.handleError(e, errors.New(valErr))

		return
	}

	groups, err := statusGroups(&p)
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}

	canEdit, err := ae.DB.VersionEditable(ctx, p.VersionID)
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}

	if !canEdit {
		err = ae.DB.RollbackVersion(ctx, p.PipelineID, p.VersionID)
		if err != nil {
			errorHandler.handleError(ApproveError, err)

			return
		}

		err = sendResponse(w, http.StatusOK, nil)
		if err != nil {
			errorHandler.handleError(UnknownError, err)
		}

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

	err = ae.DB.UpdateDraft(ctx, &p, updated, groups, hasPrivateFunction)
	if err != nil {
		errorHandler.handleError(PipelineWriteError, err)

		return
	}

	ui, err := user.GetUserInfoFromCtx(ctx)
	if err != nil {
		log.Error(err.Error())

		return
	}

	err = ae.switchScenarioApproved(ctx, &p, ui)
	if err != nil {
		errorHandler.handleError(ApproveError, err)

		return
	}

	err = ae.handleScenario(ctx, &p, ui)
	if err != nil {
		errorHandler.handleError(ApproveError, err)

		return
	}

	edited, err := ae.DB.GetPipelineVersion(ctx, p.VersionID, true)
	if err != nil {
		errorHandler.handleError(PipelineReadError, err)

		return
	}

	err = sendResponse(w, http.StatusOK, edited)
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

func validateBlockTypeErrText(valErrText string) Err {
	switch valErrText {
	case ValidateParallelNodeReturnCycle:
		return ParallelNodeReturnCycle
	case ValidateParallelNodeExitsNotConnected:
		return ParallelNodeExitsNotConnected
	case ValidateOutOfParallelNodesConnection:
		return OutOfParallelNodesConnection
	case ValidateParallelOutOfStartInsert:
		return ParallelOutOfStartInsert
	case ValidateParallelPathIntersected:
		return ParallelPathIntersected
	default:
		return PipelineValidateError
	}
}

func (ae *Env) handleScenario(ctx context.Context, p *entity.EriusScenario, ui *sso.UserInfo) (err error) {
	switch p.Status {
	case db.StatusApproved:
		err = ae.DB.SwitchApproved(ctx, p.PipelineID, p.VersionID, ui.Username)
		if err != nil {
			return err
		}
	case db.StatusRejected:
		err = ae.DB.SwitchRejected(ctx, p.VersionID, p.CommentRejected, ui.Username)
		if err != nil {
			return err
		}
	}

	return nil
}

func (ae *Env) handlePipelineBlockLength(p *entity.EriusScenario) {
	if len(p.Pipeline.Blocks) == 0 {
		p.Pipeline.FillEmptyPipeline()
	} else {
		keyOutputs := map[string]string{
			pipeline.BlockGoApproverID:  "approver",
			pipeline.BlockGoSignID:      "signer",
			pipeline.BlockGoExecutionID: "login",
		}

		p.Pipeline.ChangeOutput(keyOutputs)
	}
}

func statusGroups(p *entity.EriusScenario) (groups []*entity.NodeGroup, err error) {
	groups = make([]*entity.NodeGroup, 0)

	if p.Status == db.StatusApproved {
		groups, err = p.Pipeline.Blocks.GetGroups()
		if err != nil {
			return nil, err
		}
	}

	return groups, nil
}

func (ae *Env) switchScenarioApproved(ctx context.Context, p *entity.EriusScenario, ui *sso.UserInfo) error {
	if p.Status == db.StatusApproved {
		err := ae.DB.SwitchApproved(ctx, p.PipelineID, p.VersionID, ui.Username)
		if err != nil {
			return err
		}
	}

	return nil
}

type execVersionDTO struct {
	version  *entity.EriusScenario
	withStop bool

	storage db.Database

	w   http.ResponseWriter
	req *http.Request

	makeNewWork      bool
	allowRunAsOthers bool
	workNumber       string
	runCtx           entity.TaskRunContext
}

func (ae *Env) execVersion(ctx context.Context, dto *execVersionDTO) (*entity.RunResponse, error) {
	ctxLocal, s := trace.StartSpan(ctx, "exec_version")
	defer s.End()

	log := logger.GetLogger(ctxLocal)
	reqID := dto.req.Header.Get(XRequestIDHeader)

	defer dto.req.Body.Close()

	var pipelineVars map[string]interface{}

	log.Info("--- running pipeline:", dto.version.Name)

	usr, err := user.GetUserInfoFromCtx(ctxLocal)
	if err != nil {
		e := NoUserInContextError
		log.Error(e.errorMessage(err))

		return nil, errors.Wrap(err, e.error())
	}

	// if X-As-Other was used, then we will store the name of the real user here
	realAuthor := dto.realAuthor(usr)

	if dto.allowRunAsOthers {
		usr, err = user.GetEffectiveUserInfoFromCtx(ctx)
		if err != nil {
			e := NoUserInContextError

			log.Error(e.errorMessage(err))

			return nil, errors.Wrap(err, e.error())
		}
	}

	arg := &execVersionInternalDTO{
		storage:        dto.storage,
		reqID:          reqID,
		p:              dto.version,
		vars:           pipelineVars,
		syncExecution:  dto.withStop,
		authorName:     usr.Username,
		realAuthorName: realAuthor,
		makeNewWork:    dto.makeNewWork,
		workNumber:     dto.workNumber,
		runCtx:         dto.runCtx,
	}

	executablePipeline, e, err := ae.execVersionInternal(ctxLocal, arg)
	if err != nil {
		log.Error(e.errorMessage(err))

		return nil, errors.Wrap(err, e.error())
	}

	if executablePipeline == nil {
		log.Error("got no pipeline")

		return nil, errors.New("No pipeline started")
	}

	return &entity.RunResponse{
		PipelineID: executablePipeline.PipelineID,
		WorkNumber: executablePipeline.WorkNumber,
		Status:     statusRunned,
	}, nil
}

func (dto *execVersionDTO) realAuthor(usr *sso.UserInfo) string {
	if dto.allowRunAsOthers {
		return usr.Username
	}

	return ""
}

type execVersionInternalDTO struct {
	storage        db.Database
	reqID          string
	p              *entity.EriusScenario
	vars           map[string]interface{}
	syncExecution  bool
	authorName     string
	realAuthorName string
	makeNewWork    bool
	workNumber     string
	runCtx         entity.TaskRunContext
}

func (ae *Env) execVersionInternal(ctx context.Context, dto *execVersionInternalDTO) (*pipeline.ExecutablePipeline, Err, error) {
	ctx, span := trace.StartSpan(ctx, "exec_version_internal")
	defer span.End()

	log := logger.GetLogger(ctx).WithField("mainFuncName", "execVersionInternal")

	txStorage, transactionErr := dto.storage.StartTransaction(ctx)
	if transactionErr != nil {
		return nil, PipelineRunError, transactionErr
	}

	defer func() {
		if r := recover(); r != nil {
			log = log.WithField("funcName", "execVersionInternal").
				WithField("panic handle", true)
			log.Error(r)

			if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
				log.WithError(errors.New("couldn't rollback tx")).
					Error(txErr)
			}
		}
	}()

	ep := ae.makeExecutablePipeline(dto, txStorage)

	variableStorage := store.NewStore()
	pipelineVars := dto.vars

	parameters, err := json.Marshal(pipelineVars)
	if err != nil {
		if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
			log.WithField("funcName", "marshal vars").
				WithError(errors.New("couldn't rollback tx")).
				Error(txErr)
		}

		return nil, PipelineRunError, err
	}

	// use ctx as we need userinfo
	err = ep.CreateTask(
		ctx,
		&pipeline.CreateTaskDTO{
			Author:     dto.authorName,
			RealAuthor: dto.realAuthorName,
			IsDebug:    false,
			Params:     parameters,
			WorkNumber: dto.workNumber,
			RunCtx:     dto.runCtx,
		},
	)
	if err != nil {
		if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
			log.WithField("funcName", "CreateTask").
				WithError(errors.New("couldn't rollback tx")).
				Error(txErr)
		}

		return nil, PipelineRunError, err
	}

	runCtx := &pipeline.BlockRunContext{
		TaskID:     ep.TaskID,
		WorkNumber: ep.WorkNumber,
		ClientID:   dto.runCtx.ClientID,
		PipelineID: ep.PipelineID,
		VersionID:  ep.VersionID,
		WorkTitle:  ep.Name,
		Initiator:  dto.authorName,
		VarStore:   variableStorage,

		Services: pipeline.RunContextServices{
			HTTPClient:    ep.HTTPClient,
			Sender:        ep.Sender,
			Kafka:         ep.Kafka,
			People:        ep.People,
			ServiceDesc:   ep.ServiceDesc,
			FunctionStore: ep.FunctionStore,
			HumanTasks:    ep.HumanTasks,
			Integrations:  ep.Integrations,
			FileRegistry:  ep.FileRegistry,
			FaaS:          ep.FaaS,
			HrGate:        ae.HrGate,
			Scheduler:     ae.Scheduler,
			SLAService:    ae.SLAService,
			Storage:       txStorage,
		},
		BlockRunResults: &pipeline.BlockRunResults{},

		UpdateData: nil,
		IsTest:     dto.runCtx.InitialApplication.IsTestApplication,
		NotifName: utils.MakeTaskTitle(
			ep.Name,
			dto.runCtx.InitialApplication.CustomTitle,
			dto.runCtx.InitialApplication.IsTestApplication),
	}
	blockData := dto.p.Pipeline.Blocks[ep.EntryPoint]

	runCtx.SetTaskEvents(ctx)

	workFinished, err := pipeline.ProcessBlockWithEndMapping(ctx, ep.EntryPoint, &blockData, runCtx, false)
	if err != nil {
		if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
			log.WithField("funcName", "RollbackTransaction").
				WithError(errors.New("couldn't rollback tx")).
				Error(txErr)
		}

		variableStorage.AddError(err)

		return nil, PipelineRunError, err
	}

	err = txStorage.CommitTransaction(ctx)
	if err != nil {
		return nil, PipelineRunError, err
	}

	if workFinished {
		err = ae.Scheduler.DeleteAllTasksByWorkID(ctx, ep.TaskID)
		if err != nil {
			log.WithError(err).Error("failed delete all tasks by work id in scheduler")
		}
	}

	runCtx.NotifyEvents(ctx)

	return ep, 0, nil
}

func (ae *Env) makeExecutablePipeline(dto *execVersionInternalDTO, txStorage db.Database) *pipeline.ExecutablePipeline {
	var workNumber string
	if dto.makeNewWork {
		workNumber = dto.workNumber
	}

	return &pipeline.ExecutablePipeline{
		WorkNumber:    workNumber,
		PipelineID:    dto.p.PipelineID,
		VersionID:     dto.p.VersionID,
		Storage:       txStorage,
		FaaS:          ae.FaaS,
		PipelineModel: dto.p,
		HTTPClient:    ae.HTTPClient,
		Remedy:        ae.Remedy,
		ActiveBlocks:  make(map[string]struct{}, 0),
		SkippedBlocks: make(map[string]struct{}, 0),
		EntryPoint:    pipeline.BlockGoFirstStart,
		Kafka:         ae.Kafka,
		Sender:        ae.Mail,
		People:        ae.People,
		Name:          dto.p.Name,
		ServiceDesc:   ae.ServiceDesc,
		FunctionStore: ae.FunctionStore,
		HumanTasks:    ae.HumanTasks,
		Integrations:  ae.Integrations,
		FileRegistry:  ae.FileRegistry,
		Scheduler:     ae.Scheduler,
	}
}

func (ae *Env) SearchPipelines(w http.ResponseWriter, req *http.Request, params SearchPipelinesParams) {
	ctx, s := trace.StartSpan(req.Context(), "search_pipelines")
	defer s.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	if params.PipelineId == nil && params.PipelineName == nil {
		errorHandler.handleError(ValidationPipelineSearchError, errors.New("name and id are empty"))

		return
	}

	items, err := ae.DB.GetPipelinesByNameOrID(ctx, toDBSearchPipelinesParams(&params))
	if err != nil {
		errorHandler.handleError(GetPipelinesSearchError, err)

		return
	}

	responseItems := make([]SearchPipelineItem, 0, len(items))
	for i := range items {
		responseItems = append(responseItems, SearchPipelineItem{
			Name:       &items[i].PipelineName,
			PipelineId: &items[i].PipelineID,
		})
	}

	res := &ResponsePipelineSearch{
		Items: responseItems,
	}

	if len(items) > 0 {
		res.Total = items[0].Total
	}

	err = sendResponse(w, http.StatusOK, res)
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

func toDBSearchPipelinesParams(in *SearchPipelinesParams) (out *db.SearchPipelineRequest) {
	var (
		page    = defaultPage
		perPage = defaultPerPage
	)

	if in.Page == nil {
		in.Page = &page
	}

	if in.PerPage == nil {
		in.PerPage = &perPage
	}

	return &db.SearchPipelineRequest{
		PipelineName: in.PipelineName,
		PipelineID:   in.PipelineId,
		Limit:        *in.PerPage,
		Offset:       (*in.Page * *in.PerPage) - *in.PerPage,
	}
}

type azpClaims struct {
	AZP string `json:"azp"`
}

func (mc azpClaims) Valid() error {
	if mc.AZP != "" {
		return nil
	}

	return &PipelinerError{GetClientIDError}
}

func (ae *Env) getClientIDFromToken(token string) (string, error) {
	claims := &azpClaims{}
	parsed, _ := jwt.ParseWithClaims(strings.TrimPrefix(token, "Bearer "), claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(""), nil
	})

	if parsed == nil || parsed.Claims == nil || parsed.Claims.Valid() != nil {
		return "", &PipelinerError{TokenParseError}
	}

	return claims.AZP, nil
}

func validateApplicationBody(applicationBody orderedmap.OrderedMap, jsonSchema string) error {
	apBody, err := applicationBody.MarshalJSON()
	if err != nil {
		return err
	}

	err = script.ValidateJSONByJSONSchema(string(apBody), jsonSchema)
	if err != nil {
		return err
	}

	return nil
}

func (ae *Env) fillPipeline(p *entity.EriusScenario, pipelineID string) (Err, error) {
	if pipelineID != "" {
		pID, err := uuid.Parse(pipelineID)
		if err != nil {
			return UUIDParsingError, err
		}

		p.PipelineID = pID
	}

	ae.handlePipelineBlockLength(p)

	if p.Pipeline.Entrypoint == "" {
		p.Pipeline.Entrypoint = startEntrypoint
	}

	if _, ok := p.Pipeline.Blocks[p.Pipeline.Entrypoint]; !ok {
		return 0, nil
	}

	startOutput := p.Pipeline.Blocks[p.Pipeline.Entrypoint].Output.Properties

	if startOutput[keyApplicationBody].Type == "" {
		startOutput[keyApplicationBody] = script.JSONSchemaPropertiesValue{
			Type:       "object",
			Global:     "start_0." + keyApplicationBody,
			Properties: make(map[string]script.JSONSchemaPropertiesValue),
		}
	}

	// TODO: Убрать этот цикл к 15.04.2024
	for k := range startOutput {
		switch k {
		case keyWorkNumber, keyInitiator, keyApplicationBody:
			break
		default:
			v := startOutput[k]
			v.Global = ""

			startOutput[keyApplicationBody].Properties[k] = v
			delete(startOutput, k)
		}
	}

	return 0, nil
}

// TODO: Убрать эту функцию к 15.04.2024
func wrapApplicationBody(objStr string) string {
	strToMarshal := strings.ReplaceAll(objStr, "start_0.", "start_0.application_body.")
	strToMarshal = strings.ReplaceAll(strToMarshal, "start_0.application_body.initiator", "start_0.initiator")
	strToMarshal = strings.ReplaceAll(strToMarshal, "start_0.application_body.workNumber", "start_0.workNumber")

	return strings.ReplaceAll(strToMarshal, "start_0.application_body.application_body", "start_0.application_body")
}
