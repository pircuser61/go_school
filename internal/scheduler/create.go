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
	ctxLocal, span := trace.StartSpan(ctx, "scheduler_create_task")
	defer span.End()

	log := logger.GetLogger(ctxLocal).
		WithField("traceID", span.SpanContext().TraceID.String()).WithField("transport", "GRPC")

	ctxLocal = script.MakeContextWithRetryCnt(ctxLocal)

	res, err := s.cli.CreateTask(ctxLocal,
		&scheduler_v1.CreateTaskRequest{
			WorkNumber:  task.WorkNumber,
			TaskId:      task.WorkID,
			ActionName:  task.ActionName,
			StepName:    task.StepName,
			WaitSeconds: int32(task.WaitSeconds),
		},
	)

	attempt := script.GetRetryCnt(ctxLocal)

	if err != nil {
		log.Warning("Pipeliner failed to connect to scheduler. Exceeded max retry count: ", attempt)

		return "", err
	}

	if attempt > 0 {
		log.Warning("Pipeliner successfully reconnected to scheduler: ", attempt)
	}

	return res.TaskId, nil
}

func (s *Service) DeleteTask(ctx context.Context, task *DeleteTask) error {
	ctxLocal, span := trace.StartSpan(ctx, "scheduler_delete_task")
	defer span.End()

	log := logger.GetLogger(ctxLocal).
		WithField("traceID", span.SpanContext().TraceID.String()).WithField("transport", "GRPC")

	ctxLocal = script.MakeContextWithRetryCnt(ctxLocal)

	_, err := s.cli.DeleteTask(ctxLocal,
		&scheduler_v1.DeleteTaskRequest{
			WorkId:   task.WorkID,
			StepName: task.StepName,
		},
	)

	attempt := script.GetRetryCnt(ctxLocal)

	if err != nil {
		log.Warning("Pipeliner failed to connect to scheduler. Exceeded max retry count: ", attempt)

		return err
	}

	if attempt > 0 {
		log.Warning("Pipeliner successfully reconnected to scheduler: ", attempt)
	}

	return nil
}

func (s *Service) DeleteAllTasksByWorkID(ctx context.Context, workID uuid.UUID) error {
	ctxLocal, span := trace.StartSpan(ctx, "scheduler_delete_task_by_work_id")
	defer span.End()

	log := logger.GetLogger(ctxLocal).
		WithField("traceID", span.SpanContext().TraceID.String()).WithField("transport", "GRPC")

	ctxLocal = script.MakeContextWithRetryCnt(ctxLocal)

	_, err := s.cli.DeleteAllTasksByWorkID(ctxLocal,
		&scheduler_v1.DeleteAllTasksByWorkIDRequest{
			WorkId: workID.String(),
		},
	)

	attempt := script.GetRetryCnt(ctxLocal)

	if err != nil {
		log.Warning("Pipeliner failed to connect to scheduler. Exceeded max retry count: ", attempt)

		return err
	}

	if attempt > 0 {
		log.Warning("Pipeliner successfully reconnected to scheduler: ", attempt)
	}

	return nil
}
