package api

import (
	"encoding/json"
	"net/http"
	"time"

	"golang.org/x/exp/slices"

	"go.opencensus.io/trace"

	"github.com/pkg/errors"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/user"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
)

func (ae *APIEnv) GetFormsChangelog(w http.ResponseWriter, r *http.Request, params GetFormsChangelogParams) {
	ctx, s := trace.StartSpan(r.Context(), "get_forms_changelog")
	defer s.End()

	log := logger.GetLogger(ctx)

	currentUi, err := user.GetEffectiveUserInfoFromCtx(ctx)
	if err != nil {
		e := NoUserInContextError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}

	delegations, err := ae.HumanTasks.GetDelegationsToLogin(ctx, currentUi.Username)
	if err != nil {
		e := GetDelegationsError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)
		return
	}

	delegationsByApprovement := delegations.FilterByType("approvement")
	delegationsByExecution := delegations.FilterByType("execution")

	dbTask, err := ae.DB.GetTask(ctx,
		delegationsByApprovement.GetUserInArrayWithDelegators([]string{currentUi.Username}),
		delegationsByExecution.GetUserInArrayWithDelegators([]string{currentUi.Username}),
		currentUi.Username,
		params.WorkNumber)
	if err != nil {
		e := GetTaskError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	steps, err := ae.DB.GetTaskSteps(ctx, dbTask.ID)
	if err != nil {
		e := GetTaskError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	var formState json.RawMessage
	for _, step := range steps {
		if step.Name == params.BlockId {
			formState = step.State[params.BlockId]
		}
	}

	if formState == nil {
		e := GetFormsChangelogError
		log.Error(e.errorMessage(errors.New("no history for form node")))
		_ = e.sendError(w)

		return
	}

	formData := pipeline.FormData{}
	err = json.Unmarshal(formState, &formData)
	if err != nil {
		e := GetFormsChangelogError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	var result = make([]FormChangelogItem, len(formData.ChangesLog))
	for i := range formData.ChangesLog {
		var changelog = formData.ChangesLog[i]
		var createdAtString = changelog.CreatedAt.Format(time.RFC3339)
		result[i] = FormChangelogItem{
			SchemaId:        &formData.SchemaId,
			CreatedAt:       &createdAtString,
			Description:     &changelog.Description,
			ApplicationBody: &changelog.ApplicationBody,
			Executor:        &changelog.Executor,
		}

		if !slices.Contains([]string{changelog.Executor}, currentUi.Username) &&
			currentUi.Username == dbTask.Author && formData.HideExecutorFromInitiator {
			result[i].Executor = utils.GetAddressOfValue(hiddenUserLogin)
		}
	}

	err = sendResponse(w, http.StatusOK, result)
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}
