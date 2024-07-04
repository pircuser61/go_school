package scheduler

import (
	"context"
	"go.opencensus.io/trace"

	"github.com/google/uuid"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	scheduler_v1 "gitlab.services.mts.ru/jocasta/scheduler/pkg/proto/gen/src/task/v1"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

// TODO: Update service call to new version

func (s *Service) CreateTask(ctx context.Context, task *CreateTask) (id string, err error) {
	ctx, span := trace.StartSpan(ctx, "scheduler_create_task")
	defer span.End()

	traceID := span.SpanContext().TraceID.String()
	log := script.SetFieldsExternalCall(ctx, traceID, "v1", script.GRPC, script.GRPC, externalSystemName)

	ctx = logger.WithLogger(ctx, log)
	ctx = script.MakeContextWithRetryCnt(ctx)

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
		script.LogRetryFailure(ctx, s.maxRetryCount)

		return "", err
	}

	script.LogRetrySuccess(ctx)

	return res.TaskId, nil
}

func (s *Service) DeleteTask(ctx context.Context, task *DeleteTask) error {
	ctx, span := trace.StartSpan(ctx, "scheduler_delete_task")
	defer span.End()

	traceID := span.SpanContext().TraceID.String()
	log := script.SetFieldsExternalCall(ctx, traceID, "v1", script.GRPC, script.GRPC, externalSystemName)

	ctx = logger.WithLogger(ctx, log)
	ctx = script.MakeContextWithRetryCnt(ctx)

	_, err := s.cli.DeleteTask(ctx,
		&scheduler_v1.DeleteTaskRequest{
			WorkId:   task.WorkID,
			StepName: task.StepName,
		},
	)
	if err != nil {
		script.LogRetryFailure(ctx, s.maxRetryCount)

		return err
	}

	script.LogRetrySuccess(ctx)

	return nil
}

func (s *Service) DeleteAllTasksByWorkID(ctx context.Context, workID uuid.UUID) error {
	ctx, span := trace.StartSpan(ctx, "scheduler_delete_task_by_work_id")
	defer span.End()

	traceID := span.SpanContext().TraceID.String()
	log := script.SetFieldsExternalCall(ctx, traceID, "v1", script.GRPC, script.GRPC, externalSystemName)

	ctx = logger.WithLogger(ctx, log)
	ctx = script.MakeContextWithRetryCnt(ctx)

	_, err := s.cli.DeleteAllTasksByWorkID(ctx,
		&scheduler_v1.DeleteAllTasksByWorkIDRequest{
			WorkId: workID.String(),
		},
	)
	if err != nil {
		script.LogRetryFailure(ctx, s.maxRetryCount)

		return err
	}

	script.LogRetrySuccess(ctx)

	return nil
}
