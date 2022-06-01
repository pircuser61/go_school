package scenario

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/scenario/rep"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/user"
)

type Service struct {
	store *rep.ScenarioRepository
}

func NewService(store *rep.ScenarioRepository) *Service {
	return &Service{
		store: store,
	}
}

func (s *Service) CreateScenario(ctx context.Context, scenario *entity.EriusScenarioV2) (*entity.EriusScenarioV2, error) {
	var b []byte
	if err := json.Unmarshal(b, scenario); err != nil {
		return nil, err
	}

	u, err := user.GetUserInfoFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	scenario.ID = uuid.New()
	scenario.VersionID = uuid.New()

	canCreate, err := s.store.PipelineNameCreatable(ctx, scenario.Name)
	if err != nil {
		return nil, err
	}

	if !canCreate {
		return nil, nil
	}

	err = s.store.CreatePipeline(ctx, scenario, u.Username, b)
	if err != nil {
		return nil, err
	}

	created, err := s.store.GetPipelineVersion(ctx, scenario.VersionID)
	if err != nil {
		return nil, err
	}

	return created, nil
}
