package forms

import (
	c "context"
	"encoding/json"

	"go.opencensus.io/plugin/ocgrpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"

	forms_v1 "gitlab.services.mts.ru/jocasta/forms/pkg/forms/v1"

	"gitlab.services.mts.ru/abp/myosotis/logger"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

type Service struct {
	c   *grpc.ClientConn
	cli forms_v1.FormsServiceClient
}

func NewService(cfg Config, log logger.Logger) (*Service, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(&ocgrpc.ClientHandler{}),
	}

	if cfg.MaxRetries != 0 {
		opts = append(opts, grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(
			grpc_retry.WithMax(cfg.MaxRetries),
			grpc_retry.WithBackoff(grpc_retry.BackoffLinear(cfg.RetryDelay)),
			grpc_retry.WithPerRetryTimeout(cfg.Timeout),
			grpc_retry.WithCodes(codes.Unavailable, codes.ResourceExhausted, codes.DataLoss, codes.DeadlineExceeded, codes.Unknown),
			grpc_retry.WithOnRetryCallback(func(ctx c.Context, attempt uint, err error) {
				log.WithError(err).WithField("attempt", attempt).Error("failed to reconnect to forms")
			}),
		)))
	}

	conn, err := grpc.Dial(cfg.URL, opts...)
	if err != nil {
		return nil, err
	}

	client := forms_v1.NewFormsServiceClient(conn)

	return &Service{
		c:   conn,
		cli: client,
	}, nil
}

func (s *Service) MakeFlatSchema(ctx c.Context, schema []byte) (*script.JSONSchema, error) {
	res, err := s.cli.ConvertToFlatJSONSchema(ctx, &forms_v1.InputJSONSchema{Schema: schema})
	if err != nil {
		return nil, err
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
