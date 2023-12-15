package api

import (
	c "context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"go.opencensus.io/trace"

	"github.com/google/uuid"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	e "gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/user"
)

func (ae *APIEnv) convertProcessSettingsToFlat(ctx c.Context, ps *e.ProcessSettings) error {
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

func (ae *APIEnv) SaveVersionTaskSubscriptionSettings(w http.ResponseWriter, req *http.Request, versionID string) {
	ctx, s := trace.StartSpan(req.Context(), "save_version_task_subscription_settings")
	defer s.End()

	log := logger.GetLogger(ctx)

	b, err := io.ReadAll(req.Body)
	if err != nil {
		er := RequestReadError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}

	defer req.Body.Close()

	var settings []*e.ExternalSystemSubscriptionParams
	err = json.Unmarshal(b, &settings)
	if err != nil {
		er := ExternalSystemSettingsParseError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}

	// TODO: validation?

	txStorage, transactionErr := ae.DB.StartTransaction(ctx)
	if transactionErr != nil {
		log.WithError(transactionErr).Error("couldn't start transaction")
		er := UnknownError
		_ = er.sendError(w)
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
		er := ExternalSystemSettingsSaveError
		log.Error(er.errorMessage(rmErr))
		_ = er.sendError(w)

		return
	}

	for _, s := range settings {
		err = ae.DB.SaveExternalSystemSubscriptionParams(ctx, versionID, s)
		if err != nil {
			er := ExternalSystemSettingsSaveError
			log.Error(er.errorMessage(err))
			_ = er.sendError(w)

			return
		}
	}

	if err = txStorage.CommitTransaction(ctx); err != nil {
		log.WithError(err).Error("couldn't commit transaction")
		er := UnknownError
		_ = er.sendError(w)
		return
	}

	err = sendResponse(w, http.StatusOK, nil)
	if err != nil {
		er := UnknownError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}
}

func (ae *APIEnv) GetVersionSettings(w http.ResponseWriter, req *http.Request, versionID string) {
	ctx, s := trace.StartSpan(req.Context(), "get_version_settings")
	defer s.End()

	log := logger.GetLogger(ctx)

	processSettings, err := ae.DB.GetVersionSettings(ctx, versionID)
	if err != nil {
		er := GetProcessSettingsError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}

	slaSettings, err := ae.DB.GetSlaVersionSettings(ctx, versionID)
	if err != nil {
		er := GetProcessSlaSettingsError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}
	processSettings.SLA = slaSettings.Sla
	processSettings.WorkType = slaSettings.WorkType

	externalSystemsIds, err := ae.DB.GetExternalSystemsIDs(ctx, versionID)
	if err != nil {
		er := GetExternalSystemsError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}

	systemsNames, err := ae.Integrations.GetSystemsNames(ctx, externalSystemsIds)
	if err != nil {
		er := GetExternalSystemsNamesError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}

	externalSystems := make([]e.ExternalSystem, 0, len(externalSystemsIds))
	externalSystemsTaskSubs := make([]e.ExternalSystemSubscriptionParams, 0, len(externalSystemsIds))
	for _, id := range externalSystemsIds {
		externalSystemSettings, err := ae.DB.GetExternalSystemSettings(ctx, versionID, id.String())
		if err != nil {
			er := GetExternalSystemSettingsError
			log.Error(er.errorMessage(err))
			_ = er.sendError(w)

			return
		}
		validateEndingSettings(&externalSystemSettings)
		externalSystems = append(externalSystems, e.ExternalSystem{
			Id:               id.String(),
			Name:             systemsNames[id.String()],
			AllowRunAsOthers: externalSystemSettings.AllowRunAsOthers,
			OutputSettings:   externalSystemSettings.OutputSettings,
		})

		subscriptionSettings, err := ae.DB.GetExternalSystemTaskSubscriptions(ctx, versionID, id.String())
		if err != nil {
			er := GetExternalSystemSettingsError
			log.Error(er.errorMessage(err))
			_ = er.sendError(w)

			return
		}
		if subscriptionSettings.SystemID != "" {
			externalSystemsTaskSubs = append(externalSystemsTaskSubs, subscriptionSettings)
		}
	}

	approvalLists, err := ae.DB.GetApprovalListsSettings(ctx, versionID)
	if err != nil {
		er := UnknownError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}

	result := e.ProcessSettingsWithExternalSystems{
		ExternalSystems:    externalSystems,
		ProcessSettings:    processSettings,
		TasksSubscriptions: externalSystemsTaskSubs,
		ApprovalLists:      approvalLists,
	}

	if err = sendResponse(w, http.StatusOK, result); err != nil {
		er := UnknownError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}
}

//nolint:dupl //its not duplicate
func (ae *APIEnv) SaveVersionSettings(w http.ResponseWriter, req *http.Request, versionID string, params SaveVersionSettingsParams) {
	ctx, s := trace.StartSpan(req.Context(), "save_version_settings")
	defer s.End()

	log := logger.GetLogger(ctx)

	b, err := io.ReadAll(req.Body)
	defer req.Body.Close()

	if err != nil {
		er := RequestReadError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}

	var processSettings *e.ProcessSettings
	if err = json.Unmarshal(b, &processSettings); err != nil {
		er := ProcessSettingsParseError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}

	if convErr := ae.convertProcessSettingsToFlat(ctx, processSettings); convErr != nil {
		er := ProcessSettingsConvertError
		log.Error(er.errorMessage(convErr))
		_ = er.sendError(w)

		return
	}

	processSettings.Id = versionID
	if err = processSettings.Validate(); err != nil {
		er := JSONSchemaValidationError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}

	saveVersionErr := ae.DB.SaveVersionSettings(ctx, *processSettings, (*string)(params.SchemaFlag))
	if saveVersionErr != nil {
		er := ProcessSettingsSaveError
		log.Error(er.errorMessage(saveVersionErr))
		_ = er.sendError(w)

		return
	}

	if err = sendResponse(w, http.StatusOK, processSettings); err != nil {
		er := UnknownError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}
}

//nolint:dupl //its not duplicate
func (ae *APIEnv) SaveExternalSystemSettings(
	w http.ResponseWriter, req *http.Request, versionID, systemID string, params SaveExternalSystemSettingsParams) {
	ctx, s := trace.StartSpan(req.Context(), "save_external_system_settings")
	defer s.End()

	log := logger.GetLogger(ctx)

	b, err := io.ReadAll(req.Body)
	defer req.Body.Close()

	if err != nil {
		er := RequestReadError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}

	var externalSystem e.ExternalSystem
	if err = json.Unmarshal(b, &externalSystem); err != nil {
		er := ExternalSystemSettingsParseError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}

	externalSystem.Id = systemID

	if err = externalSystem.ValidateSchemas(); err != nil {
		er := JSONSchemaValidationError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}

	err = ae.DB.SaveExternalSystemSettings(ctx, versionID, externalSystem, (*string)(params.SchemaFlag))
	if err != nil {
		er := ExternalSystemSettingsSaveError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}

	if err = sendResponse(w, http.StatusOK, nil); err != nil {
		er := UnknownError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}
}

func (ae *APIEnv) RemoveExternalSystem(w http.ResponseWriter, req *http.Request, versionID, systemID string) {
	ctx, s := trace.StartSpan(req.Context(), "remove_external_system")
	defer s.End()

	log := logger.GetLogger(ctx)

	txStorage, transactionErr := ae.DB.StartTransaction(ctx)
	if transactionErr != nil {
		log.WithError(transactionErr).Error("couldn't start transaction")
		er := UnknownError
		_ = er.sendError(w)
		return
	}
	defer func() {
		if r := recover(); r != nil {
			log = log.WithField("funcName", "RemoveExternalSystem").
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
		er := ExternalSystemRemoveError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}

	err = txStorage.RemoveExternalSystem(ctx, versionID, systemID)
	if err != nil {
		er := ExternalSystemRemoveError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}

	if err = txStorage.CommitTransaction(ctx); err != nil {
		log.WithError(err).Error("couldn't commit transaction")
		er := UnknownError
		_ = er.sendError(w)
		return
	}

	err = sendResponse(w, http.StatusOK, nil)
	if err != nil {
		er := UnknownError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}
}

func (ae *APIEnv) GetExternalSystemSettings(w http.ResponseWriter, req *http.Request, versionID, systemID string) {
	ctx, s := trace.StartSpan(req.Context(), "get_external_system_settings")
	defer s.End()

	log := logger.GetLogger(ctx)

	externalSystemSettings, err := ae.DB.GetExternalSystemSettings(ctx, versionID, systemID)
	if err != nil {
		er := GetExternalSystemSettingsError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}
	validateEndingSettings(&externalSystemSettings)

	if err := sendResponse(w, http.StatusOK, externalSystemSettings); err != nil {
		er := UnknownError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}
}

func (ae *APIEnv) AddExternalSystemToVersion(w http.ResponseWriter, req *http.Request, versionID string) {
	ctx, s := trace.StartSpan(req.Context(), "add_external_system_to_version")
	defer s.End()

	log := logger.GetLogger(ctx)

	b, err := io.ReadAll(req.Body)
	defer req.Body.Close()

	if err != nil {
		er := RequestReadError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}

	var systemID ExternalSystemId
	err = json.Unmarshal(b, &systemID)
	if err != nil {
		er := ExternalSystemSettingsParseError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}

	err = ae.DB.AddExternalSystemToVersion(ctx, versionID, string(systemID))
	if err != nil {
		er := ExternalSystemAddingError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}

	err = sendResponse(w, http.StatusOK, nil)
	if err != nil {
		er := UnknownError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}
}

func (ae *APIEnv) SaveVersionMainSettings(w http.ResponseWriter, req *http.Request, versionID string) {
	ctx, s := trace.StartSpan(req.Context(), "save_version_main_settings")
	defer s.End()

	log := logger.GetLogger(ctx)

	b, err := io.ReadAll(req.Body)

	if err != nil {
		er := RequestReadError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)
		return
	}
	defer req.Body.Close()

	var processSettings e.ProcessSettings
	err = json.Unmarshal(b, &processSettings)
	if err != nil {
		er := ProcessSettingsParseError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}

	processSettings.Id = versionID

	transaction, transactionCreateErr := ae.DB.StartTransaction(ctx)
	if transactionCreateErr != nil {
		er := UnknownError
		log.Error(er.errorMessage(transactionCreateErr))
		_ = er.sendError(w)

		return
	}

	defer func() {
		if r := recover(); r != nil {
			log = log.WithField("funcName", "SaveVersionMainSettings").
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
		er := ProcessSettingsSaveError
		log.Error(er.errorMessage(saveVersionErr))
		_ = er.sendError(w)

		return
	}

	isValid := processSettings.ValidateSLA()
	if !isValid {
		er := ValidationSlaProcessSettingsError
		log.Error(er.errorMessage(errors.New("Error while validating SlaSettings")))
		_ = er.sendError(w)

		return
	}
	userFromContext, err := user.GetUserInfoFromCtx(ctx)
	if err != nil {
		er := GetUserinfoErr
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}

	saveVersionSLAErr := transaction.SaveSlaVersionSettings(ctx, versionID, e.SlaVersionSettings{
		Author:   userFromContext.Username,
		WorkType: processSettings.WorkType,
		Sla:      processSettings.SLA,
	})
	if saveVersionSLAErr != nil {
		er := ProcessSettingsSaveError
		log.Error(er.errorMessage(saveVersionSLAErr))
		_ = er.sendError(w)

		return
	}

	parsedUUID, parseErr := uuid.Parse(versionID)
	if parseErr != nil {
		er := UnknownError
		log.Error(er.errorMessage(parseErr))
		_ = er.sendError(w)

		return
	}

	pipeline, getPipelineErr := transaction.GetPipelineVersion(ctx, parsedUUID, true)
	if getPipelineErr != nil {
		er := UnknownError
		log.Error(er.errorMessage(getPipelineErr))
		_ = er.sendError(w)

		return
	}

	renamePipelineErr := transaction.RenamePipeline(ctx, pipeline.ID, processSettings.Name)
	if renamePipelineErr != nil {
		er := PipelineCreateError
		if db.IsUniqueConstraintError(renamePipelineErr) {
			er = PipelineNameUsed
		}
		log.Error(er.errorMessage(renamePipelineErr))
		_ = er.sendError(w)

		return
	}

	commitErr := transaction.CommitTransaction(ctx)
	if commitErr != nil {
		er := UnknownError
		log.Error(er.errorMessage(commitErr))
		_ = er.sendError(w)

		return
	}
	err = sendResponse(w, http.StatusOK, nil)
	if err != nil {
		er := UnknownError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}
}

func (ae *APIEnv) SaveExternalSystemEndSettings(w http.ResponseWriter, r *http.Request, versionID, systemID string) {
	ctx, s := trace.StartSpan(r.Context(), "save_system_ending_settings")
	defer s.End()

	log := logger.GetLogger(ctx)

	b, err := io.ReadAll(r.Body)
	if err != nil {
		er := RequestReadError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)
		return
	}
	defer r.Body.Close()

	var systemSettings EndSystemSettings
	err = json.Unmarshal(b, &systemSettings)
	if err != nil {
		er := ProcessSettingsParseError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}
	if systemSettings.Method == "" || systemSettings.URL == "" || systemSettings.MicroserviceId == "" {
		er := ValidationEndingSystemSettingsError
		log.Error(er.errorMessage(errors.New("Error while validating systemSettings")))
		_ = er.sendError(w)

		return
	}
	err = ae.DB.UpdateEndingSystemSettings(ctx, versionID, systemID, e.EndSystemSettings{
		URL:            systemSettings.URL,
		Method:         string(systemSettings.Method),
		MicroserviceId: systemSettings.MicroserviceId,
	})
	if err != nil {
		er := UpdateEndingSystemSettingsError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}
}

func (ae *APIEnv) DeleteExternalSystemEndSettings(w http.ResponseWriter, r *http.Request, versionID, systemID string) {
	ctx, s := trace.StartSpan(r.Context(), "delete_system_ending_settings")
	defer s.End()

	log := logger.GetLogger(ctx)

	err := ae.DB.UpdateEndingSystemSettings(ctx, versionID, systemID, e.EndSystemSettings{
		URL:            "",
		Method:         "",
		MicroserviceId: "",
	})
	if err != nil {
		er := UpdateEndingSystemSettingsError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}
}

func validateEndingSettings(s *e.ExternalSystem) {
	if s.OutputSettings.MicroserviceId == "" ||
		s.OutputSettings.URL == "" ||
		s.OutputSettings.Method == "" {
		s.OutputSettings = nil
	}
}

func (ae *APIEnv) AllowRunAsOthers(w http.ResponseWriter, r *http.Request, versionID, systemID string) {
	ctx, s := trace.StartSpan(r.Context(), "allow_run_as_others")
	defer s.End()

	log := logger.GetLogger(ctx)

	b, err := io.ReadAll(r.Body)
	if err != nil {
		er := RequestReadError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)
		return
	}
	defer r.Body.Close()

	var allowRunAsOthers bool
	err = json.Unmarshal(b, &allowRunAsOthers)
	if err != nil {
		er := ProcessSettingsParseError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}

	err = ae.DB.AllowRunAsOthers(ctx, versionID, systemID, allowRunAsOthers)
	if err != nil {
		er := UpdateRunAsOthersSettingsError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}
}

func (ae *APIEnv) RemoveApprovalListSettings(w http.ResponseWriter, r *http.Request, versionID, listID string) {
	ctx, s := trace.StartSpan(r.Context(), "remove_approval_list_settings")
	defer s.End()

	log := logger.GetLogger(ctx)

	if err := ae.DB.RemoveApprovalListSettings(ctx, listID); err != nil {
		er := UpdateEndingSystemSettingsError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}
}

func (ae *APIEnv) UpdateApprovalListSettings(w http.ResponseWriter, r *http.Request, versionID, listID string) {
	ctx, s := trace.StartSpan(r.Context(), "update_approval_list_settings")
	defer s.End()

	log := logger.GetLogger(ctx)

	b, err := io.ReadAll(r.Body)
	if err != nil {
		er := RequestReadError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)
		return
	}
	defer r.Body.Close()

	var req e.UpdateApprovalListSettings
	if err = json.Unmarshal(b, &req); err != nil {
		er := ProcessSettingsParseError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}

	req.ID = listID

	if err = ae.DB.UpdateApprovalListSettings(ctx, req); err != nil {
		er := UpdateEndingSystemSettingsError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}

	if err = sendResponse(w, http.StatusOK, nil); err != nil {
		er := UnknownError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}
}

func (ae *APIEnv) SaveApprovalListSettings(w http.ResponseWriter, r *http.Request, versionID string) {
	ctx, s := trace.StartSpan(r.Context(), "save_approval_list_settings")
	defer s.End()

	log := logger.GetLogger(ctx)

	b, err := io.ReadAll(r.Body)
	if err != nil {
		er := RequestReadError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)
		return
	}
	defer r.Body.Close()

	var req e.SaveApprovalListSettings
	if err = json.Unmarshal(b, &req); err != nil {
		er := ProcessSettingsParseError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}

	id, err := ae.DB.SaveApprovalListSettings(ctx, e.SaveApprovalListSettings{
		VersionId:      versionID,
		Name:           req.Name,
		Steps:          req.Steps,
		ContextMapping: req.ContextMapping,
		FormsMapping:   req.FormsMapping,
	})
	if err != nil {
		er := UpdateEndingSystemSettingsError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}

	if err = sendResponse(w, http.StatusOK, id); err != nil {
		er := UnknownError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}
}

func (ae *APIEnv) GetApprovalListSetting(w http.ResponseWriter, r *http.Request, workNumber, listID string) {
	ctx, s := trace.StartSpan(r.Context(), "get_approval_list_settings")
	defer s.End()

	log := logger.GetLogger(ctx)

	approvalList, err := ae.DB.GetApprovalListSettings(ctx, listID)
	if err != nil {
		er := UnknownError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}

	states, err := ae.DB.GetFilteredStates(ctx, approvalList.Steps, workNumber)
	if err != nil {
		er := UnknownError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}

	varStore, err := ae.DB.GetVariableStorage(ctx, workNumber)
	if err != nil {
		er := UnknownError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}

	variables, err := varStore.GrabStorage()
	if err != nil {
		er := UnknownError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}

	variables = script.RestoreMapStructure(variables)

	res, err := toResponseApprovalListSettings(&toResponseApprovalListSettingsDTO{
		approvalList,
		states,
		variables,
	})
	if err != nil {
		er := UnknownError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}

	if err = sendResponse(w, http.StatusOK, res); err != nil {
		er := UnknownError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}
}

type toResponseApprovalListSettingsDTO struct {
	approvalList *e.ApprovalListSettings
	stepsStates  map[string]map[string]interface{}
	variables    map[string]interface{}
}

func toResponseApprovalListSettings(dto *toResponseApprovalListSettingsDTO) (
	*ResponseVersionApprovalList, error) {
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
		updateTime := time.Now().String()
		tisulka := time.Now().String()

		steps = append(steps, TaskResponseStep{
			Name:                      &stepName,
			ShortTitle:                &shortTitle,
			Type:                      &stepType,
			State:                     &state,
			Errors:                    &errs,
			HasError:                  &hasError,
			Storage:                   &storage,
			IsDelegateOfAnyStepMember: &isDelegateOfAnyStepMember,
			Status:                    &status,
			Steps:                     &dto.approvalList.Steps,
			UpdateTime:                &updateTime,
			Time:                      &tisulka,
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

func (ae *APIEnv) GetApprovalListsSettings(w http.ResponseWriter, r *http.Request, versionID string) {
	ctx, s := trace.StartSpan(r.Context(), "get_approval_lists_settings")
	defer s.End()

	log := logger.GetLogger(ctx)

	approvalLists, err := ae.DB.GetApprovalListsSettings(ctx, versionID)
	if err != nil {
		er := UnknownError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}

	if err = sendResponse(w, http.StatusOK, approvalLists); err != nil {
		er := UnknownError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}
}

func (ae *APIEnv) GetApprovalListSettingById(w http.ResponseWriter, r *http.Request, versionID, listID string) {
	ctx, s := trace.StartSpan(r.Context(), "get_approval_list_setting_by_id")
	defer s.End()

	log := logger.GetLogger(ctx)

	approvalList, err := ae.DB.GetApprovalListSettings(ctx, listID)
	if err != nil {
		er := UnknownError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}

	if err = sendResponse(w, http.StatusOK, approvalList); err != nil {
		er := UnknownError
		log.Error(er.errorMessage(err))
		_ = er.sendError(w)

		return
	}
}
