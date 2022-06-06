package grpc

import (
	"context"
	"github.com/google/uuid"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
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
	return &scenarioHandler{
		service: service,
	}
}

func (h *scenarioHandler) GetScenarioById(ctx context.Context, req *pb.GetScenarioRequest) (*pb.GetScenarioResponse, error) {
	//
	//id, err := uuid.Parse(req.GetScenarioId())
	//if err != nil {
	//	return nil, err
	//}
	//
	//s, err := h.service.GetScenario(ctx, id)
	//if err != nil {
	//	return nil, err
	//}
	//return createScenarioFromService(s), nil
	return nil, nil
}
func (h *scenarioHandler) DeleteScenarioById(ctx context.Context, req *pb.DeleteScenarioRequest) (*pb.DeleteScenarioResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteScenarioById not implemented")
}
func (h *scenarioHandler) RunScenarioById(ctx context.Context, req *pb.RunScenarioRequest) (*pb.RunScenarioResponse, error) {

	return nil, status.Errorf(codes.Unimplemented, "method RunScenarioById not implemented")
}
func (h *scenarioHandler) CreateScenario(ctx context.Context, req *pb.CreateScenarioRequest) (*pb.GetScenarioResponse, error) {
	s, err := h.service.CreateScenario(ctx, createScenarioToService(req))
	if err != nil {
		return nil, err
	}
	return createScenarioFromService(s), nil
}

func (h *scenarioHandler) Mount(s *grpc.Server) {
	pb.RegisterScenarioServiceServer(s, h)
}

func createScenarioFromService(in *entity.EriusScenarioV2) *pb.GetScenarioResponse {
	// todo: dodelat'
	//input := make([]*pb.FunctionValue, 0)
	//for _, v := range in.Input {
	//	input = append(input, &pb.FunctionValue{
	//		Name:   v.Name,
	//		Type:   v.Type,
	//		Global: v.Global,
	//	})
	//}
	//
	//output := make([]*pb.FunctionValue, 0)
	//for _, v := range in.Output {
	//	output = append(output, &pb.FunctionValue{
	//		Name:   v.Name,
	//		Type:   v.Type,
	//		Global: v.Global,
	//	})
	//}
	//
	//return &pb.GetScenarioResponse{
	//		Id:        in.ID.String(),
	//		VersionId: in.VersionID.String(),
	//		Status:    int64(in.Status),
	//		HasDraft:  in.HasDraft,
	//		Name:      in.Name,
	//		Input:     input,
	//		Output:    output,
	//		OnTrue:    "",
	//		OnFalse:   "",
	//		Final:     "",
	//		OnIter:    "",
	//		Next:      nil,
	//	}
	//
	return nil
}

func createScenarioToService(in *pb.CreateScenarioRequest) *entity.EriusScenarioV2 {

	input := make([]entity.EriusFunctionValue, 0)
	for _, v := range in.Input {
		input = append(input, entity.EriusFunctionValue{
			Name:   v.Name,
			Type:   v.Type,
			Global: v.Global,
		})
	}

	output := make([]entity.EriusFunctionValue, 0)
	for _, v := range in.Output {
		output = append(output, entity.EriusFunctionValue{
			Name:   v.Name,
			Type:   v.Type,
			Global: v.Global,
		})
	}

	blockMap := make(map[string]entity.EriusFuncV2)

	for k, v := range in.GetPipeline().GetBlocks() {

		input := make([]entity.EriusFunctionValue, 0)
		for _, v := range v.Input {
			input = append(input, entity.EriusFunctionValue{
				Name:   v.Name,
				Type:   v.Type,
				Global: v.Global,
			})
		}

		output := make([]entity.EriusFunctionValue, 0)
		for _, v := range v.Output {
			output = append(output, entity.EriusFunctionValue{
				Name:   v.Name,
				Type:   v.Type,
				Global: v.Global,
			})
		}

		blockMap[k] = entity.EriusFuncV2{
			X:         int(v.X),
			Y:         int(v.Y),
			BlockType: v.BlockType,
			Title:     v.Title,
			Input:     input,
			Output:    output,
			OnTrue:    v.OnTrue,
			OnFalse:   v.OnFalse,
			Final:     v.Final,
			OnIter:    v.OnIter,
			Next:      v.Next,
		}
	}

	return &entity.EriusScenarioV2{
		ID:        uuid.New(),
		VersionID: uuid.New(),
		Status:    db.StatusDraft,
		HasDraft:  true,
		Name:      in.Name,
		Input:     input,
		Output:    output,
		Pipeline: entity.Pipeline{
			Entrypoint: in.GetPipeline().GetEntrypoint(),
			Blocks:     blockMap,
		},
		CreatedAt:       nil,
		ApprovedAt:      nil,
		Author:          "",
		Tags:            nil,
		Comment:         "",
		CommentRejected: "",
	}
}
