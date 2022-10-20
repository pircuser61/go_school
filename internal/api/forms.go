package api

import (
	"encoding/json"
	"net/http"

	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"

	"go.opencensus.io/trace"
)

func (ae *APIEnv) GetFormsChangelog(w http.ResponseWriter, r *http.Request, params GetFormsChangelogParams) {
	ctx, s := trace.StartSpan(r.Context(), "get_forms_changelog")
	defer s.End()

	log := logger.GetLogger(ctx)

	dbTask, err := ae.DB.GetTask(ctx, params.WorkNumber)
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
		log.Error(e.errorMessage(err))
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
	for _, changelog := range formData.ChangesLog {
		var createdAtString = changelog.CreatedAt.String()
		result = append(result, FormChangelogItem{
			CreatedAt:       &createdAtString,
			Description:     &changelog.Description,
			ApplicationBody: &changelog.ApplicationBody,
		})
	}

	err = sendResponse(w, http.StatusOK, result)
	if err != nil {
		e := UnknownError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}
}
