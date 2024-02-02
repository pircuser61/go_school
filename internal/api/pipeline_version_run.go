package api

import (
	c "context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	om "github.com/iancoleman/orderedmap"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/forms/pkg/jsonschema"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/metrics"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
)

const runByPipelineIDPath = "/run/versions/pipeline_id"

type runNewVersionsByPrevVersionRequest struct {
	ApplicationBody   om.OrderedMap     `json:"application_body"`
	Description       string            `json:"description"`
	WorkNumber        string            `json:"work_number"`
	AttachmentFields  []string          `json:"attachment_fields"`
	Keys              map[string]string `json:"keys"`
	CustomTitle       string            `json:"custom_title"`
	IsTestApplication bool              `json:"is_test_application"`
}

type requestStartParams struct {
	pipelineID       string
	version          *entity.EriusScenario
	keys             map[string]string
	attachmentFields []string
	hiddenFields     []string
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

	if err = json.Unmarshal(body, req); err != nil {
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

	reqParams := &requestStartParams{version.PipelineID.String(), version, req.Keys, req.AttachmentFields, nil}

	err = ae.handleStartApplicationParams(ctx, reqParams)
	if err != nil {
		e := GetHiddenFieldsError
		log.Error(e.errorMessage(err))
	}

	started, execErr := ae.execVersion(ctx, &execVersionDTO{
		storage:     ae.DB,
		version:     version,
		withStop:    false,
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

type runVersionByPipelineIDRequest struct {
	ApplicationBody   om.OrderedMap     `json:"application_body"`
	Description       string            `json:"description"`
	PipelineId        string            `json:"pipeline_id"`
	AttachmentFields  []string          `json:"attachment_fields"`
	Keys              map[string]string `json:"keys"`
	IsTestApplication bool              `json:"is_test_application"`
	CustomTitle       string            `json:"custom_title"`
}

//nolint:gocyclo //its ok here
func (ae *APIEnv) RunVersionsByPipelineId(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ctx, s := trace.StartSpan(r.Context(), "run_version_by_pipeline_id")

	requestInfo := &metrics.RequestInfo{Method: http.MethodGet, Path: runByPipelineIDPath}
	defer func() {
		s.End()
		requestInfo.Duration = time.Since(start)
		ae.Metrics.RequestsIncrease(requestInfo)
	}()

	log := logger.GetLogger(ctx)

	body, err := io.ReadAll(r.Body)
	defer r.Body.Close()

	if err != nil {
		e := RequestReadError
		log.Error(e.errorMessage(err))
		requestInfo.Status = e.Status()
		_ = e.sendError(w)

		return
	}

	req := &runVersionByPipelineIDRequest{}

	if err = json.Unmarshal(body, req); err != nil {
		e := BodyParseError
		log.Error(e.errorMessage(err))
		requestInfo.Status = e.Status()
		_ = e.sendError(w)

		return
	}

	requestInfo.PipelineID = req.PipelineId

	if req.PipelineId == "" {
		e := ValidationError
		log.Error(e.errorMessage(errors.New("pipelineID is empty")))
		requestInfo.Status = e.Status()
		_ = e.sendError(w)

		return
	}

	storage, acquireErr := ae.DB.Acquire(ctx)
	if acquireErr != nil {
		e := PipelineExecutionError
		log.Error(e.errorMessage(acquireErr))
		requestInfo.Status = e.Status()
		_ = e.sendError(w)

		return
	}

	defer storage.Release(ctx)

	version, err := storage.GetVersionByPipelineID(ctx, req.PipelineId)
	if err != nil {
		e := GetVersionsByBlueprintIdError
		log.Error(e.errorMessage(err))
		requestInfo.Status = e.Status()
		_ = e.sendError(w)

		return
	}

	requestInfo.VersionID = version.VersionID.String()

	var clientID string
	clientID, err = ae.getClientIDFromToken(r.Header.Get(AuthorizationHeader))
	if err != nil {
		e := GetClientIDError
		log.Error(e.errorMessage(err))
		requestInfo.Status = e.Status()
		_ = e.sendError(w)

		return
	}

	requestInfo.ClientID = clientID

	var externalSystem *entity.ExternalSystem
	externalSystem, err = ae.getExternalSystem(ctx, storage, clientID, req.PipelineId, version.VersionID.String())
	if err != nil {
		e := GetExternalSystemsError
		log.Error(e.errorMessage(err))
		requestInfo.Status = e.Status()
		_ = e.sendError(w)

		return
	}

	var allowRunAsOthers bool
	if externalSystem != nil {
		allowRunAsOthers = externalSystem.AllowRunAsOthers
	}

	var mappedApplicationBody om.OrderedMap
	mappedApplicationBody, err = ae.processMappings(externalSystem, version, req.ApplicationBody)
	if err != nil {
		e := MappingError
		log.Error(e.errorMessage(err))
		requestInfo.Status = e.Status()
		_ = e.sendError(w)

		return
	}

	if err = version.FillEntryPointOutput(); err != nil {
		e := GetEntryPointOutputError
		log.Error(e.errorMessage(err))
		requestInfo.Status = e.Status()
		_ = e.sendError(w)

		return
	}

	reqParams := &requestStartParams{req.PipelineId, version, req.Keys, req.AttachmentFields, nil}

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
		log.Error(e.errorMessage(execErr))
		requestInfo.Status = e.Status()
		_ = e.sendError(w)

		return
	}

	if v == nil {
		e := PipelineExecutionError
		log.Error(e.errorMessage(errors.New("run_version_by_pipeline_id execution error")))
		requestInfo.Status = e.Status()
		_ = e.sendError(w)

		return
	}

	requestInfo.WorkNumber = v.WorkNumber
	requestInfo.Status = http.StatusOK

	if err = sendResponse(w, http.StatusOK, []*entity.RunResponse{v}); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		requestInfo.Status = e.Status()
		_ = e.sendError(w)

		return
	}
}

func (ae *APIEnv) handleStartApplicationParams(ctx c.Context, dto *requestStartParams) error {
	var schemaJson *jsonschema.Schema
	if unmarshalErr := json.Unmarshal(dto.version.Settings.StartSchemaRaw, &schemaJson); unmarshalErr != nil {
		return unmarshalErr
	}

	if schemaJson == nil {
		return errors.New("schema is empty")
	}

	if len(dto.keys) == 0 {
		res, _, getErr := schemaJson.GetAllFields()
		if getErr != nil {
			return getErr
		}

		dto.keys = res
	}

	if len(dto.attachmentFields) == 0 {
		dto.attachmentFields = schemaJson.GetAttachmentFields()
	}

	hiddenFields, err := ae.getHiddenFields(ctx, dto.version)
	if err != nil {
		e := GetHiddenFieldsError
		ae.Log.Error(e.errorMessage(err))
	}

	if len(hiddenFields) == 0 {
		hiddenFieldsSchema, getHiddenFieldsErr := schemaJson.GetHiddenFields()
		if getHiddenFieldsErr != nil {
			return err
		}

		dto.hiddenFields = hiddenFieldsSchema
	}

	dto.hiddenFields = hiddenFields

	return nil
}

func (ae *APIEnv) getHiddenFields(ctx c.Context, version *entity.EriusScenario) ([]string, error) {
	const sdBlockName = "servicedesk_application_0"
	hiddenFields := make([]string, 0)

	if _, exists := version.Pipeline.Blocks[sdBlockName]; !exists {
		return hiddenFields, errors.New("can`t find hidden fields, block is not found " + sdBlockName)
	}

	params := pipeline.ApplicationData{}
	errJson := json.Unmarshal(version.Pipeline.Blocks[sdBlockName].Params, &params)
	if errJson != nil {
		return hiddenFields, errJson
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
