package api

import (
	c "context"
	"encoding/json"
	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/kafka"
)

func (ae *Env) RunTaskHandler(ctx c.Context, message kafka.RunnerInMessage) error {
	ctx, span := trace.StartSpan(ctx, "RunTaskHandler")
	defer span.End()

	log := ae.Log.WithField("funcName", "RunTaskHandler").
		WithField("stepID", message.TaskID).
		WithField("method", "kafka")

	ctx = logger.WithLogger(ctx, log)

	messageTmp, err := json.Marshal(message)
	if err != nil {
		log.WithError(err).
			Error("error marshaling message from kafka")
	}

	messageString := string(messageTmp)

	log.WithField("body", messageString).
		Info("start handle message from kafka")

	defer func() {
		if r := recover(); r != nil {
			log.WithField("funcName", "recover").
				Error(r)
		}
	}()

	log.WithField("funcName", "RunTaskHandler").
		WithField("body", messageString).
		Info("message from kafka successfully handled")

	return nil
}
