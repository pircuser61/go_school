package api

import (
	c "context"
	"errors"
	"fmt"
	"net/http"

	"gitlab.services.mts.ru/abp/myosotis/logger"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"

	"go.opencensus.io/trace"
)

func (ae *Env) SendEventsToKafka(w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.StartSpan(r.Context(), "send_events_to_kafka")
	defer span.End()

	log := script.SetMainFuncLog(ctx,
		"SendEventsToKafka",
		script.MethodGet,
		script.HTTP,
		span.SpanContext().TraceID.String(),
		"v1",
	)

	errorHandler := newHTTPErrorHandler(log, w)

	events, err := ae.DB.GetEventsToSend(ctx)
	if err != nil {
		log.Error(err)

		err = errors.New("couldn't get event to send")
		errorHandler.handleError(UnknownError, err)

		return
	}

	spCtx := span.SpanContext()

	// nolint // так надо и без этого нельзя
	routineCtx := c.WithValue(c.Background(), XRequestIDHeader, ctx.Value(XRequestIDHeader))

	routineCtx = logger.WithLogger(routineCtx, log)
	processCtx, fakeSpan := trace.StartSpanWithRemoteParent(routineCtx, "start_send_events_to_kafka", spCtx)
	fakeSpan.End()

	for i := range events {
		err = ae.Kafka.ProduceEventMessage(processCtx, &events[i].Event)
		if err != nil {
			log.WithError(err).Error(fmt.Sprintf("couldn't produce event message: %+v", events[i].Event))

			continue
		}

		err = ae.DB.DeleteEventToSend(processCtx, events[i].EventID)
		if err != nil {
			log.WithError(err).Error(fmt.Sprintf("couldn't update event: %+v", events[i].Event))

			continue
		}
	}

	err = sendResponse(w, http.StatusOK, nil)
	if err != nil {
		errorHandler.handleError(UnknownError, err)

		return
	}
}
