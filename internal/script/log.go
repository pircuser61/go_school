package script

import (
	c "context"
	"net/http"

	"gitlab.services.mts.ru/abp/myosotis/logger"
)

const (
	GRPC         = "GRPC"
	HTTP         = "HTTP"
	MethodPost   = http.MethodPost
	MethodGet    = http.MethodGet
	Method       = "method"
	MainFuncName = "mainFuncName"
	FuncName     = "funcName"
	Transport    = "transport"
	LogVersion   = "logVersion"
	TraceID      = "traceID"
	WorkID       = "workID"
	StepName     = "stepName"
	WorkNumber   = "workNumber"
	PipelineID   = "pipelineID"
	VersionID    = "versionID"
	StepID       = "stepID"
	Body         = "body"
)

type (
	retryCnt         struct{}
	restRetryStarted struct{}
)

func SetMainFuncLog(ctx c.Context, mainFuncName, method, transport, traceID, v string) logger.Logger {
	return logger.GetLogger(ctx).
		WithField(MainFuncName, mainFuncName).
		WithField(Method, method).
		WithField(Transport, transport).
		WithField(TraceID, traceID).
		WithField(LogVersion, v)
}

func SetFieldsExternalCall(ctx c.Context, traceID, v, tr, method, systemName string) logger.Logger {
	return logger.GetLogger(ctx).
		WithField(TraceID, traceID).
		WithField(Transport, tr).
		WithField(LogVersion, v).
		WithField("callMethod ", method).
		WithField("callTransport", tr).
		WithField("integrationName", systemName)
}

func LogRetryFailure(ctx c.Context, maxCount uint) {
	attempt := getRetryCnt(ctx)
	log := logger.GetLogger(ctx).WithField("attempt", attempt)

	if attempt == maxCount {
		log.Error("pipeliner failed to connect, Exceeded max retry count")

		return
	}

	log.Error("pipeliner failed to connect")
}

func LogRetrySuccess(ctx c.Context) {
	attempt := getRetryCnt(ctx)

	if attempt > 0 {
		logger.
			GetLogger(ctx).
			WithField("attempt", attempt).
			Info("pipeliner successfully reconnected")
	}
}

func MakeContextWithRetryCnt(ctx c.Context) c.Context {
	count := uint(0)
	retryStarted := false

	// счетчик ретраев
	ctx = c.WithValue(ctx, retryCnt{}, &count)
	// флаг для запросов по http
	ctx = c.WithValue(ctx, restRetryStarted{}, &retryStarted)

	return ctx
}

func IncreaseReqRetryCntREST(req *http.Request) {
	ctx := req.Context()

	retryStarted, _ := ctx.Value(restRetryStarted{}).(*bool)
	if retryStarted != nil && !*retryStarted {
		*retryStarted = true

		return
	}

	incReqRetry(ctx)
}

func IncreaseReqRetryCntGRPC(ctx c.Context) {
	incReqRetry(ctx)
}

func incReqRetry(ctx c.Context) {
	cnt := ctx.Value(retryCnt{})

	i, _ := cnt.(*uint)
	if i != nil {
		*i++
	}
}

func getRetryCnt(ctx c.Context) uint {
	attempt, _ := ctx.Value(retryCnt{}).(*uint)
	if attempt == nil {
		return 0
	}

	return *attempt
}
