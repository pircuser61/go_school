package scheduler

import (
	c "context"

	scheduler_v1 "gitlab.services.mts.ru/jocasta/scheduler/pkg/proto/gen/src/task/v1"
)

func (s *Service) CreateTask(ctx c.Context, task *CreateTask) (id string, err error) {
	res, err := s.cli.CreateTask(ctx,
		&scheduler_v1.CreateTaskRequest{
			WorkNumber:  task.WorkNumber,
			TaskId:      task.WorkId,
			ActionName:  task.ActionName,
			WaitSeconds: int32(task.WaitSeconds),
		},
	)

	if err != nil {
		return id, err
	}

	return res.TaskId, nil
}
