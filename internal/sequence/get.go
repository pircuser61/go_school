package sequence

import (
	c "context"

	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	sequence "gitlab.services.mts.ru/jocasta/sequence/pkg/proto/gen/src/sequence/v1"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

func (s *service) GetWorkNumber(ctx c.Context) (workNumber string, err error) {
	ctx, span := trace.StartSpan(ctx, "sequence.get_work_number")
	defer span.End()

	traceID := span.SpanContext().TraceID.String()
	log := script.SetFieldsExternalCall(ctx, traceID, "v1", script.GRPC, script.GRPC, externalSystemName)

	ctx = logger.WithLogger(ctx, log)
	ctx = script.MakeContextWithRetryCnt(ctx)

	resp, err := s.cli.GetWorkNumber(ctx, &sequence.GetWorkNumberRequest{})
	if err != nil {
		script.LogRetryFailure(ctx, s.maxRetryCount)

		return "", err
	}

	script.LogRetrySuccess(ctx)

	return resp.WorkNumber, nil
}
