package api

import (
	c "context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/iancoleman/orderedmap"

	"github.com/pkg/errors"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/forms/pkg/jsonschema"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/metrics"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
)

const runByPipelineIDPath = "/run/versions/pipeline_id"

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

	log := logger.GetLogger(ctx)
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

	if req.WorkNumber == "" {
		errorHandler.handleError(ValidationError, errors.New("workNumber is empty"))

		return
	}

	version, err := ae.DB.GetVersionByWorkNumber(ctx, req.WorkNumber)
	if err != nil {
		errorHandler.handleError(GetVersionsByWorkNumberError, err)

		return
	}

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

	err = sendResponse(w, http.StatusOK, started)
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

type runVersionByPipelineIDRequest struct {
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

	requestInfo := &metrics.RequestInfo{Method: http.MethodPost, Path: runByPipelineIDPath}

	defer func() {
		s.End()

		requestInfo.Duration = time.Since(start)

		ae.Metrics.RequestsIncrease(requestInfo)
	}()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	body, err := io.ReadAll(r.Body)
	defer r.Body.Close()

	if err != nil {
		e := RequestReadError
		requestInfo.Status = e.Status()

		errorHandler.handleError(e, err)

		return
	}

	req := &runVersionByPipelineIDRequest{}

	if err = json.Unmarshal(body, req); err != nil {
		e := BodyParseError
		requestInfo.Status = e.Status()

		errorHandler.handleError(BodyParseError, err)

		return
	}

	requestInfo.PipelineID = req.PipelineID

	if req.PipelineID == "" {
		e := ValidationError
		requestInfo.Status = e.Status()

		errorHandler.handleError(e, errors.New("pipelineID is empty"))

		return
	}

	log.Info("RunVersionsByPipelineId pipeline_id:", req.PipelineID)

	storage, acquireErr := ae.DB.Acquire(ctx)
	if acquireErr != nil {
		e := PipelineExecutionError
		requestInfo.Status = e.Status()

		errorHandler.handleError(PipelineExecutionError, acquireErr)

		return
	}

	//nolint:errcheck // нецелесообразно отслеживать подобные ошибки в defer
	defer storage.Release(ctx)

	version, err := storage.GetVersionByPipelineID(ctx, req.PipelineID)
	if err != nil {
		e := GetVersionsByBlueprintIDError
		requestInfo.Status = e.Status()

		errorHandler.handleError(e, err)

		return
	}

	requestInfo.VersionID = version.VersionID.String()

	var clientID string

	clientID, err = ae.getClientIDFromToken(r.Header.Get(AuthorizationHeader))
	if err != nil {
		e := GetClientIDError
		requestInfo.Status = e.Status()

		errorHandler.handleError(e, err)

		return
	}

	requestInfo.ClientID = clientID

	var externalSystem *entity.ExternalSystem

	externalSystem, err = ae.getExternalSystem(ctx, storage, clientID, req.PipelineID, version.VersionID.String())
	if err != nil {
		e := GetExternalSystemsError
		requestInfo.Status = e.Status()

		errorHandler.handleError(e, err)

		return
	}

	var allowRunAsOthers bool
	if externalSystem != nil {
		allowRunAsOthers = externalSystem.AllowRunAsOthers
	}

	var mappedApplicationBody orderedmap.OrderedMap

	mappedApplicationBody, err = ae.processMappings(externalSystem, version, req.ApplicationBody)
	if err != nil {
		e := MappingError
		requestInfo.Status = e.Status()

		errorHandler.handleError(e, err)

		return
	}

	if err = version.FillEntryPointOutput(); err != nil {
		e := GetEntryPointOutputError
		requestInfo.Status = e.Status()

		errorHandler.handleError(e, err)

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
		runCtx: entity.TaskRunContext{
			ClientID: clientID,
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
		e := PipelineExecutionError
		requestInfo.Status = e.Status()

		errorHandler.handleError(e, execErr)

		return
	}

	if v == nil {
		e := PipelineExecutionError
		requestInfo.Status = e.Status()

		errorHandler.handleError(e, errors.New("run_version_by_pipeline_id execution error"))

		return
	}

	requestInfo.WorkNumber = v.WorkNumber
	requestInfo.Status = http.StatusOK

	if err = sendResponse(w, http.StatusOK, []*entity.RunResponse{v}); err != nil {
		e := UnknownError
		requestInfo.Status = e.Status()

		errorHandler.handleError(e, err)

		return
	}
}

func (ae *Env) handleStartApplicationParams(ctx c.Context, dto *requestStartParams) error {
	hiddenFields, err := ae.getHiddenFields(ctx, dto.version)
	if err != nil {
		e := GetHiddenFieldsError
		ae.Log.Error(e.errorMessage(err))
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
