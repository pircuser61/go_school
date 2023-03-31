package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/google/uuid"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

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

	uuidVersion, parseErr := uuid.Parse(versionID)
	if parseErr != nil {
		e := UnknownError
		log.Error(e.errorMessage(parseErr))
		_ = e.sendError(w)

		return
	}

	pipeline, getPipelinerErr := ae.DB.GetPipeline(ctx, uuidVersion)
	if getPipelinerErr != nil {
		e := GetPipelineError
		log.Error(e.errorMessage(getPipelinerErr))
		_ = e.sendError(w)

		return
	}
	processSettings.Name = pipeline.Name

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
	for _, id := range externalSystemsIds {
		externalSystems = append(externalSystems, entity.ExternalSystem{
			Id:   id.String(),
			Name: systemsNames[id.String()],
		})
	}

	result := entity.ProcessSettingsWithExternalSystems{
		ExternalSystems: externalSystems,
		ProcessSettings: processSettings,
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

	var processSettings entity.ProcessSettings
	err = json.Unmarshal(b, &processSettings)
	if err != nil {
		e := ProcessSettingsParseError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	processSettings.Id = versionID

	transaction, startTransactionErr := ae.DB.StartTransaction(ctx)

	if startTransactionErr != nil {
		e := UnknownError
		log.Error(e.errorMessage(startTransactionErr))
		_ = e.sendError(w)

		return
	}
	defer func(transaction db.Database, ctx context.Context) {
		rollbackErr := transaction.RollbackTransaction(ctx)
		if rollbackErr != nil {
			e := UnknownError
			log.Error(e.errorMessage(rollbackErr))
			_ = e.sendError(w)

			return
		}
	}(transaction, ctx)

	saveVersionErr := transaction.SaveVersionSettings(ctx, processSettings, (*string)(params.SchemaFlag))
	if saveVersionErr != nil {
		e := ProcessSettingsSaveError
		log.Error(e.errorMessage(saveVersionErr))
		_ = e.sendError(w)

		return
	}

	canCreate, err := ae.DB.PipelineNameCreatable(ctx, processSettings.Name)
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
	if !canCreate {
		e := PipelineNameUsed
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	uuidVersion, parseErr := uuid.Parse(versionID)
	if parseErr != nil {
		e := UnknownError
		log.Error(e.errorMessage(parseErr))
		_ = e.sendError(w)

		return
	}

	saveNameErr := transaction.RenamePipeline(ctx, uuidVersion, processSettings.Name)
	if saveNameErr != nil {
		e := ProcessSettingsSaveError
		log.Error(e.errorMessage(saveNameErr))
		_ = e.sendError(w)

		return
	}

	_ = transaction.CommitTransaction(ctx)

	err = sendResponse(w, http.StatusOK, nil)
	if err != nil {
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

	err := ae.DB.RemoveExternalSystem(ctx, versionID, systemID)
	if err != nil {
		e := ExternalSystemRemoveError
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
