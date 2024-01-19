package test

import (
	"context"

	"github.com/pkg/errors"

	"github.com/google/uuid"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

var errMocked = errors.New("mocked")

// nolint:gochecknoglobals // nedded for tests
var (
	Test1 = func() entity.EriusScenario {
		return entity.EriusScenario{
			ID:   uuid.MustParse("5238e070-46e0-4f7d-ae3b-1a4eea0d608f"),
			Name: "test",
			Pipeline: entity.PipelineType{
				Blocks: entity.BlocksType{
					"block": {
						BlockType: script.TypeScenario,
						Title:     "parent",
					},
				},
			},
		}
	}

	Test2 = func() entity.EriusScenario {
		return entity.EriusScenario{
			ID:   uuid.MustParse("5238e070-46e0-4f7d-ae3b-1a4eea0d608f"),
			Name: "test2",
			Pipeline: entity.PipelineType{
				Blocks: entity.BlocksType{
					"block": {
						BlockType: script.TypeScenario,
						Title:     "noparent",
					},
				},
			},
		}
	}
)

type MockPipelinerStorer struct {
	Worked func() ([]entity.EriusScenario, error)
	Get    func() (*entity.EriusScenario, error)
}

func (m MockPipelinerStorer) GetWorkedVersions(_ context.Context) ([]entity.EriusScenario, error) {
	return m.Worked()
}

func (m MockPipelinerStorer) GetPipeline(_ context.Context, _ uuid.UUID) (*entity.EriusScenario, error) {
	return m.Get()
}

func (m MockPipelinerStorer) CreatePipeline(_ context.Context, _ *entity.EriusScenario, _ string, _ []byte) error {
	return errMocked
}

func (m MockPipelinerStorer) DeletePipeline(_ context.Context, _ uuid.UUID) error {
	return errMocked
}

func (m MockPipelinerStorer) PipelineRemovable(_ context.Context, _ uuid.UUID) (bool, error) {
	return false, errMocked
}

func (m *MockPipelinerStorer) RenamePipeline(_ context.Context, _ uuid.UUID, _ string) error {
	return errMocked
}
