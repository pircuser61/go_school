package api

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"

	"go.opencensus.io/trace"
)

func (ae *APIEnv) GetFormsChangelog(w http.ResponseWriter, r *http.Request, params GetFormsChangelogParams) {
	ctx, s := trace.StartSpan(r.Context(), "update_task")
	defer s.End()

	log := logger.GetLogger(ctx)

	taskStep, err := ae.DB.GetTaskStepById(ctx, uuid.MustParse(params.BlockId))
	if err != nil {
		e := GetFormsChangelogError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	var stepName = taskStep.Name
	var formBlockState = taskStep.State[stepName]

	formData := pipeline.FormData{}
	err = json.Unmarshal(formBlockState, &formData)
	if err != nil {
		e := GetFormsChangelogError
		log.Error(e.errorMessage(err))
		_ = e.sendError(w)

		return
	}

	var result = make([]entity.FormChangelogEntry, 0)
	for _, changelog := range formData.ChangesLog {
		result = append(result, entity.FormChangelogEntry{
			CreatedAt:       changelog.CreatedAt,
			Description:     changelog.Description,
			ApplicationBody: changelog.ApplicationBody,
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
