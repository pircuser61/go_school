package api

import (
	c "context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/pkg/errors"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	e "gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/user"
)

func (ae *Env) toFlatProcessSettings(ctx c.Context, ps *e.ProcessSettings) error {
	if ps.StartSchemaRaw != nil {
		start, err := ae.Forms.MakeFlatSchema(ctx, ps.StartSchemaRaw)
		if err != nil {
			return errors.Wrap(err, "couldn't convert start schema")
		}

		ps.StartSchema = start
	}

	if ps.EndSchemaRaw != nil {
		end, err := ae.Forms.MakeFlatSchema(ctx, ps.EndSchemaRaw)
		if err != nil {
			return errors.Wrap(err, "couldn't convert end schema")
		}

		ps.EndSchema = end
	}

	return nil
}

func (ae *Env) SaveVersionTaskSubscriptionSettings(w http.ResponseWriter, req *http.Request, versionID string) {
	ctx, s := trace.StartSpan(req.Context(), "save_version_task_subscription_settings")
	defer s.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	b, err := io.ReadAll(req.Body)
	if err != nil {
		errorHandler.handleError(RequestReadError, err)

		return
	}

	defer req.Body.Close()

	var settings []*e.ExternalSystemSubscriptionParams

	err = json.Unmarshal(b, &settings)
	if err != nil {
		errorHandler.handleError(ExternalSystemSettingsParseError, err)

		return
	}

	// TODO: validation?

	txStorage, transactionErr := ae.DB.StartTransaction(ctx)
	if transactionErr != nil {
		log.WithError(transactionErr).Error("couldn't start transaction")

		errorHandler.sendError(UnknownError)

		return
	}

	defer func() {
		if r := recover(); r != nil {
			log = log.WithField("funcName", "SaveVersionTaskSubscriptionSettings").
				WithField("panic handle", true)
			log.Error(r)

			if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
				log.WithError(errors.New("couldn't rollback tx")).
					Error(txErr)
			}
		}
	}()

	defer func(transaction db.Database, ctx c.Context) {
		_ = transaction.RollbackTransaction(ctx)
	}(txStorage, ctx)

	if rmErr := ae.DB.RemoveExternalSystemTaskSubscriptions(ctx, versionID, ""); rmErr != nil {
		errorHandler.handleError(ExternalSystemSettingsSaveError, err)

		return
	}

	for _, s := range settings {
		err = ae.DB.SaveExternalSystemSubscriptionParams(ctx, versionID, s)
		if err != nil {
			errorHandler.handleError(ExternalSystemSettingsSaveError, err)

			return
		}
	}

	if err = txStorage.CommitTransaction(ctx); err != nil {
		log.WithError(err).Error("couldn't commit transaction")

		errorHandler.sendError(UnknownError)

		return
	}

	err = sendResponse(w, http.StatusOK, nil)
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

func (ae *Env) GetVersionSettings(w http.ResponseWriter, req *http.Request, versionID string) {
	ctx, s := trace.StartSpan(req.Context(), "get_version_settings")
	defer s.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	processSettings, err := ae.DB.GetVersionSettings(ctx, versionID)
	if err != nil {
		errorHandler.handleError(GetProcessSettingsError, err)

		return
	}

	slaSettings, err := ae.DB.GetSLAVersionSettings(ctx, versionID)
	if err != nil {
		errorHandler.handleError(GetProcessSLASettingsError, err)

		return
	}

	processSettings.SLA = slaSettings.SLA
	processSettings.WorkType = slaSettings.WorkType

	externalSystemsIds, err := ae.DB.GetExternalSystemsIDs(ctx, versionID)
	if err != nil {
		errorHandler.handleError(GetExternalSystemsError, err)

		return
	}

	systemsNames, err := ae.Integrations.GetSystemsNames(ctx, externalSystemsIds)
	if err != nil {
		errorHandler.handleError(GetExternalSystemsNamesError, err)

		return
	}

	externalSystems := make([]e.ExternalSystem, 0, len(externalSystemsIds))
	externalSystemsTaskSubs := make([]e.ExternalSystemSubscriptionParams, 0, len(externalSystemsIds))

	for _, id := range externalSystemsIds {
		externalSystemSettings, systemSettingsErr := ae.DB.GetExternalSystemSettings(ctx, versionID, id.String())
		if systemSettingsErr != nil {
			errorHandler.handleError(GetExternalSystemSettingsError, err)

			return
		}

		validateEndingSettings(&externalSystemSettings)

		externalSystems = append(
			externalSystems,
			e.ExternalSystem{
				ID:               id.String(),
				Name:             systemsNames[id.String()],
				AllowRunAsOthers: externalSystemSettings.AllowRunAsOthers,
				OutputSettings:   externalSystemSettings.OutputSettings,
			},
		)

		subscriptionSettings, taskSubscriptionsErr := ae.DB.GetExternalSystemTaskSubscriptions(ctx, versionID, id.String())
		if taskSubscriptionsErr != nil {
			errorHandler.handleError(GetExternalSystemSettingsError, taskSubscriptionsErr)

			return
		}

		if subscriptionSettings.SystemID != "" {
			externalSystemsTaskSubs = append(externalSystemsTaskSubs, subscriptionSettings)
		}
	}

	approvalLists, err := ae.DB.GetApprovalListsSettings(ctx, versionID)
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}

	result := e.ProcessSettingsWithExternalSystems{
		ExternalSystems:    externalSystems,
		ProcessSettings:    processSettings,
		TasksSubscriptions: externalSystemsTaskSubs,
		ApprovalLists:      approvalLists,
	}

	if err = sendResponse(w, http.StatusOK, result); err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

//nolint:dupl //its not duplicate
func (ae *Env) SaveVersionSettings(w http.ResponseWriter, req *http.Request, versionID string, params SaveVersionSettingsParams) {
	ctx, s := trace.StartSpan(req.Context(), "save_version_settings")
	defer s.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	var errCustom Err

	b, err := io.ReadAll(req.Body)
	defer req.Body.Close()

	if err != nil {
		errorHandler.handleError(RequestReadError, err)

		return
	}

	var processSettings *e.ProcessSettings

	if err = json.Unmarshal(b, &processSettings); err != nil {
		errorHandler.handleError(ProcessSettingsParseError, err)

		return
	}

	if convErr := ae.toFlatProcessSettings(ctx, processSettings); convErr != nil {
		errorHandler.handleError(ProcessSettingsConvertError, convErr)

		return
	}

	scenario, err := ae.DB.GetPipelineVersion(ctx, uuid.MustParse(versionID), true)
	if err != nil {
		errorHandler.handleError(GetVersionError, err)

		return
	}

	scenario.Settings = *processSettings

	err = processSettings.Validate()
	if err != nil {
		errorHandler.handleError(JSONSchemaValidationError, err)

		return
	}

	txStorage, transactionErr := ae.DB.StartTransaction(ctx)
	if transactionErr != nil {
		log.WithError(transactionErr).Error("couldn't set update version or settings")

		return
	}

	defer func() {
		if r := recover(); r != nil {
			log = log.WithField("funcName", "SaveVersionSettings").
				WithField("panic handle", true)
			log.Error(r)

			if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
				log.WithError(errors.New("couldn't rollback tx")).
					Error(txErr)
			}
		}
	}()

	processSettings.VersionID, errCustom, err = ae.createVersion(ctx, scenario)
	if err != nil {
		errorHandler.handleError(errCustom, err)

		if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
			log.WithField("funcName", "createVersion").
				WithError(errors.New("couldn't rollback tx")).
				Error(txErr)
		}

		return
	}

	err = ae.DB.SaveVersionSettings(ctx, *processSettings, (*string)(params.SchemaFlag))
	if err != nil {
		errorHandler.handleError(ProcessSettingsSaveError, err)

		if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
			log.WithField("funcName", "SaveVersionSettings").
				WithError(errors.New("couldn't rollback tx")).
				Error(txErr)
		}

		return
	}

	if commitErr := txStorage.CommitTransaction(ctx); commitErr != nil {
		log.WithError(commitErr).Error("couldn't update pipeline settings")

		if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
			log.Error(txErr)
		}

		errorHandler.handleError(UnknownError, commitErr)

		return
	}

	if err = sendResponse(w, http.StatusOK, processSettings); err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

func (ae *Env) createVersion(ctx c.Context, scenario *e.EriusScenario) (string, Err, error) {
	p, errCustom, err := ae.createPipelineVersion(ctx, scenario, scenario.PipelineID.String())
	if err != nil {
		return "", errCustom, err
	}

	return p.VersionID.String(), 0, nil
}

//nolint:dupl //its not duplicate
func (ae *Env) SaveExternalSystemSettings(
	w http.ResponseWriter, req *http.Request, versionID, systemID string, params SaveExternalSystemSettingsParams,
) {
	ctx, s := trace.StartSpan(req.Context(), "save_external_system_settings")
	defer s.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	b, err := io.ReadAll(req.Body)
	defer req.Body.Close()

	if err != nil {
		errorHandler.handleError(RequestReadError, err)

		return
	}

	var externalSystem e.ExternalSystem

	err = json.Unmarshal(b, &externalSystem)
	if err != nil {
		errorHandler.handleError(ExternalSystemSettingsParseError, err)

		return
	}

	externalSystem.ID = systemID

	err = externalSystem.ValidateSchemas()
	if err != nil {
		errorHandler.handleError(JSONSchemaValidationError, err)

		return
	}

	err = ae.DB.SaveExternalSystemSettings(ctx, versionID, externalSystem, (*string)(params.SchemaFlag))
	if err != nil {
		errorHandler.handleError(ExternalSystemSettingsSaveError, err)

		return
	}

	if err = sendResponse(w, http.StatusOK, nil); err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

func (ae *Env) RemoveExternalSystem(w http.ResponseWriter, req *http.Request, versionID, systemID string) {
	ctx, s := trace.StartSpan(req.Context(), "remove_external_system")
	defer s.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	txStorage, transactionErr := ae.DB.StartTransaction(ctx)
	if transactionErr != nil {
		log.WithError(transactionErr).Error("couldn't start transaction")
		errorHandler.sendError(UnknownError)

		return
	}

	defer func() {
		if r := recover(); r != nil {
			log = log.
				WithField("funcName", "RemoveExternalSystem").
				WithField("panic handle", true)

			log.Error(r)

			if txErr := txStorage.RollbackTransaction(ctx); txErr != nil {
				log.WithError(errors.New("couldn't rollback tx")).
					Error(txErr)
			}
		}
	}()

	defer func(transaction db.Database, ctx c.Context) {
		_ = transaction.RollbackTransaction(ctx)
	}(txStorage, ctx)

	err := txStorage.RemoveExternalSystemTaskSubscriptions(ctx, versionID, systemID)
	if err != nil {
		errorHandler.handleError(ExternalSystemRemoveError, err)

		return
	}

	err = txStorage.RemoveExternalSystem(ctx, versionID, systemID)
	if err != nil {
		errorHandler.handleError(ExternalSystemRemoveError, err)

		return
	}

	if err = txStorage.CommitTransaction(ctx); err != nil {
		log.WithError(err).Error("couldn't commit transaction")
		errorHandler.sendError(UnknownError)

		return
	}

	err = sendResponse(w, http.StatusOK, nil)
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

func (ae *Env) GetExternalSystemSettings(w http.ResponseWriter, req *http.Request, versionID, systemID string) {
	ctx, s := trace.StartSpan(req.Context(), "get_external_system_settings")
	defer s.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	externalSystemSettings, err := ae.DB.GetExternalSystemSettings(ctx, versionID, systemID)
	if err != nil {
		errorHandler.handleError(GetExternalSystemSettingsError, err)

		return
	}

	validateEndingSettings(&externalSystemSettings)

	if err := sendResponse(w, http.StatusOK, externalSystemSettings); err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

func (ae *Env) AddExternalSystemToVersion(w http.ResponseWriter, req *http.Request, versionID string) {
	ctx, s := trace.StartSpan(req.Context(), "add_external_system_to_version")
	defer s.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	b, err := io.ReadAll(req.Body)
	defer req.Body.Close()

	if err != nil {
		errorHandler.handleError(RequestReadError, err)

		return
	}

	var systemID ExternalSystemId

	err = json.Unmarshal(b, &systemID)
	if err != nil {
		errorHandler.handleError(ExternalSystemSettingsParseError, err)

		return
	}

	err = ae.DB.AddExternalSystemToVersion(ctx, versionID, string(systemID))
	if err != nil {
		errorHandler.handleError(ExternalSystemAddingError, err)

		return
	}

	err = sendResponse(w, http.StatusOK, nil)
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

func (ae *Env) SaveVersionMainSettings(w http.ResponseWriter, req *http.Request, versionID string) {
	ctx, s := trace.StartSpan(req.Context(), "save_version_main_settings")
	defer s.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	b, err := io.ReadAll(req.Body)
	if err != nil {
		errorHandler.handleError(RequestReadError, err)

		return
	}

	defer req.Body.Close()

	var processSettings e.ProcessSettings

	err = json.Unmarshal(b, &processSettings)
	if err != nil {
		errorHandler.handleError(ProcessSettingsParseError, err)

		return
	}

	processSettings.VersionID = versionID

	transaction, transactionCreateErr := ae.DB.StartTransaction(ctx)
	if transactionCreateErr != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}

	defer func() {
		if r := recover(); r != nil {
			log = log.
				WithField("funcName", "SaveVersionMainSettings").
				WithField("panic handle", true)

			log.Error(r)

			if txErr := transaction.RollbackTransaction(ctx); txErr != nil {
				log.WithError(errors.New("couldn't rollback tx")).
					Error(txErr)
			}
		}
	}()

	defer func(transaction db.Database, ctx c.Context) {
		_ = transaction.RollbackTransaction(ctx)
	}(transaction, ctx)

	saveVersionErr := transaction.SaveVersionMainSettings(ctx, processSettings)
	if saveVersionErr != nil {
		errorHandler.handleError(ProcessSettingsSaveError, saveVersionErr)

		return
	}

	isValid := processSettings.ValidateSLA()
	if !isValid {
		er := ValidationSLAProcessSettingsError

		log.Error(er.errorMessage(errors.New("Error while validating SlaSettings")))
		errorHandler.sendError(er)

		return
	}

	userFromContext, err := user.GetUserInfoFromCtx(ctx)
	if err != nil {
		errorHandler.handleError(GetUserinfoErr, err)

		return
	}

	saveVersionSLAErr := transaction.SaveSLAVersionSettings(ctx, versionID, e.SLAVersionSettings{
		Author:   userFromContext.Username,
		WorkType: processSettings.WorkType,
		SLA:      processSettings.SLA,
	})
	if saveVersionSLAErr != nil {
		errorHandler.handleError(ProcessSettingsSaveError, saveVersionSLAErr)

		return
	}

	parsedUUID, parseErr := uuid.Parse(versionID)
	if parseErr != nil {
		errorHandler.handleError(UnknownError, parseErr)

		return
	}

	pipeline, getPipelineErr := transaction.GetPipelineVersion(ctx, parsedUUID, true)
	if getPipelineErr != nil {
		errorHandler.handleError(UnknownError, getPipelineErr)

		return
	}

	renamePipelineErr := transaction.RenamePipeline(ctx, pipeline.PipelineID, processSettings.Name)
	if renamePipelineErr != nil {
		if db.IsUniqueConstraintError(renamePipelineErr) {
			errorHandler.handleError(PipelineNameUsed, renamePipelineErr)
		} else {
			errorHandler.handleError(PipelineCreateError, renamePipelineErr)
		}

		return
	}

	commitErr := transaction.CommitTransaction(ctx)
	if commitErr != nil {
		errorHandler.handleError(UnknownError, commitErr)

		return
	}

	err = sendResponse(w, http.StatusOK, nil)
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

func (ae *Env) SaveExternalSystemEndSettings(w http.ResponseWriter, r *http.Request, versionID, systemID string) {
	ctx, s := trace.StartSpan(r.Context(), "save_system_ending_settings")
	defer s.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	b, err := io.ReadAll(r.Body)
	if err != nil {
		errorHandler.handleError(RequestReadError, err)

		return
	}

	defer r.Body.Close()

	var systemSettings EndSystemSettings

	err = json.Unmarshal(b, &systemSettings)
	if err != nil {
		errorHandler.handleError(ProcessSettingsParseError, err)

		return
	}

	if systemSettings.Method == "" || systemSettings.URL == "" || systemSettings.MicroserviceId == "" {
		er := ValidationEndingSystemSettingsError

		log.Error(er.errorMessage(errors.New("Error while validating systemSettings")))
		errorHandler.sendError(er)

		return
	}

	err = ae.DB.UpdateEndingSystemSettings(
		ctx,
		versionID,
		systemID,
		e.EndSystemSettings{
			URL:            systemSettings.URL,
			Method:         string(systemSettings.Method),
			MicroserviceID: systemSettings.MicroserviceId,
		},
	)
	if err != nil {
		errorHandler.handleError(UpdateEndingSystemSettingsError, err)

		return
	}
}

func (ae *Env) DeleteExternalSystemEndSettings(w http.ResponseWriter, r *http.Request, versionID, systemID string) {
	ctx, s := trace.StartSpan(r.Context(), "delete_system_ending_settings")
	defer s.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	err := ae.DB.UpdateEndingSystemSettings(ctx, versionID, systemID, e.EndSystemSettings{})
	if err != nil {
		errorHandler.handleError(UpdateEndingSystemSettingsError, err)

		return
	}
}

func validateEndingSettings(s *e.ExternalSystem) {
	if s.OutputSettings.MicroserviceID == "" ||
		s.OutputSettings.URL == "" ||
		s.OutputSettings.Method == "" {
		s.OutputSettings = nil
	}
}

func (ae *Env) AllowRunAsOthers(w http.ResponseWriter, r *http.Request, versionID, systemID string) {
	ctx, s := trace.StartSpan(r.Context(), "allow_run_as_others")
	defer s.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	b, err := io.ReadAll(r.Body)
	if err != nil {
		errorHandler.handleError(RequestReadError, err)

		return
	}

	defer r.Body.Close()

	var allowRunAsOthers bool

	err = json.Unmarshal(b, &allowRunAsOthers)
	if err != nil {
		errorHandler.handleError(ProcessSettingsParseError, err)

		return
	}

	err = ae.DB.AllowRunAsOthers(ctx, versionID, systemID, allowRunAsOthers)
	if err != nil {
		errorHandler.handleError(UpdateRunAsOthersSettingsError, err)

		return
	}
}

func (ae *Env) RemoveApprovalListSettings(w http.ResponseWriter, r *http.Request, _, listID string) {
	ctx, s := trace.StartSpan(r.Context(), "remove_approval_list_settings")
	defer s.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	if err := ae.DB.RemoveApprovalListSettings(ctx, listID); err != nil {
		errorHandler.handleError(UpdateEndingSystemSettingsError, err)

		return
	}
}

func (ae *Env) UpdateApprovalListSettings(w http.ResponseWriter, r *http.Request, _, listID string) {
	ctx, s := trace.StartSpan(r.Context(), "update_approval_list_settings")
	defer s.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	b, err := io.ReadAll(r.Body)
	if err != nil {
		errorHandler.handleError(RequestReadError, err)

		return
	}

	defer r.Body.Close()

	var req e.UpdateApprovalListSettings
	if err = json.Unmarshal(b, &req); err != nil {
		errorHandler.handleError(ProcessSettingsParseError, err)

		return
	}

	req.ID = listID

	if err = ae.DB.UpdateApprovalListSettings(ctx, req); err != nil {
		errorHandler.handleError(UpdateEndingSystemSettingsError, err)

		return
	}

	if err = sendResponse(w, http.StatusOK, nil); err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

func (ae *Env) SaveApprovalListSettings(w http.ResponseWriter, r *http.Request, versionID string) {
	ctx, s := trace.StartSpan(r.Context(), "save_approval_list_settings")
	defer s.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	b, err := io.ReadAll(r.Body)
	if err != nil {
		errorHandler.handleError(RequestReadError, err)

		return
	}

	defer r.Body.Close()

	var req e.SaveApprovalListSettings
	if err = json.Unmarshal(b, &req); err != nil {
		errorHandler.handleError(ProcessSettingsParseError, err)

		return
	}

	id, err := ae.DB.SaveApprovalListSettings(ctx, e.SaveApprovalListSettings{
		VersionID:      versionID,
		Name:           req.Name,
		Steps:          req.Steps,
		ContextMapping: req.ContextMapping,
		FormsMapping:   req.FormsMapping,
	})
	if err != nil {
		errorHandler.handleError(UpdateEndingSystemSettingsError, err)

		return
	}

	if err = sendResponse(w, http.StatusOK, id); err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

func (ae *Env) GetApprovalListSetting(w http.ResponseWriter, r *http.Request, workNumber, listID string) {
	ctx, s := trace.StartSpan(r.Context(), "get_approval_list_settings")
	defer s.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	approvalList, err := ae.DB.GetApprovalListSettings(ctx, listID)
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}

	states, dates, err := ae.DB.GetFilteredStates(ctx, approvalList.Steps, workNumber)
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}

	varStore, err := ae.DB.GetVariableStorage(ctx, workNumber)
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}

	variables, err := varStore.GrabStorage()
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}

	variables = script.RestoreMapStructure(variables)

	res, err := toResponseApprovalListSettings(&toResponseApprovalListSettingsDTO{
		approvalList,
		states,
		variables,
		dates,
	})
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}

	if err = sendResponse(w, http.StatusOK, res); err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

type toResponseApprovalListSettingsDTO struct {
	approvalList *e.ApprovalListSettings
	stepsStates  map[string]map[string]interface{}
	variables    map[string]interface{}
	dates        map[string]map[string]*time.Time
}

func toResponseApprovalListSettings(dto *toResponseApprovalListSettingsDTO) (
	*ResponseVersionApprovalList, error,
) {
	steps := make([]TaskResponseStep, 0, len(dto.stepsStates))

	for i := range dto.stepsStates {
		stepName := i
		state := dto.stepsStates[stepName]
		stepType := "approver"
		errs := make([]string, 0)
		hasError := false
		storage := map[string]interface{}{}
		shortTitle := ""
		isDelegateOfAnyStepMember := false
		status := ""

		var (
			updatedAt *string
			createdAt *string
		)

		if ut, ok := dto.dates[stepName]["updatedAt"]; ok && ut != nil {
			utt := ut.Format(time.RFC3339)
			updatedAt = &utt
		}

		if ct, ok := dto.dates[stepName]["createdAt"]; ok && ct != nil {
			ctt := ct.Format(time.RFC3339)
			createdAt = &ctt
		}

		steps = append(steps, TaskResponseStep{
			Name:       &stepName,
			ShortTitle: &shortTitle,
			Type:       &stepType,
			State: &map[string]interface{}{
				stepName: state,
			},
			Errors:                    &errs,
			HasError:                  &hasError,
			Storage:                   &storage,
			IsDelegateOfAnyStepMember: &isDelegateOfAnyStepMember,
			Status:                    &status,
			Steps:                     &dto.approvalList.Steps,
			UpdateTime:                updatedAt,
			Time:                      createdAt,
		})
	}

	contextVariables, err := script.MapData(dto.approvalList.ContextMapping, dto.variables, nil)
	if err != nil {
		return nil, err
	}

	formsVariables, err := script.MapData(dto.approvalList.FormsMapping, dto.variables, nil)
	if err != nil {
		return nil, err
	}

	return &ResponseVersionApprovalList{
		Id:               dto.approvalList.ID,
		Name:             dto.approvalList.Name,
		Steps:            steps,
		ContextVariables: contextVariables,
		FormsVariables:   formsVariables,
	}, nil
}

func (ae *Env) GetApprovalListsSettings(w http.ResponseWriter, r *http.Request, versionID string) {
	ctx, s := trace.StartSpan(r.Context(), "get_approval_lists_settings")
	defer s.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	approvalLists, err := ae.DB.GetApprovalListsSettings(ctx, versionID)
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}

	if err = sendResponse(w, http.StatusOK, approvalLists); err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

//nolint:revive,stylecheck //need to implement interface in api.go
func (ae *Env) GetApprovalListSettingById(w http.ResponseWriter, r *http.Request, versionID, listID string) {
	ctx, s := trace.StartSpan(r.Context(), "get_approval_list_setting_by_id")
	defer s.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	approvalList, err := ae.DB.GetApprovalListSettings(ctx, listID)
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}

	if err = sendResponse(w, http.StatusOK, approvalList); err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}
