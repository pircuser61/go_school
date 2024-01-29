package api

import (
	"context"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/pipeline"
)

func (ae *Env) processTasks(ctx context.Context, stoppedTasks []stoppedTask) error {
	for i := range stoppedTasks {
		task := stoppedTasks[i]
		runCtx := pipeline.BlockRunContext{
			WorkNumber: task.WorkNumber,
			TaskID:     task.ID,
			Services: pipeline.RunContextServices{
				HTTPClient:   ae.HTTPClient,
				Integrations: ae.Integrations,
				Storage:      ae.DB,
			},
			BlockRunResults: &pipeline.BlockRunResults{},
		}

		runCtx.SetTaskEvents(ctx)

		nodeEvents, eventErr := runCtx.GetCancelledStepsEvents(ctx)
		if eventErr != nil {
			return eventErr
		}

		runCtx.BlockRunResults.NodeEvents = nodeEvents
		runCtx.NotifyEvents(ctx)
	}

	return nil
}
