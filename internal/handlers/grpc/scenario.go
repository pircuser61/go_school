package grpc

import (
	"context"
	pb "gitlab.services.mts.ru/jocasta/proto/gen/proto/go/scenario/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type scenarioHandler struct {
	pb.UnimplementedScenarioServiceServer
}

func NewScenarioHandler() *scenarioHandler {
	return &scenarioHandler{}
}

func (h *scenarioHandler) GetScenarioById(context.Context, *pb.GetScenarioRequest) (*pb.GetScenarioResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetScenarioById not implemented")
}
func (h *scenarioHandler) DeleteScenarioById(context.Context, *pb.DeleteScenarioRequest) (*pb.DeleteScenarioResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteScenarioById not implemented")
}
func (h *scenarioHandler) RunScenarioById(context.Context, *pb.RunScenarioRequest) (*pb.RunScenarioResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RunScenarioById not implemented")
}
func (h *scenarioHandler) CreateScenario(context.Context, *pb.CreateScenarioRequest) (*pb.CreateScenarioResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CreateScenario not implemented")
}

func (h *scenarioHandler) Mount(s *grpc.Server) {
	pb.RegisterScenarioServiceServer(s, h)
}
