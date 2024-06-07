package sequence

import (
	c "context"

	"go.opencensus.io/trace"

	sequence "gitlab.services.mts.ru/jocasta/sequence/pkg/proto/gen/src/sequence/v1"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

func (s *service) GetWorkNumber(ctx c.Context) (workNumber string, err error) {
	ctxLocal, span := trace.StartSpan(ctx, "sequence.get_work_number")
	defer span.End()

	log := logger.GetLogger(ctxLocal).
		WithField("traceID", span.SpanContext().TraceID.String()).WithField("transport", "GRPC")

	ctxLocal = script.MakeContextWithRetryCnt(ctxLocal)

	resp, err := s.cli.GetWorkNumber(ctxLocal, &sequence.GetWorkNumberRequest{})
	attempt := script.GetRetryCnt(ctxLocal)

	if err != nil {
		log.Warning("Pipeliner failed to connect to sequence. Exceeded max retry count: ", attempt)

		return "", err
	}

	if attempt > 0 {
		log.Warning("Pipeliner successfully reconnected to sequence: ", attempt)
	}

	return resp.WorkNumber, nil
}
