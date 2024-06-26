package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/pkg/errors"

	"go.opencensus.io/trace"

	"golang.org/x/exp/slices"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/user"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

func (ae *Env) GetFormsChangelog(w http.ResponseWriter, r *http.Request, params GetFormsChangelogParams) {
	ctx, s := trace.StartSpan(r.Context(), "get_forms_changelog")
	defer s.End()

	log := logger.GetLogger(ctx)
	errorHandler := newHTTPErrorHandler(log, w)

	currentUI, err := user.GetEffectiveUserInfoFromCtx(ctx)
	if err != nil {
		errorHandler.handleError(NoUserInContextError, err)

		return
	}

	delegations, err := ae.HumanTasks.GetDelegationsToLogin(ctx, currentUI.Username)
	if err != nil {
		errorHandler.handleError(GetDelegationsError, err)

		return
	}

	delegationsByApprovement := delegations.FilterByType("approvement")
	delegationsByExecution := delegations.FilterByType("execution")

	dbTask, err := ae.DB.GetTask(ctx,
		delegationsByApprovement.GetUserInArrayWithDelegators([]string{currentUI.Username}),
		delegationsByExecution.GetUserInArrayWithDelegators([]string{currentUI.Username}),
		currentUI.Username,
		params.WorkNumber)
	if err != nil {
		errorHandler.handleError(GetTaskError, err)

		return
	}

	steps, err := ae.DB.GetTaskSteps(ctx, dbTask.ID)
	if err != nil {
		errorHandler.handleError(GetTaskError, err)

		return
	}

	fState := formState(steps, params)

	if fState == nil {
		errorHandler.handleError(GetFormsChangelogError, errors.New("no history for form node"))

		return
	}

	formData := pipeline.FormData{
		Executors:          make(map[string]struct{}, 0),
		InitialExecutors:   make(map[string]struct{}, 0),
		ApplicationBody:    make(map[string]interface{}, 0),
		Constants:          make(map[string]interface{}, 0),
		ChangesLog:         make([]pipeline.ChangesLogItem, 0),
		HiddenFields:       make([]string, 0),
		FormsAccessibility: make([]script.FormAccessibility, 0),
		Mapping:            make(map[string]script.JSONSchemaPropertiesValue, 0),
		AttachmentFields:   make([]string, 0),
		Keys:               make(map[string]string, 0),
	}

	err = json.Unmarshal(fState, &formData)
	if err != nil {
		errorHandler.handleError(GetFormsChangelogError, err)

		return
	}

	result := make([]FormChangelogItem, len(formData.ChangesLog))

	for i := range formData.ChangesLog {
		var (
			changelog       = formData.ChangesLog[i]
			createdAtString = changelog.CreatedAt.Format(time.RFC3339)
		)

		result[i] = FormChangelogItem{
			SchemaId:        &formData.SchemaID,
			CreatedAt:       &createdAtString,
			Description:     &changelog.Description,
			ApplicationBody: &changelog.ApplicationBody,
			Executor:        &changelog.Executor,
		}

		if !slices.Contains([]string{changelog.Executor}, currentUI.Username) &&
			currentUI.Username == dbTask.Author && formData.HideExecutorFromInitiator {
			result[i].Executor = utils.GetAddressOfValue(hiddenUserLogin)
		}
	}

	err = sendResponse(w, http.StatusOK, result)
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}

func formState(steps entity.TaskSteps, params GetFormsChangelogParams) json.RawMessage {
	for _, step := range steps {
		if step.Name == params.BlockId {
			return step.State[params.BlockId]
		}
	}

	return nil
}
