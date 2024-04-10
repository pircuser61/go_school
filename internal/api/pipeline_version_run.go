package api

import (
	c "context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/user"
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

	log := logger.GetLogger(ctx).
		WithField("funcName", "RunNewVersionByPrevVersion")
	errorHandler := newHTTPErrorHandler(log, w)

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

	log = log.WithField("workNumber", req.WorkNumber)

	if req.WorkNumber == "" {
		errorHandler.handleError(ValidationError, errors.New("workNumber is empty"))

		return
	}

	usr, err := user.GetUserInfoFromCtx(ctx)
	if err != nil {
		errorHandler.handleError(NoUserInContextError, err)

		return
	}

	taskID := uuid.New()
	log = log.WithField("workID", taskID)

	err = ae.createEmptyTask(ctx, ae.DB, &db.CreateEmptyTaskDTO{
		TaskID:     taskID,
		WorkNumber: req.WorkNumber,
		Author:     usr.Username,
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

	workID, err := ae.DB.GetWorkIDByWorkNumber(ctx, req.WorkNumber)
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

	version, err := ae.DB.GetVersionByWorkNumber(ctx, req.WorkNumber)
	if err != nil {
		errorHandler.handleError(GetVersionsByWorkNumberError, err)

		return
	}

	log = log.WithField("versionID", version.VersionID).
		WithField("pipelineID", version.PipelineID)
	ctx = logger.WithLogger(ctx, log)

	reqParams := &requestStartParams{
		version:          version,
		keys:             req.Keys,
		attachmentFields: req.AttachmentFields,
	}

	err = ae.handleStartApplicationParams(ctx, reqParams)
	if err != nil {
		e := GetHiddenFieldsError
		log.Error(e.errorMessage(err))
	}

	started, execErr := ae.execVersion(ctx, &execVersionDTO{
		storage:     ae.DB,
		version:     version,
		w:           w,
		req:         r,
		makeNewWork: true,
		workNumber:  req.WorkNumber,
		taskID:      taskID,
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
		log.WithError(err).Error("failed delete all tasks by work id in scheduler")
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
	start := time.Now()
	ctx, s := trace.StartSpan(r.Context(), "run_version_by_pipeline_id")

	requestInfo := metrics.NewPostRequestInfo(runByPipelineIDPath)

	defer func() {
		s.End()

		requestInfo.Duration = time.Since(start)

		ae.Metrics.RequestsIncrease(requestInfo)
	}()

	log := logger.GetLogger(ctx).
		WithField("funcName", "RunVersionsByPipelineId")
	errorHandler := newHTTPErrorHandler(log, w)
	errorHandler.setMetricsRequestInfo(requestInfo)

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

	log = log.WithField("pipelineID", req.PipelineID).
		WithField("funcName", "RunVersionsByPipelineId")

	requestInfo.PipelineID = req.PipelineID

	if req.PipelineID == "" {
		errorHandler.handleError(ValidationError, errors.New("pipelineID is empty"))

		return
	}

	log.WithField("body", req).Info("RunVersionsByPipelineId pipeline_id:", req.PipelineID)

	clientID, err := ae.getClientIDFromToken(r.Header.Get(AuthorizationHeader))
	if err != nil {
		errorHandler.handleError(GetClientIDError, err)

		return
	}

	log = log.WithField("clientID", clientID)
	requestInfo.ClientID = clientID

	storage, acquireErr := ae.DB.Acquire(ctx)
	if acquireErr != nil {
		errorHandler.handleError(PipelineExecutionError, acquireErr)

		return
	}

	usr, err := user.GetUserInfoFromCtx(ctx)
	if err != nil {
		errorHandler.handleError(NoUserInContextError, err)

		return
	}

	workNumber, err := ae.Sequence.GetWorkNumber(ctx)
	if err != nil {
		errorHandler.handleError(GetWorkNumberError, err)

		return
	}

	taskID := uuid.New()
	log = log.WithField("workID", taskID)

	err = ae.createEmptyTask(ctx, storage,
		&db.CreateEmptyTaskDTO{
			TaskID:     taskID,
			WorkNumber: workNumber,
			Author:     usr.Username,
			RunContext: &entity.TaskRunContext{
				ClientID:   clientID,
				PipelineID: req.PipelineID,
				InitialApplication: entity.InitialApplication{
					Description:               req.Description,
					Keys:                      req.Keys,
					AttachmentFields:          req.AttachmentFields,
					IsTestApplication:         req.IsTestApplication,
					ApplicationBodyFromSystem: req.ApplicationBody,
					CustomTitle:               req.CustomTitle,
				},
			},
		},
	)
	if err != nil {
		errorHandler.handleError(PipelineCreateError, err)

		return
	}

	log = log.WithField("workNumber", workNumber)

	//nolint:errcheck // нецелесообразно отслеживать подобные ошибки в defer
	defer storage.Release(ctx)

	version, err := storage.GetVersionByPipelineID(ctx, req.PipelineID)
	if err != nil {
		errorHandler.handleError(GetVersionsByBlueprintIDError, err)
		_ = ae.DB.UpdateTaskStatus(ctx, taskID, db.RunStatusError, GetVersionsByBlueprintIDError.error(), "")

		return
	}

	requestInfo.VersionID = version.VersionID.String()
	log = log.WithField("versionID", requestInfo.VersionID)
	ctx = logger.WithLogger(ctx, log)

	var externalSystem *entity.ExternalSystem

	externalSystem, err = ae.getExternalSystem(ctx, storage, clientID, req.PipelineID, version.VersionID.String())
	if err != nil {
		errorHandler.handleError(GetExternalSystemsError, err)
		_ = ae.DB.UpdateTaskStatus(ctx, taskID, db.RunStatusError, GetExternalSystemsError.error(), "")

		return
	}

	var allowRunAsOthers bool
	if externalSystem != nil {
		allowRunAsOthers = externalSystem.AllowRunAsOthers
	}

	mappedApplicationBody, err := ae.processMappings(externalSystem, version, req.ApplicationBody)
	if err != nil {
		errorHandler.handleError(MappingError, err)
		_ = ae.DB.UpdateTaskStatus(ctx, taskID, db.RunStatusError, MappingError.error(), "")

		return
	}

	if err = version.FillEntryPointOutput(); err != nil {
		errorHandler.handleError(GetEntryPointOutputError, err)
		_ = ae.DB.UpdateTaskStatus(ctx, taskID, db.RunStatusError, GetEntryPointOutputError.error(), "")

		return
	}

	reqParams := &requestStartParams{
		version:          version,
		keys:             req.Keys,
		attachmentFields: req.AttachmentFields,
	}

	paramsErr := ae.handleStartApplicationParams(ctx, reqParams)
	if paramsErr != nil {
		e := GetHiddenFieldsError
		log.Error(e.errorMessage(err))
	}

	v, execErr := ae.execVersion(ctx, &execVersionDTO{
		storage:          storage,
		version:          version,
		withStop:         false,
		w:                w,
		req:              r,
		allowRunAsOthers: allowRunAsOthers,
		workNumber:       workNumber,
		taskID:           taskID,
		runCtx: entity.TaskRunContext{
			ClientID:   clientID,
			PipelineID: req.PipelineID,
			InitialApplication: entity.InitialApplication{
				Description:               req.Description,
				ApplicationBody:           mappedApplicationBody,
				Keys:                      reqParams.keys,
				AttachmentFields:          reqParams.attachmentFields,
				IsTestApplication:         req.IsTestApplication,
				ApplicationBodyFromSystem: req.ApplicationBody,
				CustomTitle:               req.CustomTitle,
				HiddenFields:              reqParams.hiddenFields,
			},
		},
	})
	if execErr != nil {
		errorHandler.handleError(PipelineExecutionError, execErr)

		return
	}

	if v == nil {
		errorHandler.handleError(PipelineExecutionError, errors.New("run_version_by_pipeline_id execution error"))

		return
	}

	requestInfo.WorkNumber = v.WorkNumber

	if err = sendResponse(w, http.StatusOK, []*entity.RunResponse{v}); err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
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

	return cleanUnexpectedSymbols(keyStr)
}

func cleanUnexpectedSymbols(s string) string {
	replacements := map[string]string{
		"\\t":  "",
		"\t":   "",
		"\\n":  "",
		"\n":   "",
		"\r":   "",
		"\\r":  "",
		"\"\"": "",
		"\"":   "''",
	}

	for old, news := range replacements {
		s = strings.ReplaceAll(s, old, news)
	}

	return strings.ReplaceAll(s, "\\", "")
}

func (ae *Env) createEmptyTask(ctx c.Context, storage db.Database, dto *db.CreateEmptyTaskDTO) error {
	txStorage, err := storage.StartTransaction(ctx)
	if err != nil {
		return fmt.Errorf("failed start transaction, %w", err)
	}

	defer func() {
		rollbackerr := txStorage.RollbackTransaction(ctx)
		if rollbackerr != nil {
			ae.Log.WithError(rollbackerr).Error("failed rollback transaction")
		}
	}()

	err = txStorage.CreateEmptyTask(ctx, dto)
	if err != nil {
		return fmt.Errorf("failed create empty task in database, %w", err)
	}

	err = txStorage.CommitTransaction(ctx)
	if err != nil {
		return fmt.Errorf("failed commit transaction, %w", err)
	}

	return nil
}
