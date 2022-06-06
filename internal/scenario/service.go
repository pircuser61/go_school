package scenario

import (
	"context"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/scenario/rep"
)

type Service struct {
	store *rep.ScenarioRepository
}

func NewService(store *rep.ScenarioRepository) *Service {
	return &Service{
		store: store,
	}
}

func (s *Service) GetScenario(ctx context.Context, id string) (*entity.EriusScenarioV2, error) {
	scenario, err := s.store.GetPipelineVersion(ctx, id)
	if err != nil {
		return nil, err
	}

	return scenario, nil
}
