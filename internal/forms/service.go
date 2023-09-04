package forms

import (
	c "context"
	"encoding/json"

	"go.opencensus.io/plugin/ocgrpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	forms_v1 "gitlab.services.mts.ru/jocasta/forms/pkg/forms/v1"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

type Service struct {
	c   *grpc.ClientConn
	cli forms_v1.FormsServiceClient
}

func NewService(cfg Config) (*Service, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(&ocgrpc.ClientHandler{})}
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
