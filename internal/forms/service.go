package forms

import (
	c "context"
	"encoding/json"

	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/trace"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"

	forms_v1 "gitlab.services.mts.ru/jocasta/forms/pkg/forms/v1"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/metrics"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

const externalSystemName = "forms"

type Service struct {
	conn *grpc.ClientConn
	cli  forms_v1.FormsServiceClient
}

func NewService(cfg Config, _ logger.Logger, m metrics.Metrics) (*Service, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(&ocgrpc.ClientHandler{}),
		grpc.WithUnaryInterceptor(metrics.GrpcMetrics(externalSystemName, m)),
	}

	if cfg.MaxRetries != 0 {
		opts = append(opts, grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(
			grpc_retry.WithMax(cfg.MaxRetries),
			grpc_retry.WithBackoff(grpc_retry.BackoffLinear(cfg.RetryDelay)),
			grpc_retry.WithPerRetryTimeout(cfg.Timeout),
			grpc_retry.WithCodes(codes.Unavailable, codes.ResourceExhausted, codes.DataLoss, codes.DeadlineExceeded, codes.Unknown),
			grpc_retry.WithOnRetryCallback(func(ctx c.Context, attempt uint, err error) {
				script.IncreaseReqRetryCntGRPC(ctx)
			}),
		)))
	}

	conn, err := grpc.NewClient(cfg.URL, opts...)
	if err != nil {
		return nil, err
	}

	client := forms_v1.NewFormsServiceClient(conn)

	return &Service{
		conn: conn,
		cli:  client,
	}, nil
}

func (s *Service) MakeFlatSchema(ctx c.Context, schema []byte) (*script.JSONSchema, error) {
	ctxLocal, span := trace.StartSpan(ctx, "forms.make_flat_schema")
	defer span.End()

	log := logger.GetLogger(ctxLocal).
		WithField("traceID", span.SpanContext().TraceID.String()).WithField("transport", "GRPC")

	ctxLocal = script.MakeContextWithRetryCnt(ctxLocal)

	res, err := s.cli.ConvertToFlatJSONSchema(ctxLocal, &forms_v1.InputJSONSchema{Schema: schema})

	attempt := script.GetRetryCnt(ctxLocal)

	if err != nil {
		log.Warning("Pipeliner failed to connect to forms. Exceeded max retry count: ", attempt)

		return nil, err
	}

	if attempt > 0 {
		log.Warning("Pipeliner successfully reconnected to forms: ", attempt)
	}

	if res != nil {
		var newSchema *script.JSONSchema
		if unmErr := json.Unmarshal(res.Schema, &newSchema); unmErr != nil {
			return nil, unmErr
		}

		return newSchema, nil
	}

	return nil, nil
}
