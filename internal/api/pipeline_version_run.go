package api

import (
	c "context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/iancoleman/orderedmap"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/forms/pkg/jsonschema"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/errorutils"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/metrics"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/user"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

const (
	runByPipelineIDPath = "/run/versions/pipeline_id"
	runByKafka          = "Kafka"

	titleKey      = "title"
	emailKey      = "email"
	propertiesKey = "properties"
)

type runNewVersionsByPrevVersionRequest struct {
	ApplicationBody   orderedmap.OrderedMap `json:"application_body"`
	Description       string                `json:"description"`
	WorkNumber        string                `json:"work_number"`
	AttachmentFields  []string              `json:"attachment_fields"`
	Keys              map[string]string     `json:"keys"`
	CustomTitle       string                `json:"custom_title"`
	IsTestApplication bool                  `json:"is_test_application"`
}

type requestStartParams struct {
	version          *entity.EriusScenario
	keys             map[string]string
	attachmentFields []string
	hiddenFields     []string
}

func (ae *Env) RunNewVersionByPrevVersion(w http.ResponseWriter, r *http.Request) {
	ctx, s := trace.StartSpan(r.Context(), "run_new_version_by_prev_version")
	defer s.End()

	log := logger.GetLogger(ctx).
		WithField("mainFuncName", "RunNewVersionByPrevVersion").
		WithField("method", "post").
		WithField("transport", "rest").
		WithField("logVersion", "v1").
		WithField("TraceID", s.SpanContext().TraceID.String())

	errorHandler := newHTTPErrorHandler(log, w)

	body, err := io.ReadAll(r.Body)
	defer r.Body.Close()

	if err != nil {
		log.Error(err)
		errorHandler.handleError(RequestReadError, err)

		return
	}

	log = log.WithField("body", string(body))

	req := &runNewVersionsByPrevVersionRequest{}

	if err = json.Unmarshal(body, req); err != nil {
		log.Error(err)
		errorHandler.handleError(BodyParseError, err)

		return
	}

	log = log.WithField("workNumber", req.WorkNumber)

	errorHandler.log = log
	ctx = logger.WithLogger(ctx, errorHandler.log)

	if req.WorkNumber == "" {
		log.Error(err)
		errorHandler.handleError(ValidationError, errors.New("workNumber is empty"))

		return
	}

	runResp, err := ae.runVersionByPrevVersion(ctx, req, r.Header.Get(XRequestIDHeader))
	if err != nil {
		httpErr := getErr(err)

		errorHandler.handleError(httpErr, err)

		return
	}

	err = sendResponse(w, http.StatusOK, runResp)
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

func (ae *Env) runVersionByPrevVersion(
	ctx c.Context,
	req *runNewVersionsByPrevVersionRequest,
	requestID string,
) (*entity.RunResponse, error) {
	log := logger.GetLogger(ctx)

	usr, err := user.GetUserInfoFromCtx(ctx)
	if err != nil {
		return nil, errors.Join(NoUserInContextError, err)
	}

	workID, err := ae.DB.GetWorkIDByWorkNumber(ctx, req.WorkNumber)
	if err != nil {
		return nil, errors.Join(GetVersionsByWorkNumberError, err)
	}

	isPaused, err := ae.DB.IsTaskPaused(ctx, workID)
	if err != nil {
		return nil, errors.Join(CheckIsTaskPausedError, err)
	}

	if isPaused {
		return nil, errors.Join(TaskIsPausedError, err)
	}

	err = ae.Scheduler.DeleteAllTasksByWorkID(ctx, workID)
	if err != nil {
		log.WithError(err).Error("failed delete all tasks by work id in scheduler")
	}

	workID = uuid.New()
	log = log.WithField("workID", workID)

	emptyTask := &db.Task{
		WorkID:        workID,
		WorkNumber:    req.WorkNumber,
		Author:        usr.Username,
		ByPrevVersion: true,
		RunContext: &entity.TaskRunContext{
			InitialApplication: entity.InitialApplication{
				Description:               req.Description,
				ApplicationBody:           req.ApplicationBody,
				AttachmentFields:          req.AttachmentFields,
				Keys:                      req.Keys,
				ApplicationBodyFromSystem: req.ApplicationBody,
				CustomTitle:               req.CustomTitle,
				IsTestApplication:         req.IsTestApplication,
			},
		},
	}

	version, err := ae.DB.GetVersionByWorkNumber(ctx, req.WorkNumber)
	if err != nil {
		_ = ae.createEmptyTask(
			ctx,
			ae.DB,
			emptyTask,
		)
		_ = ae.DB.UpdateTaskStatus(ctx, emptyTask.WorkID, db.RunStatusError, GetVersionsByWorkNumberError.error(), "")

		log.WithError(err).Error("GetVersionByPipelineID")

		return nil, errors.Join(GetVersionsByWorkNumberError, err)
	}

	emptyTask.VersionID = version.VersionID

	err = ae.createEmptyTask(ctx,
		ae.DB,
		emptyTask,
	)
	if err != nil {
		return nil, errors.Join(PipelineCreateError, err)
	}

	log = log.
		WithField("versionID", version.VersionID).
		WithField("pipelineID", version.PipelineID)

	ctx = logger.WithLogger(ctx, log)

	reqParams := &requestStartParams{
		version:          version,
		keys:             req.Keys,
		attachmentFields: req.AttachmentFields,
	}

	err = ae.handleStartApplicationParams(ctx, reqParams)
	if err != nil {
		log.Error(GetHiddenFieldsError.errorMessage(err))
	}

	err = ae.execVersion(ctx, &execVersionDTO{
		storage:     ae.DB,
		version:     version,
		makeNewWork: true,
		workNumber:  req.WorkNumber,
		workID:      workID,
		requestID:   requestID,
		runCtx: entity.TaskRunContext{
			InitialApplication: entity.InitialApplication{
				Description:               req.Description,
				ApplicationBody:           req.ApplicationBody,
				AttachmentFields:          reqParams.attachmentFields,
				Keys:                      reqParams.keys,
				ApplicationBodyFromSystem: req.ApplicationBody,
				CustomTitle:               req.CustomTitle,
				IsTestApplication:         req.IsTestApplication,
				HiddenFields:              reqParams.hiddenFields,
			},
		},
	})
	if err != nil {
		log.WithError(err).Error("process empty task error")

		_ = ae.DB.UpdateTaskStatus(ctx, emptyTask.WorkID, db.RunStatusError, err.Error(), "")

		return nil, err
	}

	return &entity.RunResponse{
		PipelineID: version.PipelineID,
		WorkNumber: req.WorkNumber,
		Status:     statusRunned,
	}, nil
}

type runVersionByPipelineIDRequest struct {
	WorkNumber        string                `json:"-"`
	ApplicationBody   orderedmap.OrderedMap `json:"application_body"`
	Description       string                `json:"description"`
	PipelineID        string                `json:"pipeline_id"`
	AttachmentFields  []string              `json:"attachment_fields"`
	Keys              map[string]string     `json:"keys"`
	IsTestApplication bool                  `json:"is_test_application"`
	CustomTitle       string                `json:"custom_title"`
}

//nolint:revive,stylecheck //need to implement interface in api.go
func (ae *Env) RunVersionsByPipelineId(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ctx, s := trace.StartSpan(r.Context(), "run_version_by_pipeline_id")

	log := logger.GetLogger(r.Context()).
		WithField("mainFuncName", "RunVersionsByPipelineId").
		WithField("method", "post").
		WithField("transport", "rest").
		WithField("logVersion", "v1").
		WithField("traceID", s.SpanContext().TraceID.String())

	requestInfo := metrics.NewPostRequestInfo(runByPipelineIDPath)
	defer func() {
		s.End()

		requestInfo.Duration = time.Since(start)

		ae.Metrics.RequestsIncrease(requestInfo)
	}()

	errorHandler := newHTTPErrorHandler(log, w)

	body, err := io.ReadAll(r.Body)
	defer r.Body.Close()

	if err != nil {
		log.Error(err)
		errorHandler.handleError(RequestReadError, err)
		requestInfo.Status = RequestReadError.Status()

		return
	}

	log = log.WithField("body", body)
	ctx = logger.WithLogger(ctx, log)

	req := &runVersionByPipelineIDRequest{}

	if err = json.Unmarshal(body, req); err != nil {
		log.Error(err)
		errorHandler.handleError(BodyParseError, err)
		requestInfo.Status = BodyParseError.Status()

		return
	}

	if req.PipelineID == "" {
		log.Error("empty pipeline id")
		errorHandler.handleError(ValidatePipelineIDError, ValidatePipelineIDError)
		requestInfo.Status = ValidatePipelineIDError.Status()

		return
	}

	requestInfo.PipelineID = req.PipelineID

	log = log.WithField("pipelineID", req.PipelineID)

	ctx = logger.WithLogger(ctx, log)

	errorHandler.log = log

	if req.WorkNumber == "" {
		req.WorkNumber, err = ae.Sequence.GetWorkNumber(r.Context())
		if err != nil {
			log.WithField("funcName", "GetWorkNumber").Error(err)
			errorHandler.handleError(GetWorkNumberError, err)
			requestInfo.Status = GetWorkNumberError.Status()

			return
		}
	}

	requestInfo.WorkNumber = req.WorkNumber

	run := &runVersionsDTO{
		WorkNumber:        req.WorkNumber,
		Description:       req.Description,
		PipelineID:        req.PipelineID,
		AttachmentFields:  req.AttachmentFields,
		Keys:              req.Keys,
		IsTestApplication: req.IsTestApplication,
		CustomTitle:       req.CustomTitle,
		ApplicationBody:   req.ApplicationBody,
		RequestID:         r.Header.Get(XRequestIDHeader),
		Authorization:     r.Header.Get(AuthorizationHeader),
		requestInfo:       requestInfo,
	}

	err = ae.runVersion(ctx, errorHandler.log, run)
	if err != nil {
		httpErr := getErr(err)

		errorHandler.handleError(httpErr, err)
		requestInfo.Status = httpErr.Status()

		return
	}

	pipelineID, _ := uuid.Parse(req.PipelineID)

	resp := &entity.RunResponse{
		PipelineID: pipelineID,
		WorkNumber: req.WorkNumber,
		Status:     statusRunned,
	}

	if err = sendResponse(w, http.StatusOK, []*entity.RunResponse{resp}); err != nil {
		errorHandler.handleError(UnknownError, err)
		requestInfo.Status = UnknownError.Status()

		return
	}
}

func (ae *Env) runVersion(ctx c.Context, log logger.Logger, run *runVersionsDTO) error {
	var err error

	ctx, s := trace.StartSpan(ctx, "run_version")
	defer s.End()

	if run.ClientID == "" {
		run.ClientID, err = ae.getClientIDFromToken(run.Authorization)
		if err != nil {
			return errors.Join(GetClientIDError, err)
		}
	}

	log = log.WithField("clientID", run.ClientID).
		WithField("funcName", "runVersion")

	if run.requestInfo != nil {
		run.requestInfo.ClientID = run.ClientID
	}

	storage, err := ae.DB.Acquire(ctx)
	if err != nil {
		log.Error(err)

		return errors.Join(PipelineExecutionError, err)
	}

	//nolint:errcheck // нецелесообразно отслеживать подобные ошибки в defer
	defer storage.Release(ctx)

	usr, err := user.GetUserInfoFromCtx(ctx)
	if err != nil {
		log.Error(err)

		return errors.Join(NoUserInContextError, err)
	}

	workID := uuid.New()
	log = log.
		WithField("workNumber", run.WorkNumber).
		WithField("workID", workID)

	ctx = logger.WithLogger(ctx, log)

	emptyTask := &db.Task{
		WorkID:        workID,
		WorkNumber:    run.WorkNumber,
		Author:        usr.Username,
		ByPrevVersion: false,
		RunContext: &entity.TaskRunContext{
			ClientID:   run.ClientID,
			PipelineID: run.PipelineID,
			InitialApplication: entity.InitialApplication{
				Description:               run.Description,
				Keys:                      run.Keys,
				AttachmentFields:          run.AttachmentFields,
				IsTestApplication:         run.IsTestApplication,
				ApplicationBodyFromSystem: run.ApplicationBody,
				CustomTitle:               run.CustomTitle,
			},
		},
	}

	version, err := storage.GetVersionByPipelineID(ctx, emptyTask.RunContext.PipelineID)
	if err != nil {
		_ = ae.createEmptyTask(
			ctx,
			storage,
			emptyTask,
		)
		_ = storage.UpdateTaskStatus(ctx, emptyTask.WorkID, db.RunStatusError, GetVersionsByBlueprintIDError.error(), "")

		log.WithError(err).Error("GetVersionByPipelineID")

		return errors.Join(GetVersionsByBlueprintIDError, err)
	}

	emptyTask.VersionID = version.VersionID

	err = ae.createEmptyTask(
		ctx,
		storage,
		emptyTask,
	)
	if err != nil {
		log.Error(err)

		return errors.Join(PipelineCreateError, err)
	}

	return ae.launchEmptyTask(ctx, storage, emptyTask, run.RequestID, run.requestInfo)
}

func getErr(err error) Err {
	switch {
	case errors.Is(err, MappingError):
		return MappingError
	case errors.Is(err, GetVersionsByBlueprintIDError):
		return GetVersionsByBlueprintIDError
	case errors.Is(err, entity.ErrUnknownAction),
		errors.Is(err, entity.ErrEmptyStepTypes):
		return UpdateTaskError
	default:
		var httpErr Err
		if errors.As(err, &httpErr) {
			return httpErr
		}

		return UnknownError
	}
}

func (ae *Env) launchEmptyTask(
	ctx c.Context,
	storage db.Database,
	emptyTask *db.Task,
	requestID string,
	requestInfo *metrics.RequestInfo,
) error {
	err := ae.processEmptyTask(ctx, storage, emptyTask, requestID, requestInfo)

	return handleLaunchTaskError(ctx, storage, emptyTask.WorkID, err)
}

func handleLaunchTaskError(ctx c.Context, storage db.Database, taskID uuid.UUID, err error) error {
	log := logger.GetLogger(ctx).WithField("funcName", "handleLaunchTaskError")

	switch {
	case errorutils.IsRemoteCallError(err):
		log.WithError(err).Warning("remote call error")

		return nil
	case err != nil:
		log.WithError(err).Error("process empty task error")

		_ = storage.UpdateTaskStatus(ctx, taskID, db.RunStatusError, err.Error(), "")

		return err
	default:
		return nil
	}
}

func (ae *Env) processEmptyTask(
	ctx c.Context,
	storage db.Database,
	emptyTask *db.Task,
	requestID string,
	requestInfo *metrics.RequestInfo,
) error {
	ctx, span := trace.StartSpan(ctx, "process_empty_task")
	defer span.End()

	log := logger.GetLogger(ctx).WithField("funcName", "processEmptyTask")

	version, err := storage.GetVersionByPipelineID(ctx, emptyTask.RunContext.PipelineID)
	if err != nil {
		return errors.Join(GetVersionsByBlueprintIDError, err)
	}

	requestInfo.VersionID = version.VersionID.String()
	log = log.WithField("versionID", requestInfo.VersionID)
	ctx = logger.WithLogger(ctx, log)

	var externalSystem *entity.ExternalSystem

	externalSystem, err = ae.getExternalSystem(
		ctx,
		storage,
		emptyTask.RunContext.ClientID,
		emptyTask.RunContext.PipelineID,
		version.VersionID.String(),
	)
	if err != nil {
		log.WithError(err).Error("getExternalSystem")

		return errors.Join(GetExternalSystemsError, err)
	}

	var allowRunAsOthers bool
	if externalSystem != nil {
		allowRunAsOthers = externalSystem.AllowRunAsOthers
	}

	mappedApplicationBody, err := ae.processMappings(
		externalSystem,
		version,
		emptyTask.RunContext.InitialApplication.ApplicationBodyFromSystem,
	)
	if err != nil {
		return errors.Join(MappingError, err)
	}

	version.FillEntryPointOutput()

	reqParams := &requestStartParams{
		version:          version,
		keys:             emptyTask.RunContext.InitialApplication.Keys,
		attachmentFields: emptyTask.RunContext.InitialApplication.AttachmentFields,
	}

	paramsErr := ae.handleStartApplicationParams(ctx, reqParams)
	if paramsErr != nil {
		log.Error(GetHiddenFieldsError.errorMessage(paramsErr))
	}

	execErr := ae.execVersion(ctx, &execVersionDTO{
		storage:          storage,
		version:          version,
		withStop:         false,
		allowRunAsOthers: allowRunAsOthers,
		workNumber:       emptyTask.WorkNumber,
		workID:           emptyTask.WorkID,
		requestID:        requestID,
		runCtx: entity.TaskRunContext{
			ClientID:   emptyTask.RunContext.ClientID,
			PipelineID: emptyTask.RunContext.PipelineID,
			InitialApplication: entity.InitialApplication{
				Description:               emptyTask.RunContext.InitialApplication.Description,
				ApplicationBody:           mappedApplicationBody,
				Keys:                      reqParams.keys,
				AttachmentFields:          reqParams.attachmentFields,
				IsTestApplication:         emptyTask.RunContext.InitialApplication.IsTestApplication,
				ApplicationBodyFromSystem: emptyTask.RunContext.InitialApplication.ApplicationBodyFromSystem,
				CustomTitle:               emptyTask.RunContext.InitialApplication.CustomTitle,
				HiddenFields:              reqParams.hiddenFields,
			},
		},
	})
	if execErr != nil {
		return execErr
	}

	return nil
}

func (ae *Env) handleStartApplicationParams(ctx c.Context, dto *requestStartParams) error {
	hiddenFields, err := ae.getHiddenFields(ctx, dto.version)
	if err != nil {
		ae.Log.Error(GetHiddenFieldsError.errorMessage(err))
	}

	dto.hiddenFields = hiddenFields

	if len(dto.keys) != 0 && len(dto.attachmentFields) != 0 {
		return nil
	}

	var schemaJSON jsonschema.Schema
	if unmarshalErr := json.Unmarshal(dto.version.Settings.StartSchemaRaw, &schemaJSON); unmarshalErr != nil {
		return unmarshalErr
	}

	if len(schemaJSON) == 0 {
		return errors.New("schema is empty")
	}

	if len(dto.hiddenFields) == 0 {
		if hiddenFields, err = schemaJSON.GetHiddenFields(); err == nil {
			dto.hiddenFields = hiddenFields
		}
	}

	schemaJSON = checkGroup(schemaJSON)

	if len(dto.keys) == 0 {
		if res, _, getErr := schemaJSON.GetAllFields(); getErr == nil {
			dto.keys = res
		}
	}

	if len(dto.attachmentFields) == 0 {
		dto.attachmentFields = schemaJSON.GetAttachmentFields()
	}

	return nil
}

func (ae *Env) getHiddenFields(ctx c.Context, version *entity.EriusScenario) ([]string, error) {
	const sdBlockName = "servicedesk_application_0"

	hiddenFields := make([]string, 0)

	if _, exists := version.Pipeline.Blocks[sdBlockName]; !exists {
		return hiddenFields, nil
	}

	params := pipeline.ApplicationData{
		ApplicationBody: make(map[string]interface{}, 0),
	}

	errJSON := json.Unmarshal(version.Pipeline.Blocks[sdBlockName].Params, &params)
	if errJSON != nil {
		return hiddenFields, errJSON
	}

	ae.Log.Info("params", fmt.Sprintf("%+v", params))

	if params.BlueprintID == "" {
		return hiddenFields, errors.New("can`t find blueprintID")
	}

	var (
		schema jsonschema.Schema
		err    error
	)

	schema, err = ae.ServiceDesc.GetSchemaByBlueprintID(ctx, params.BlueprintID)
	if err != nil {
		return hiddenFields, err
	}

	ae.Log.Info("schema", fmt.Sprintf("%+v", schema))

	hiddenFields, err = schema.GetHiddenFields()
	if err != nil {
		return hiddenFields, err
	}

	ae.Log.Info("hiddenFields", fmt.Sprintf("%+v", hiddenFields))

	return hiddenFields, nil
}

//nolint:gocognit //it's ok
func checkGroup(rawStartSchema jsonschema.Schema) jsonschema.Schema {
	properties, ok := rawStartSchema[propertiesKey]
	if !ok {
		return rawStartSchema
	}

	propertiesMap := properties.(map[string]interface{})

	for k, v := range propertiesMap {
		valMap, mapOk := v.(map[string]interface{})
		if !mapOk {
			continue
		}

		if valMap[titleKey] == "" {
			valMap[titleKey] = " "
		} else {
			newTitle := cleanKey(v)
			if newTitle != "" {
				valMap[titleKey] = newTitle
			}
		}

		propVal, propValOk := valMap[propertiesKey]
		if !propValOk {
			continue
		}

		propValMap := propVal.(map[string]interface{})
		if _, ok := propValMap[emailKey]; ok {
			continue
		}

		for key, val := range propValMap {
			valMaps, valOk := v.(map[string]interface{})
			if !valOk {
				propertiesMap[key] = val

				continue
			}

			if valMaps[titleKey] == "" {
				valMap[titleKey] = " "
			} else {
				newAdTitle := cleanKey(val)
				if newAdTitle != "" {
					valMaps[titleKey] = newAdTitle
				}
			}

			propVals, propValOks := valMaps[propertiesKey]
			if !propValOks {
				continue
			}

			propMap := propVals.(map[string]interface{})
			if _, ok := propMap[emailKey]; ok {
				continue
			}

			for propMapKey, propMapVal := range propMap {
				propertiesMap[propMapKey] = propMapVal
			}
		}

		delete(propertiesMap, k)
	}

	return rawStartSchema
}

func cleanKey(mapKeys interface{}) string {
	keys, ok := mapKeys.(map[string]interface{})
	if !ok {
		return ""
	}

	key, oks := keys[titleKey]
	if !oks {
		return ""
	}

	keyStr, okStr := key.(string)
	if !okStr {
		return ""
	}

	return utils.CleanUnexpectedSymbols(keyStr)
}

func (ae *Env) createEmptyTask(
	ctx c.Context,
	storage db.Database,
	dto *db.Task,
) error {
	ctx, span := trace.StartSpan(ctx, "create_empty_task")
	defer span.End()

	txStorage, err := storage.StartTransaction(ctx)
	if err != nil {
		return fmt.Errorf("start transaction, %w", err)
	}

	defer func() {
		rollBackErr := txStorage.RollbackTransaction(ctx)
		if rollBackErr != nil {
			ae.Log.WithError(rollBackErr).Error("rollback transaction")
		}
	}()

	err = txStorage.CreateEmptyTask(ctx, dto)
	if err != nil {
		return fmt.Errorf("create empty task in database, %w", err)
	}

	createTime := time.Now()

	block := pipeline.Block{
		DB:            txStorage,
		Name:          pipeline.BlockGoFirstStart,
		StepType:      pipeline.BlockGoStartID,
		WorkID:        dto.WorkID,
		VarStore:      store.NewStore(),
		IsPaused:      false,
		HasUpdateData: false,
		Time:          createTime,
	}

	err = block.CreateInDB(ctx)
	if err != nil {
		return fmt.Errorf("create start block in db, %w", err)
	}

	_, err = txStorage.CreateTaskEvent(ctx, &entity.CreateTaskEvent{
		WorkID:    dto.WorkID.String(),
		Author:    dto.Author,
		EventType: string(MonitoringTaskActionRequestActionStart),
		Params:    []byte(`{"steps":[]}`),
		Time:      createTime,
	})
	if err != nil {
		return fmt.Errorf("create task event, %w", err)
	}

	err = txStorage.CommitTransaction(ctx)
	if err != nil {
		return fmt.Errorf("commit transaction, %w", err)
	}

	return nil
}
