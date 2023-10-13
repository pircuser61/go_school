package scheduler

import (
	c "context"

	scheduler_v1 "gitlab.services.mts.ru/jocasta/scheduler/pkg/proto/gen/src/task/v1"
)

// TODO: Update service call to new version

func (s *Service) CreateTask(ctx c.Context, task *CreateTask) (id string, err error) {
	res, err := s.cli.CreateTask(ctx,
		&scheduler_v1.CreateTaskRequest{
			WorkNumber:  task.WorkNumber,
			TaskId:      task.WorkID,
			ActionName:  task.ActionName,
			StepName:    task.StepName,
			WaitSeconds: int32(task.WaitSeconds),
		},
	)

	if err != nil {
		return id, err
	}

	return res.TaskId, nil
}

func (s *Service) DeleteTask(ctx c.Context, task *DeleteTask) error {
	_, err := s.cli.DeleteTask(ctx,
		&scheduler_v1.DeleteTaskRequest{
			WorkId:   task.WorkID,
			StepName: task.StepName,
		},
	)

	if err != nil {
		return err
	}

	return nil
}
