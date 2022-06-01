package grpc

import (
	"context"
	"github.com/google/uuid"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/scenario"
	pb "gitlab.services.mts.ru/jocasta/proto/gen/proto/go/scenario/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type scenarioHandler struct {
	pb.UnimplementedScenarioServiceServer
	service *scenario.Service
}

func NewScenarioHandler(service *scenario.Service) *scenarioHandler {
	return &scenarioHandler{}
}

func (h *scenarioHandler) GetScenarioById(ctx context.Context, req *pb.GetScenarioRequest) (*pb.GetScenarioResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetScenarioById not implemented")
}
func (h *scenarioHandler) DeleteScenarioById(ctx context.Context, req *pb.DeleteScenarioRequest) (*pb.DeleteScenarioResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteScenarioById not implemented")
}
func (h *scenarioHandler) RunScenarioById(ctx context.Context, req *pb.RunScenarioRequest) (*pb.RunScenarioResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RunScenarioById not implemented")
}
func (h *scenarioHandler) CreateScenario(ctx context.Context, req *pb.CreateScenarioRequest) (*pb.CreateScenarioResponse, error) {
	s, err := h.service.CreateScenario(ctx, createScenarioToService(req))
	if err != nil {
		return nil, err
	}
	return createScenarioFromService(s), nil
}

func (h *scenarioHandler) Mount(s *grpc.Server) {
	pb.RegisterScenarioServiceServer(s, h)
}

func createScenarioFromService(in *entity.EriusScenarioV2) *pb.CreateScenarioResponse {

	return &pb.CreateScenarioResponse{
		Scenario: &pb.Scenario{
			Id:        in.ID.String(),
			VersionId: in.VersionID.String(),
			Status:    int64(in.Status),
			HasDraft:  in.HasDraft,
			Name:      in.Name,
			Input:     nil,
			Output:    nil,
			OnTrue:    "",
			OnFalse:   "",
			Final:     "",
			OnIter:    "",
			Next:      nil,
		},
	}
}

func createScenarioToService(in *pb.CreateScenarioRequest) *entity.EriusScenarioV2 {
	s := in.Scenario
	return &entity.EriusScenarioV2{
		ID:              uuid.UUID{},
		VersionID:       uuid.UUID{},
		Status:          int(s.Status),
		HasDraft:        s.HasDraft,
		Name:            s.Name,
		Input:           nil,
		Output:          nil,
		CreatedAt:       nil,
		ApprovedAt:      nil,
		Author:          "",
		Tags:            nil,
		Comment:         "",
		CommentRejected: "",
	}
}
