package metrics

import (
	c "context"
	"net/http"
	"time"

	g "google.golang.org/grpc"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

func GrpcMetrics(
	sysName string,
	metrics Metrics,
) func(ctx c.Context, md string, r, res any, cc *g.ClientConn, inv g.UnaryInvoker, op ...g.CallOption) error {
	return func(ctx c.Context, md string, r, res any, cc *g.ClientConn, inv g.UnaryInvoker, op ...g.CallOption) error {
		info := NewExternalRequestInfo(sysName)
		info.Method = "grpc"
		info.URL = md

		reqID := ctx.Value(script.RequestID{})
		if reqID == nil {
			reqID = ""
		}

		info.TraceID = reqID.(string)

		start := time.Now()

		err := inv(ctx, md, r, res, cc, op...)

		statusCode := http.StatusOK
		if err != nil {
			statusCode = http.StatusInternalServerError
		}

		info.ResponseCode = statusCode
		info.Duration = time.Since(start)

		metrics.Request2ExternalSystem(info)

		return err
	}
}
