package metrics

import (
	c "context"
	"net/http"
	"time"

	g "google.golang.org/grpc"
)

func GrpcMetrics(
	sysName string,
	metrics Metrics,
) func(ctx c.Context, md string, r, res any, cc *g.ClientConn, inv g.UnaryInvoker, op ...g.CallOption) error {
	return func(ctx c.Context, md string, r, res any, cc *g.ClientConn, inv g.UnaryInvoker, op ...g.CallOption) error {
		info := NewExternalRequestInfo(sysName)
		info.Method = "grpc"
		info.URL = md
		info.TraceID = ""

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
