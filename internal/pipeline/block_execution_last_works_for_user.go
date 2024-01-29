package pipeline

import (
	"context"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

func (gb *GoExecutionBlock) lastWorksForUser(
	ctx context.Context,
	processSettings *entity.ProcessSettings,
	login string,
	task *entity.EriusScenario,
) ([]*entity.EriusTask, error) {
	if processSettings.ResubmissionPeriod > 0 {
		lastWorksForUser, getWorksErr := gb.RunContext.Services.Storage.GetWorksForUserWithGivenTimeRange(
			ctx,
			processSettings.ResubmissionPeriod,
			login,
			task.VersionID.String(),
			gb.RunContext.WorkNumber,
		)
		if getWorksErr != nil {
			return make([]*entity.EriusTask, 0), getWorksErr
		}

		return lastWorksForUser, nil
	}

	return make([]*entity.EriusTask, 0), nil
}
