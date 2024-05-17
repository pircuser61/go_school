package api

import (
	c "context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/iancoleman/orderedmap"

	"github.com/pkg/errors"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/forms/pkg/jsonschema"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/metrics"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/user"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

const (
	runByPipelineIDPath = "/run/versions/pipeline_id"

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

	errorHandler := newHTTPErrorHandler(
		logger.GetLogger(ctx).
			WithField("funcName", "RunNewVersionByPrevVersion"),
		w,
	)

	body, err := io.ReadAll(r.Body)
	defer r.Body.Close()

	if err != nil {
		errorHandler.handleError(RequestReadError, err)

		return
	}

	req := &runNewVersionsByPrevVersionRequest{}

	if err = json.Unmarshal(body, req); err != nil {
		errorHandler.handleError(BodyParseError, err)

		return
	}

	errorHandler.log = errorHandler.log.WithField("workNumber", req.WorkNumber)

	if req.WorkNumber == "" {
		errorHandler.handleError(ValidationError, errors.New("workNumber is empty"))

		return
	}

	usr, err := user.GetUserInfoFromCtx(ctx)
	if err != nil {
		errorHandler.handleError(NoUserInContextError, err)

		return
	}

	workID := uuid.New()
	errorHandler.log = errorHandler.log.WithField("workID", workID)

	err = ae.createEmptyTask(ctx,
		ae.DB,
		&db.CreateEmptyTaskDTO{
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
		})
	if err != nil {
		errorHandler.handleError(PipelineCreateError, err)

		return
	}

	version, err := ae.DB.GetVersionByWorkNumber(ctx, req.WorkNumber)
	if err != nil {
		errorHandler.handleError(GetVersionsByWorkNumberError, err)

		return
	}

	errorHandler.log = errorHandler.log.
		WithField("versionID", version.VersionID).
		WithField("pipelineID", version.PipelineID)

	ctx = logger.WithLogger(ctx, errorHandler.log)

	workID, err = ae.DB.GetWorkIDByWorkNumber(ctx, req.WorkNumber)
	if err != nil {
		errorHandler.handleError(ValidationError, err)

		return
	}

	isPaused, err := ae.DB.IsTaskPaused(ctx, workID)
	if err != nil {
		errorHandler.handleError(CheckIsTaskPausedError, err)

		return
	}

	if isPaused {
		errorHandler.handleError(TaskIsPausedError, err)

		return
	}

	reqParams := &requestStartParams{
		version:          version,
		keys:             req.Keys,
		attachmentFields: req.AttachmentFields,
	}

	err = ae.handleStartApplicationParams(ctx, reqParams)
	if err != nil {
		errorHandler.log.Error(GetHiddenFieldsError.errorMessage(err))
	}

	started, execErr := ae.execVersion(ctx, &execVersionDTO{
		storage:     ae.DB,
		version:     version,
		makeNewWork: true,
		workNumber:  req.WorkNumber,
		workID:      workID,
		requestID:   r.Header.Get(XRequestIDHeader),
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

	if execErr != nil {
		errorHandler.handleError(UnknownError, execErr)

		return
	}

	if started == nil {
		errorHandler.handleError(UnknownError, errors.New("no one version was started"))

		return
	}

	err = ae.Scheduler.DeleteAllTasksByWorkID(ctx, workID)
	if err != nil {
		errorHandler.log.WithError(err).Error("failed delete all tasks by work id in scheduler")
	}

	err = sendResponse(w, http.StatusOK, started)
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
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
	errorHandler := newHTTPErrorHandler(
		logger.
			GetLogger(r.Context()).
			WithField("funcName", "RunVersionsByPipelineId"),
		w,
	)

	body, err := io.ReadAll(r.Body)
	defer r.Body.Close()

	if err != nil {
		errorHandler.handleError(RequestReadError, err)

		return
	}

	req := &runVersionByPipelineIDRequest{}

	if err = json.Unmarshal(body, req); err != nil {
		errorHandler.handleError(BodyParseError, err)

		return
	}

	if req.PipelineID == "" {
		errorHandler.handleError(ValidationError, errors.New("pipelineID is empty"))

		return
	}

	errorHandler.log = errorHandler.log.WithField("pipelineID", req.PipelineID).
		WithField("funcName", "RunVersionsByPipelineId")

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
	}

	res, err := ae.runVersion(r.Context(), errorHandler.log, run)
	if err != nil {
		errorHandler.handleError(PipelineExecutionError, err)

		return
	}

	if err = sendResponse(w, http.StatusOK, []*entity.RunResponse{res}); err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

func (ae *Env) runVersion(ctx c.Context, log logger.Logger, run *runVersionsDTO) (*entity.RunResponse, error) {
	var err error

	ctx, s := trace.StartSpan(ctx, "run_version")

	requestInfo := metrics.NewPostRequestInfo(runByPipelineIDPath)

	defer func() {
		s.End()

		requestInfo.Duration = time.Since(time.Now())

		ae.Metrics.RequestsIncrease(requestInfo)
	}()

	requestInfo.PipelineID = run.PipelineID

	if run.ClientID == "" {
		run.ClientID, err = ae.getClientIDFromToken(run.Authorization)
		if err != nil {
			return nil, errors.Wrap(err, GetClientIDError.error())
		}
	}

	log = log.WithField("clientID", run.ClientID)
	requestInfo.ClientID = run.ClientID

	storage, err := ae.DB.Acquire(ctx)
	if err != nil {
		return nil, errors.Wrap(err, PipelineExecutionError.error())
	}

	//nolint:errcheck // нецелесообразно отслеживать подобные ошибки в defer
	defer storage.Release(ctx)

	usr, err := user.GetUserInfoFromCtx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, NoUserInContextError.error())
	}

	if run.WorkNumber == "" {
		run.WorkNumber, err = ae.Sequence.GetWorkNumber(ctx)
		if err != nil {
			return nil, errors.Wrap(err, GetWorkNumberError.error())
		}
	}

	log = log.WithField("workNumber", run.WorkNumber)

	workID := uuid.New()
	log = log.WithField("workID", workID)

	err = ae.createEmptyTask(
		ctx,
		storage,
		&db.CreateEmptyTaskDTO{
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
		},
	)
	if err != nil {
		return nil, errors.Wrap(err, PipelineCreateError.error())
	}

	version, err := storage.GetVersionByPipelineID(ctx, run.PipelineID)
	if err != nil {
		_ = ae.DB.UpdateTaskStatus(ctx, workID, db.RunStatusError, MappingError.error(), "")

		log.WithError(err).Error("GetVersionByPipelineID")

		return nil, errors.Wrap(err, GetVersionsByBlueprintIDError.error())
	}

	requestInfo.VersionID = version.VersionID.String()
	log = log.WithField("versionID", requestInfo.VersionID)
	ctx = logger.WithLogger(ctx, log)

	var externalSystem *entity.ExternalSystem

	externalSystem, err = ae.getExternalSystem(ctx, storage, run.ClientID, run.PipelineID, version.VersionID.String())
	if err != nil {
		_ = ae.DB.UpdateTaskStatus(ctx, workID, db.RunStatusError, GetExternalSystemsError.error(), "")

		log.WithError(err).Error("getExternalSystem")

		return nil, errors.Wrap(err, GetExternalSystemsError.error())
	}

	var allowRunAsOthers bool
	if externalSystem != nil {
		allowRunAsOthers = externalSystem.AllowRunAsOthers
	}

	mappedApplicationBody, err := ae.processMappings(externalSystem, version, run.ApplicationBody)
	if err != nil {
		_ = ae.DB.UpdateTaskStatus(ctx, workID, db.RunStatusError, MappingError.error(), "")

		log.WithError(err).Error("processMappings")

		return nil, errors.Wrap(err, MappingError.error())
	}

	if err = version.FillEntryPointOutput(); err != nil {
		_ = ae.DB.UpdateTaskStatus(ctx, workID, db.RunStatusError, GetEntryPointOutputError.error(), "")

		log.WithError(err).Error("entry")

		return nil, errors.Wrap(err, GetEntryPointOutputError.error())
	}

	reqParams := &requestStartParams{
		version:          version,
		keys:             run.Keys,
		attachmentFields: run.AttachmentFields,
	}

	paramsErr := ae.handleStartApplicationParams(ctx, reqParams)
	if paramsErr != nil {
		log.Error(GetHiddenFieldsError.errorMessage(paramsErr))
	}

	runRes, execErr := ae.execVersion(ctx, &execVersionDTO{
		storage:          storage,
		version:          version,
		withStop:         false,
		allowRunAsOthers: allowRunAsOthers,
		workNumber:       run.WorkNumber,
		workID:           workID,
		requestID:        run.RequestID,
		runCtx: entity.TaskRunContext{
			ClientID:   run.ClientID,
			PipelineID: run.PipelineID,
			InitialApplication: entity.InitialApplication{
				Description:               run.Description,
				ApplicationBody:           mappedApplicationBody,
				Keys:                      reqParams.keys,
				AttachmentFields:          reqParams.attachmentFields,
				IsTestApplication:         run.IsTestApplication,
				ApplicationBodyFromSystem: run.ApplicationBody,
				CustomTitle:               run.CustomTitle,
				HiddenFields:              reqParams.hiddenFields,
			},
		},
	})
	if execErr != nil {
		return nil, errors.Wrap(execErr, PipelineCreateError.error())
	}

	if runRes == nil {
		return nil, errors.New(PipelineExecutionError.error())
	}

	requestInfo.WorkNumber = runRes.WorkNumber

	return runRes, nil
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

	params := pipeline.ApplicationData{}

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
	dto *db.CreateEmptyTaskDTO,
) error {
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

	err = createStartBlock(ctx, txStorage, dto.WorkID, dto.Author)
	if err != nil {
		return err
	}

	err = txStorage.CommitTransaction(ctx)
	if err != nil {
		return fmt.Errorf("commit transaction, %w", err)
	}

	return nil
}

func createStartBlock(ctx c.Context, tx db.Database, workID uuid.UUID, author string) error {
	log := logger.GetLogger(ctx)

	block := pipeline.Block{
		DB:            tx,
		Name:          pipeline.BlockGoFirstStart,
		StepType:      pipeline.BlockGoStartID,
		WorkID:        workID,
		VarStore:      store.NewStore(),
		IsPaused:      false,
		HasUpdateData: false,
	}

	err := block.CreateInDB(ctx)
	if err != nil {
		return fmt.Errorf("create start block in db, %w", err)
	}

	_, err = tx.CreateTaskEvent(ctx, &entity.CreateTaskEvent{
		WorkID:    workID.String(),
		Author:    author,
		EventType: string(MonitoringTaskActionRequestActionStart),
		Params:    []byte(`{"steps":[]}`),
	})
	if err != nil {
		log.WithField("funcName", "CreateTaskEvent").Error(err)
	}

	return nil
}
