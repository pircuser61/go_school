package api

import (
	c "context"
	"encoding/json"
	"github.com/google/uuid"
	"io"
	"net/http"
	"strings"

	"github.com/pkg/errors"

	"go.opencensus.io/trace"

	"github.com/iancoleman/orderedmap"

	"github.com/golang-jwt/jwt/v4"

	"github.com/jackc/pgx/v4"

	integration_v1 "gitlab.services.mts.ru/jocasta/integrations/pkg/proto/gen/integration/v1"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/user"
)

const (
	defaultPage    = 1
	defaultPerPage = 10
)

func (ae *APIEnv) CreatePipelineVersion(w http.ResponseWriter, req *http.Request, pipelineID string) {
	ctx, s := trace.StartSpan(req.Context(), "create_draft")
	defer s.End()

	log := logger.GetLogger(ctx)

	b, err := io.ReadAll(req.Body)
	defer req.Body.Close()

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

	oldVersionID := p.VersionID
	p.VersionID = uuid.New()
	p.ID, err = uuid.Parse(pipelineID)
	if err != nil {
		e := VersionCreateError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	ui, err := user.GetUserInfoFromCtx(ctx)
	if err != nil {
		log.WithError(err).Error("user failed")
	}

	updated, err := json.Marshal(p)
	if err != nil {
		e := PipelineParseError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = ae.DB.CreateVersion(ctx, &p, ui.Username, updated, oldVersionID)
	if err != nil {
		e := PipelineWriteError
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

func (ae *APIEnv) RunVersion(w http.ResponseWriter, req *http.Request, versionID string) {
	ctx, s := trace.StartSpan(req.Context(), "run_pipeline")
	defer s.End()

	log := logger.GetLogger(ctx)

	id, err := uuid.Parse(versionID)
	if err != nil {
		e := UUIDParsingError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	p, err := ae.DB.GetPipelineVersion(ctx, id, true)
	if err != nil {
		e := GetPipelineError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	runResponse, err := ae.execVersion(ctx, &execVersionDTO{
		version:  p,
		withStop: false,
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

type runVersionByPipelineIDRequest struct {
	ApplicationBody   orderedmap.OrderedMap `json:"application_body"`
	Description       string                `json:"description"`
	PipelineId        string                `json:"pipeline_id"`
	AttachmentFields  []string              `json:"attachment_fields"`
	Keys              map[string]string     `json:"keys"`
	IsTestApplication bool                  `json:"is_test_application"`
}

//nolint:gocyclo //its ok here
func (ae *APIEnv) RunVersionsByPipelineId(w http.ResponseWriter, r *http.Request) {
	ctx, s := trace.StartSpan(r.Context(), "run_version_by_pipeline_id")
	defer s.End()

	log := logger.GetLogger(ctx)

	body, err := io.ReadAll(r.Body)
	defer r.Body.Close()

	if err != nil {
		e := RequestReadError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	req := &runVersionByPipelineIDRequest{}

	if err = json.Unmarshal(body, req); err != nil {
		e := BodyParseError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if req.PipelineId == "" {
		e := ValidationError
		log.Error(e.errorMessage(errors.New("pipelineID is empty")))
		_ = e.sendError(w)

		return
	}

	version, err := ae.DB.GetVersionByPipelineID(ctx, req.PipelineId)
	if err != nil {
		e := GetVersionsByBlueprintIdError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	var clientID string
	clientID, err = ae.getClietIDFromToken(r.Header.Get(AuthorizationHeader))
	if err != nil {
		e := GetClientIDError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	var mappedApplicationBody orderedmap.OrderedMap
	mappedApplicationBody, err = ae.processMappings(ctx, clientID, *version, req.ApplicationBody)
	if err != nil {
		e := MappingError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if err = version.FillEntryPointOutput(); err != nil {
		e := GetEntryPointOutputError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	v, execErr := ae.execVersion(ctx, &execVersionDTO{
		version:  version,
		withStop: false,
		w:        w,
		req:      r,
		runCtx: entity.TaskRunContext{
			ClientID: clientID,
			InitialApplication: entity.InitialApplication{
				Description:               req.Description,
				ApplicationBody:           mappedApplicationBody,
				Keys:                      req.Keys,
				AttachmentFields:          req.AttachmentFields,
				IsTestApplication:         req.IsTestApplication,
				ApplicationBodyFromSystem: req.ApplicationBody,
			},
		},
	})
	if execErr != nil {
		e := PipelineExecutionError
		log.Error(e.errorMessage(execErr))
		_ = e.sendError(w)
		return
	}

	if v == nil {
		e := PipelineExecutionError
		log.Error(e.errorMessage(errors.New("run_version_by_pipeline_id execution error")))
		_ = e.sendError(w)
		return
	}

	if err = sendResponse(w, http.StatusOK, version); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

func (ae *APIEnv) processMappings(ctx c.Context, clientID string,
	version entity.EriusScenario, applicationBody orderedmap.OrderedMap) (orderedmap.OrderedMap, error) {
	system, err := ae.Integrations.Cli.GetIntegrationByClientId(ctx, &integration_v1.GetIntegrationByClientIdRequest{
		ClientId: clientID,
	})
	if err != nil {
		if strings.Contains(err.Error(), "system not found") { // TODO: delete
			return applicationBody, nil
		}

		return orderedmap.OrderedMap{}, err
	}

	externalSystem, err := ae.DB.GetExternalSystemSettings(ctx, version.VersionID.String(), system.Integration.IntegrationId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) { // TODO: delete
			return applicationBody, nil
		}

		return orderedmap.OrderedMap{}, err
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
	} else {
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
			nil,
		)
		if err != nil {
			return orderedmap.OrderedMap{}, err
		}

		mappedApplicationBody, err = script.MapToOrderedMap(mappedData)
		if err != nil {
			return orderedmap.OrderedMap{}, err
		}
	}

	err = validateApplicationBody(mappedApplicationBody, startSchemaString)
	if err != nil {
		return orderedmap.OrderedMap{}, err
	}

	return mappedApplicationBody, nil
}

type runNewVersionsByPrevVersionRequest struct {
	ApplicationBody  orderedmap.OrderedMap `json:"application_body"`
	Description      string                `json:"description"`
	WorkNumber       string                `json:"work_number"`
	AttachmentFields []string              `json:"attachment_fields"`
	Keys             map[string]string     `json:"keys"`
}

func (ae *APIEnv) RunNewVersionByPrevVersion(w http.ResponseWriter, r *http.Request) {
	ctx, s := trace.StartSpan(r.Context(), "run_new_version_by_prev_version")
	defer s.End()

	log := logger.GetLogger(ctx)

	body, err := io.ReadAll(r.Body)
	defer r.Body.Close()

	if err != nil {
		e := RequestReadError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	req := &runNewVersionsByPrevVersionRequest{}

	err = json.Unmarshal(body, req)
	if err != nil {
		e := BodyParseError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if req.WorkNumber == "" {
		e := ValidationError
		log.Error(e.errorMessage(errors.New("workNumber is empty")))
		_ = e.sendError(w)

		return
	}

	version, err := ae.DB.GetVersionByWorkNumber(ctx, req.WorkNumber)
	if err != nil {
		e := GetVersionsByWorkNumberError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	started, execErr := ae.execVersion(ctx, &execVersionDTO{
		version:     version,
		withStop:    false,
		w:           w,
		req:         r,
		makeNewWork: true,
		workNumber:  req.WorkNumber,
		runCtx: entity.TaskRunContext{
			InitialApplication: entity.InitialApplication{
				Description:      req.Description,
				ApplicationBody:  req.ApplicationBody,
				AttachmentFields: req.AttachmentFields,
				Keys:             req.Keys,
			},
		},
	})
	if execErr != nil {
		e := UnknownError
		log.Error(e.errorMessage(execErr))
		_ = e.sendError(w)
		return
	}

	if started == nil {
		e := UnknownError
		log.Error(e.errorMessage(errors.New("no one version was started")))
		_ = e.sendError(w)
		return
	}

	err = sendResponse(w, http.StatusOK, started)
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}
}

func (ae *APIEnv) DeleteVersion(w http.ResponseWriter, req *http.Request, versionID string) {
	ctx, s := trace.StartSpan(req.Context(), "delete_version")
	defer s.End()

	log := logger.GetLogger(ctx)

	vID, err := uuid.Parse(versionID)
	if err != nil {
		e := UUIDParsingError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	p, err := ae.DB.GetPipelineVersion(ctx, vID, true)
	if err != nil {
		e := PipelineDeleteError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if p.Status == db.StatusDraft {
		err = ae.DeleteDraftPipeline(ctx, w, p)
		if err != nil {
			e := PipelineDeleteError
			log.Error(e.errorMessage(err))
			_ = e.sendError(w)

			return
		}
	}

	err = ae.DB.DeleteVersion(ctx, vID)
	if err != nil {
		e := PipelineDeleteError
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

//nolint:dupl //its not duplicate
func (ae *APIEnv) GetPipelineVersion(w http.ResponseWriter, req *http.Request, versionID string) {
	ctx, s := trace.StartSpan(req.Context(), "get_pipeline_version")
	defer s.End()

	log := logger.GetLogger(ctx)

	versionUUID, err := uuid.Parse(versionID)
	if err != nil {
		e := UUIDParsingError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	p, err := ae.DB.GetPipelineVersion(ctx, versionUUID, true)
	if err != nil {
		e := GetVersionError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = p.FillEntryPointOutput()
	if err != nil {
		e := GetEntryPointOutputError
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

func (ae *APIEnv) EditVersion(w http.ResponseWriter, req *http.Request) {
	ctx, s := trace.StartSpan(req.Context(), "edit_draft")
	defer s.End()

	log := logger.GetLogger(ctx)

	b, err := io.ReadAll(req.Body)
	defer req.Body.Close()

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

	canEdit, err := ae.DB.VersionEditable(ctx, p.VersionID)
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if !canEdit {
		err = ae.DB.RollbackVersion(ctx, p.ID, p.VersionID)
		if err != nil {
			e := ApproveError
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

		return
	}

	err = ae.DB.UpdateDraft(ctx, &p, b)
	if err != nil {
		e := PipelineWriteError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	ui, err := user.GetUserInfoFromCtx(ctx)
	if err != nil {
		log.Error(err.Error())
	}

	if p.Status == db.StatusApproved {
		err = ae.DB.SwitchApproved(ctx, p.ID, p.VersionID, ui.Username)
		if err != nil {
			e := ApproveError
			log.Error(e.errorMessage(err))
			_ = e.sendError(w)

			return
		}
	}

	if p.Status == db.StatusRejected {
		err = ae.DB.SwitchRejected(ctx, p.VersionID, p.CommentRejected, ui.Username)
		if err != nil {
			e := ApproveError
			log.Error(e.errorMessage(err))
			_ = e.sendError(w)

			return
		}
	}

	edited, err := ae.DB.GetPipelineVersion(ctx, p.VersionID, true)
	if err != nil {
		e := PipelineReadError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = sendResponse(w, http.StatusOK, edited)
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

type execVersionDTO struct {
	version  *entity.EriusScenario
	withStop bool

	w   http.ResponseWriter
	req *http.Request

	makeNewWork bool
	workNumber  string
	runCtx      entity.TaskRunContext
}

// nolint //need big cyclo,need equal string for all usages
func (ae *APIEnv) execVersion(ctx c.Context, dto *execVersionDTO) (*entity.RunResponse, error) {
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

	arg := &execVersionInternalDTO{
		reqID:         reqID,
		p:             dto.version,
		vars:          pipelineVars,
		syncExecution: dto.withStop,
		userName:      usr.Username,
		makeNewWork:   dto.makeNewWork,
		workNumber:    dto.workNumber,
		runCtx:        dto.runCtx,
	}

	executablePipeline, e, err := ae.execVersionInternal(ctxLocal, arg)
	if err != nil {
		log.Error(e.errorMessage(err))
		return nil, errors.Wrap(err, e.error())
	}

	return &entity.RunResponse{
		PipelineID: executablePipeline.PipelineID,
		WorkNumber: executablePipeline.WorkNumber,
		Status:     statusRunned,
	}, nil
}

type execVersionInternalDTO struct {
	reqID         string
	p             *entity.EriusScenario
	vars          map[string]interface{}
	syncExecution bool
	userName      string
	makeNewWork   bool
	workNumber    string
	runCtx        entity.TaskRunContext
}

func (ae *APIEnv) execVersionInternal(ctx c.Context, dto *execVersionInternalDTO) (*pipeline.ExecutablePipeline, Err, error) {
	_, span := trace.StartSpan(ctx, "exec_version_internal")
	defer span.End()

	log := logger.GetLogger(ctx).WithField("mainFuncName", "execVersionInternal")

	spCtx := span.SpanContext()
	// nolint:staticcheck //its ok here
	routineCtx := c.WithValue(c.Background(), XRequestIDHeader, ctx.Value(XRequestIDHeader))
	routineCtx = logger.WithLogger(routineCtx, log)
	processCtx, fakeSpan := trace.StartSpanWithRemoteParent(routineCtx, "start_processing", spCtx)
	fakeSpan.End()

	txStorage, transactionErr := ae.DB.StartTransaction(processCtx)
	if transactionErr != nil {
		e := PipelineRunError
		return nil, e, transactionErr
	}

	ep := pipeline.ExecutablePipeline{}
	ep.PipelineID = dto.p.ID
	ep.VersionID = dto.p.VersionID
	ep.Storage = txStorage
	ep.EntryPoint = dto.p.Pipeline.Entrypoint
	ep.FaaS = ae.FaaS
	ep.PipelineModel = dto.p
	ep.HTTPClient = ae.HTTPClient
	ep.Remedy = ae.Remedy
	ep.ActiveBlocks = map[string]struct{}{}
	ep.SkippedBlocks = map[string]struct{}{}
	ep.EntryPoint = pipeline.BlockGoFirstStart
	ep.Kafka = ae.Kafka
	ep.Sender = ae.Mail
	ep.People = ae.People
	ep.Name = dto.p.Name
	ep.ServiceDesc = ae.ServiceDesc
	ep.FunctionStore = ae.FunctionStore
	ep.HumanTasks = ae.HumanTasks
	ep.Integrations = ae.Integrations

	if dto.makeNewWork {
		ep.WorkNumber = dto.workNumber
	}

	variableStorage := store.NewStore()
	pipelineVars := dto.vars

	parameters, err := json.Marshal(pipelineVars)
	if err != nil {
		if txErr := txStorage.RollbackTransaction(processCtx); txErr != nil {
			log.WithField("funcName", "marshal vars").
				WithError(errors.New("couldn't rollback tx")).
				Error(txErr)
		}
		e := PipelineRunError
		return nil, e, err
	}

	// use ctx as we need userinfo
	if err = ep.CreateTask(ctx, &pipeline.CreateTaskDTO{
		Author:     dto.userName,
		IsDebug:    false,
		Params:     parameters,
		WorkNumber: dto.workNumber,
		RunCtx:     dto.runCtx,
	}); err != nil {
		if txErr := txStorage.RollbackTransaction(processCtx); txErr != nil {
			log.WithField("funcName", "CreateTask").
				WithError(errors.New("couldn't rollback tx")).
				Error(txErr)
		}
		e := PipelineRunError
		return nil, e, err
	}

	runCtx := &pipeline.BlockRunContext{
		TaskID:     ep.TaskID,
		WorkNumber: ep.WorkNumber,
		WorkTitle:  ep.Name,
		Initiator:  dto.userName,
		Storage:    txStorage,
		VarStore:   variableStorage,

		Sender:        ep.Sender,
		Kafka:         ep.Kafka,
		People:        ep.People,
		ServiceDesc:   ep.ServiceDesc,
		FunctionStore: ep.FunctionStore,
		HumanTasks:    ep.HumanTasks,
		Integrations:  ep.Integrations,
		FaaS:          ep.FaaS,

		UpdateData: nil,
	}

	blockData := dto.p.Pipeline.Blocks[ep.EntryPoint]

	err = pipeline.ProcessBlock(processCtx, ep.EntryPoint, &blockData, runCtx, false)
	if err != nil {
		if txErr := txStorage.RollbackTransaction(processCtx); txErr != nil {
			log.WithField("funcName", "RollbackTransaction").
				WithError(errors.New("couldn't rollback tx")).
				Error(txErr)
		}
		variableStorage.AddError(err)
		e := PipelineRunError
		return nil, e, err
	}
	if err = txStorage.CommitTransaction(processCtx); err != nil {
		e := PipelineRunError
		return nil, e, err
	}
	return &ep, 0, nil
}

func (ae *APIEnv) SearchPipelines(w http.ResponseWriter, req *http.Request, params SearchPipelinesParams) {
	ctx, s := trace.StartSpan(req.Context(), "search_pipelines")
	defer s.End()

	log := logger.GetLogger(ctx)

	if params.PipelineId == nil && params.PipelineName == nil {
		e := ValidationPipelineSearchError
		log.Error(e.errorMessage(errors.New("name and id are empty")))
		_ = e.sendError(w)

		return
	}

	items, err := ae.DB.GetPipelinesByNameOrId(ctx, toDbSearchPipelinesParams(&params))
	if err != nil {
		e := GetPipelinesSearchError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	res := &ResponsePipelineSearch{}

	for i := range items {
		res.Items = append(res.Items, SearchPipelineItem{
			Name:       &items[i].PipelineName,
			PipelineId: &items[i].PipelineId,
		})
	}

	if len(items) > 0 {
		res.Total = items[0].Total
	}

	err = sendResponse(w, http.StatusOK, res)
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

func toDbSearchPipelinesParams(in *SearchPipelinesParams) (out *db.SearchPipelineRequest) {
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
		PipelineId:   in.PipelineId,
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

func (ae *APIEnv) getClietIDFromToken(token string) (string, error) {
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
