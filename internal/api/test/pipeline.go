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

func (m MockPipelinerStorer) GetWorkedVersions(c context.Context) ([]entity.EriusScenario, error) {
	return m.Worked()
}

func (m MockPipelinerStorer) GetPipeline(c context.Context, id uuid.UUID) (*entity.EriusScenario, error) {
	return m.Get()
}

func (m MockPipelinerStorer) CreatePipeline(c context.Context, p *entity.EriusScenario, author string, pipelineData []byte) error {
	return errMocked
}

func (m MockPipelinerStorer) DeletePipeline(c context.Context, id uuid.UUID) error {
	return errMocked
}

func (m MockPipelinerStorer) PipelineRemovable(c context.Context, id uuid.UUID) (bool, error) {
	return false, errMocked
}

func (m *MockPipelinerStorer) RenamePipeline(c context.Context, id uuid.UUID, name string) error {
	return errMocked
}
