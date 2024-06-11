package script

import (
	"context"
	"net/http"

	"gitlab.services.mts.ru/abp/myosotis/logger"
)

type (
	retryCnt         struct{}
	restRetryStarted struct{}
)

func LogRetryFailure(ctx context.Context, maxCount uint) {
	log := logger.GetLogger(ctx)
	attempt := getRetryCnt(ctx)

	log.WithField("attempt", attempt).
		Warning("connect, exceeded max retry count: %d", maxCount)
}

func LogRetrySuccess(ctx context.Context) {
	attempt := getRetryCnt(ctx)

	if attempt > 0 {
		logger.
			GetLogger(ctx).
			WithField("attempt", attempt).
			Info("pipeliner successfully reconnected")
	}
}

func MakeContextWithRetryCnt(ctx context.Context) context.Context {
	count := uint(0)
	retryStarted := false

	ctx = context.WithValue(ctx, retryCnt{}, &count)
	ctx = context.WithValue(ctx, restRetryStarted{}, &retryStarted)

	return ctx
}

func IncreaseReqRetryCntREST(req *http.Request) {
	ctx := req.Context()

	retryStarted, _ := ctx.Value(restRetryStarted{}).(*bool)
	if retryStarted == nil || !*retryStarted {
		*retryStarted = true

		return
	}

	incReqRetry(ctx)
}

func IncreaseReqRetryCntGRPC(ctx context.Context) {
	incReqRetry(ctx)
}

func incReqRetry(ctx context.Context) {
	cnt := ctx.Value(retryCnt{})

	i, _ := cnt.(*uint)
	if i != nil {
		*i++
	}
}

func getRetryCnt(ctx context.Context) uint {
	attempt, _ := ctx.Value(retryCnt{}).(*uint)
	if attempt == nil {
		return 0
	}

	return *attempt
}

func GetRetryCnt(context.Context) uint {
	return 0
}
