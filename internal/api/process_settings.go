package api

import (
	c "context"
	"encoding/json"

	"io"
	"net/http"

	"go.opencensus.io/trace"

	"github.com/google/uuid"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/user"
)

func (ae *APIEnv) convertProcessSettingsToFlat(ctx c.Context, ps *entity.ProcessSettings) error {
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
		e := RequestReadError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	defer req.Body.Close()

	var settings []*entity.ExternalSystemSubscriptionParams
	err = json.Unmarshal(b, &settings)
	if err != nil {
		e := ExternalSystemSettingsParseError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	// TODO: validation?

	txStorage, transactionErr := ae.DB.StartTransaction(ctx)
	if transactionErr != nil {
		log.WithError(transactionErr).Error("couldn't start transaction")
		e := UnknownError
		_ = e.sendError(w)
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
		e := ExternalSystemSettingsSaveError
		log.Error(e.errorMessage(rmErr))
		_ = e.sendError(w)

		return
	}

	for _, s := range settings {
		err = ae.DB.SaveExternalSystemSubscriptionParams(ctx, versionID, s)
		if err != nil {
			e := ExternalSystemSettingsSaveError
			log.Error(e.errorMessage(err))
			_ = e.sendError(w)

			return
		}
	}

	if err = txStorage.CommitTransaction(ctx); err != nil {
		log.WithError(err).Error("couldn't commit transaction")
		e := UnknownError
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

func (ae *APIEnv) GetVersionSettings(w http.ResponseWriter, req *http.Request, versionID string) {
	ctx, s := trace.StartSpan(req.Context(), "get_version_settings")
	defer s.End()

	log := logger.GetLogger(ctx)

	processSettings, err := ae.DB.GetVersionSettings(ctx, versionID)
	if err != nil {
		e := GetProcessSettingsError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	slaSettings, err := ae.DB.GetSlaVersionSettings(ctx, versionID)
	if err != nil {
		e := GetProcessSlaSettingsError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
	processSettings.SLA = slaSettings.Sla
	processSettings.WorkType = slaSettings.WorkType

	externalSystemsIds, err := ae.DB.GetExternalSystemsIDs(ctx, versionID)
	if err != nil {
		e := GetExternalSystemsError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	systemsNames, err := ae.Integrations.GetSystemsNames(ctx, externalSystemsIds)
	if err != nil {
		e := GetExternalSystemsNamesError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	externalSystems := make([]entity.ExternalSystem, 0, len(externalSystemsIds))
	externalSystemsTaskSubs := make([]entity.ExternalSystemSubscriptionParams, 0, len(externalSystemsIds))
	for _, id := range externalSystemsIds {
		externalSystemSettings, err := ae.DB.GetExternalSystemSettings(ctx, versionID, id.String())
		if err != nil {
			e := GetExternalSystemSettingsError
			log.Error(e.errorMessage(err))
			_ = e.sendError(w)

			return
		}
		validateEndingSettings(&externalSystemSettings)
		externalSystems = append(externalSystems, entity.ExternalSystem{
			Id:               id.String(),
			Name:             systemsNames[id.String()],
			AllowRunAsOthers: externalSystemSettings.AllowRunAsOthers,
			OutputSettings:   externalSystemSettings.OutputSettings,
		})

		subscriptionSettings, err := ae.DB.GetExternalSystemTaskSubscriptions(ctx, versionID, id.String())
		if err != nil {
			e := GetExternalSystemSettingsError
			log.Error(e.errorMessage(err))
			_ = e.sendError(w)

			return
		}
		if subscriptionSettings.SystemID != "" {
			externalSystemsTaskSubs = append(externalSystemsTaskSubs, subscriptionSettings)
		}
	}

	result := entity.ProcessSettingsWithExternalSystems{
		ExternalSystems:    externalSystems,
		ProcessSettings:    processSettings,
		TasksSubscriptions: externalSystemsTaskSubs,
	}

	if err := sendResponse(w, http.StatusOK, result); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

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
		e := RequestReadError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	var processSettings *entity.ProcessSettings
	err = json.Unmarshal(b, &processSettings)
	if err != nil {
		e := ProcessSettingsParseError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if convErr := ae.convertProcessSettingsToFlat(ctx, processSettings); convErr != nil {
		e := ProcessSettingsConvertError
		log.Error(e.errorMessage(convErr))
		_ = e.sendError(w)

		return
	}

	processSettings.Id = versionID

	err = processSettings.Validate()
	if err != nil {
		e := JSONSchemaValidationError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	saveVersionErr := ae.DB.SaveVersionSettings(ctx, *processSettings, (*string)(params.SchemaFlag))
	if saveVersionErr != nil {
		e := ProcessSettingsSaveError
		log.Error(e.errorMessage(saveVersionErr))
		_ = e.sendError(w)

		return
	}

	if err := sendResponse(w, http.StatusOK, processSettings); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

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
		e := RequestReadError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	var externalSystem entity.ExternalSystem
	err = json.Unmarshal(b, &externalSystem)
	if err != nil {
		e := ExternalSystemSettingsParseError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	externalSystem.Id = systemID

	err = externalSystem.ValidateSchemas()
	if err != nil {
		e := JSONSchemaValidationError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = ae.DB.SaveExternalSystemSettings(ctx, versionID, externalSystem, (*string)(params.SchemaFlag))
	if err != nil {
		e := ExternalSystemSettingsSaveError
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

func (ae *APIEnv) RemoveExternalSystem(w http.ResponseWriter, req *http.Request, versionID, systemID string) {
	ctx, s := trace.StartSpan(req.Context(), "remove_external_system")
	defer s.End()

	log := logger.GetLogger(ctx)

	txStorage, transactionErr := ae.DB.StartTransaction(ctx)
	if transactionErr != nil {
		log.WithError(transactionErr).Error("couldn't start transaction")
		e := UnknownError
		_ = e.sendError(w)
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
		e := ExternalSystemRemoveError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = txStorage.RemoveExternalSystem(ctx, versionID, systemID)
	if err != nil {
		e := ExternalSystemRemoveError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if err = txStorage.CommitTransaction(ctx); err != nil {
		log.WithError(err).Error("couldn't commit transaction")
		e := UnknownError
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

func (ae *APIEnv) GetExternalSystemSettings(w http.ResponseWriter, req *http.Request, versionID, systemID string) {
	ctx, s := trace.StartSpan(req.Context(), "get_external_system_settings")
	defer s.End()

	log := logger.GetLogger(ctx)

	externalSystemSettings, err := ae.DB.GetExternalSystemSettings(ctx, versionID, systemID)
	if err != nil {
		e := GetExternalSystemSettingsError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
	validateEndingSettings(&externalSystemSettings)

	if err := sendResponse(w, http.StatusOK, externalSystemSettings); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

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
		e := RequestReadError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	var systemID ExternalSystemId
	err = json.Unmarshal(b, &systemID)
	if err != nil {
		e := ExternalSystemSettingsParseError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = ae.DB.AddExternalSystemToVersion(ctx, versionID, string(systemID))
	if err != nil {
		e := ExternalSystemAddingError
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

func (ae *APIEnv) SaveVersionMainSettings(w http.ResponseWriter, req *http.Request, versionID string) {
	ctx, s := trace.StartSpan(req.Context(), "save_version_main_settings")
	defer s.End()

	log := logger.GetLogger(ctx)

	b, err := io.ReadAll(req.Body)

	if err != nil {
		e := RequestReadError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}
	defer req.Body.Close()

	var processSettings entity.ProcessSettings
	err = json.Unmarshal(b, &processSettings)
	if err != nil {
		e := ProcessSettingsParseError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	processSettings.Id = versionID

	transaction, transactionCreateErr := ae.DB.StartTransaction(ctx)
	if transactionCreateErr != nil {
		e := UnknownError
		log.Error(e.errorMessage(transactionCreateErr))
		_ = e.sendError(w)

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
		e := ProcessSettingsSaveError
		log.Error(e.errorMessage(saveVersionErr))
		_ = e.sendError(w)

		return
	}

	isValid := processSettings.ValidateSLA()
	if !isValid {
		e := ValidationSlaProcessSettingsError
		log.Error(e.errorMessage(errors.New("Error while validating SlaSettings")))
		_ = e.sendError(w)

		return
	}
	userFromContext, err := user.GetUserInfoFromCtx(ctx)
	if err != nil {
		e := GetUserinfoErr
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	saveVersionSLAErr := transaction.SaveSlaVersionSettings(ctx, versionID, entity.SlaVersionSettings{
		Author:   userFromContext.Username,
		WorkType: processSettings.WorkType,
		Sla:      processSettings.SLA,
	})
	if saveVersionSLAErr != nil {
		e := ProcessSettingsSaveError
		log.Error(e.errorMessage(saveVersionSLAErr))
		_ = e.sendError(w)

		return
	}

	parsedUUID, parseErr := uuid.Parse(versionID)
	if parseErr != nil {
		e := UnknownError
		log.Error(e.errorMessage(parseErr))
		_ = e.sendError(w)

		return
	}

	pipeline, getPipelineErr := transaction.GetPipelineVersion(ctx, parsedUUID, true)
	if getPipelineErr != nil {
		e := UnknownError
		log.Error(e.errorMessage(getPipelineErr))
		_ = e.sendError(w)

		return
	}

	renamePipelineErr := transaction.RenamePipeline(ctx, pipeline.ID, processSettings.Name)
	if renamePipelineErr != nil {
		e := PipelineCreateError
		if db.IsUniqueConstraintError(renamePipelineErr) {
			e = PipelineNameUsed
		}
		log.Error(e.errorMessage(renamePipelineErr))
		_ = e.sendError(w)

		return
	}

	commitErr := transaction.CommitTransaction(ctx)
	if commitErr != nil {
		e := UnknownError
		log.Error(e.errorMessage(commitErr))
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

func (ae *APIEnv) SaveExternalSystemEndSettings(w http.ResponseWriter, r *http.Request, versionID string, systemID string) {
	ctx, s := trace.StartSpan(r.Context(), "save_system_ending_settings")
	defer s.End()

	log := logger.GetLogger(ctx)

	b, err := io.ReadAll(r.Body)
	if err != nil {
		e := RequestReadError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}
	defer r.Body.Close()

	var systemSettings EndSystemSettings
	err = json.Unmarshal(b, &systemSettings)
	if err != nil {
		e := ProcessSettingsParseError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
	if systemSettings.Method == "" || systemSettings.URL == "" || systemSettings.MicroserviceId == "" {
		e := ValidationEndingSystemSettingsError
		log.Error(e.errorMessage(errors.New("Error while validating systemSettings")))
		_ = e.sendError(w)

		return
	}
	err = ae.DB.UpdateEndingSystemSettings(ctx, versionID, systemID, entity.EndSystemSettings{
		URL:            systemSettings.URL,
		Method:         string(systemSettings.Method),
		MicroserviceId: systemSettings.MicroserviceId,
	})
	if err != nil {
		e := UpdateEndingSystemSettingsError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

func (ae *APIEnv) DeleteExternalSystemEndSettings(w http.ResponseWriter, r *http.Request, versionID string, systemID string) {
	ctx, s := trace.StartSpan(r.Context(), "delete_system_ending_settings")
	defer s.End()

	log := logger.GetLogger(ctx)

	err := ae.DB.UpdateEndingSystemSettings(ctx, versionID, systemID, entity.EndSystemSettings{
		URL:            "",
		Method:         "",
		MicroserviceId: "",
	})
	if err != nil {
		e := UpdateEndingSystemSettingsError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

func validateEndingSettings(s *entity.ExternalSystem) {
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
		e := RequestReadError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}
	defer r.Body.Close()

	var allowRunAsOthers bool
	err = json.Unmarshal(b, &allowRunAsOthers)
	if err != nil {
		e := ProcessSettingsParseError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	err = ae.DB.AllowRunAsOthers(ctx, versionID, systemID, allowRunAsOthers)
	if err != nil {
		e := UpdateRunAsOthersSettingsError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}

func (ae *APIEnv) SaveApprovalListSettings(w http.ResponseWriter, r *http.Request, versionID string) {
	ctx, s := trace.StartSpan(r.Context(), "save_approval_list_settings")
	defer s.End()

	log := logger.GetLogger(ctx)

	b, err := io.ReadAll(r.Body)
	if err != nil {
		e := RequestReadError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}
	defer r.Body.Close()

	var systemSettings EndSystemSettings
	if err = json.Unmarshal(b, &systemSettings); err != nil {
		e := ProcessSettingsParseError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	id, err := ae.DB.SaveApprovalListSettings(ctx, entity.SaveApprovalListSettings{
		VersionId: versionID,
		Name:      "",
		Steps:     []string{},
	})
	if err != nil {
		e := UpdateEndingSystemSettingsError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	if err = sendResponse(w, http.StatusOK, id); err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}
