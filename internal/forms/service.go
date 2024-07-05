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
	conn          *grpc.ClientConn
	cli           forms_v1.FormsServiceClient
	maxRetryCount uint
}

func NewService(cfg Config, m metrics.Metrics) (*Service, error) {
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
			grpc_retry.WithOnRetryCallback(func(ctx c.Context, _ uint, _ error) {
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
		conn:          conn,
		cli:           client,
		maxRetryCount: cfg.MaxRetries,
	}, nil
}

func (s *Service) MakeFlatSchema(ctx c.Context, schema []byte) (*script.JSONSchema, error) {
	ctx, span := trace.StartSpan(ctx, "forms.make_flat_schema")
	defer span.End()

	traceID := span.SpanContext().TraceID.String()
	log := script.SetFieldsExternalCall(ctx, traceID, "v1", script.GRPC, script.GRPC, externalSystemName)

	ctx = script.MakeContextWithRetryCnt(ctx)
	ctx = logger.WithLogger(ctx, log)

	res, err := s.cli.ConvertToFlatJSONSchema(ctx, &forms_v1.InputJSONSchema{Schema: schema})
	if err != nil {
		script.LogRetryFailure(ctx, s.maxRetryCount)

		return nil, err
	}

	script.LogRetrySuccess(ctx)

	if res != nil {
		var newSchema *script.JSONSchema
		if unmErr := json.Unmarshal(res.Schema, &newSchema); unmErr != nil {
			return nil, unmErr
		}

		return newSchema, nil
	}

	return nil, nil
}
