package scheduler

import (
	"context"

	"github.com/google/uuid"
	scheduler_v1 "gitlab.services.mts.ru/jocasta/scheduler/pkg/proto/gen/src/task/v1"
	"go.opencensus.io/trace"
)

// TODO: Update service call to new version

func (s *Service) CreateTask(ctx context.Context, task *CreateTask) (id string, err error) {
	ctx, span := trace.StartSpan(ctx, "scheduler_create_task")
	defer span.End()

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

func (s *Service) DeleteTask(ctx context.Context, task *DeleteTask) error {
	ctx, span := trace.StartSpan(ctx, "scheduler_delete_task")
	defer span.End()

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

func (s *Service) DeleteAllTasksByWorkID(ctx context.Context, workID uuid.UUID) error {
	ctx, span := trace.StartSpan(ctx, "scheduler_delete_task_by_work_id")
	defer span.End()

	_, err := s.cli.DeleteAllTasksByWorkID(ctx,
		&scheduler_v1.DeleteAllTasksByWorkIDRequest{
			WorkId: workID.String(),
		},
	)
	if err != nil {
		return err
	}

	return nil
}
