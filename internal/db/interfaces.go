package db

import (
	"context"

	"github.com/google/uuid"
	"gitlab.services.mts.ru/erius/pipeliner/internal/entity"
)

type Database interface {
	GetApprovedVersions(c context.Context) ([]entity.EriusScenarioInfo, error)
	GetVersionsByStatus(c context.Context, status int) ([]entity.EriusScenarioInfo, error)
	GetDraftVersions(c context.Context) ([]entity.EriusScenarioInfo, error)
	GetOnApproveVersions(c context.Context) ([]entity.EriusScenarioInfo, error)
	GetWorkedVersions(c context.Context) ([]entity.EriusScenario, error)
	SwitchApproved(c context.Context, pipelineID, versionID uuid.UUID, author string) error
	VersionEditable(c context.Context, versionID uuid.UUID) (bool, error)
	CreatePipeline(c context.Context,
		p *entity.EriusScenario, author string, pipelineData []byte) error
	CreateVersion(c context.Context,
		p *entity.EriusScenario, author string, pipelineData []byte) error
	DeleteVersion(c context.Context, versionID uuid.UUID) error
	DeletePipeline(c context.Context, id uuid.UUID) error
	GetPipeline(c context.Context, id uuid.UUID) (*entity.EriusScenario, error)
	GetPipelineVersion(c context.Context, id uuid.UUID) (*entity.EriusScenario, error)
	UpdateDraft(c context.Context,
		p *entity.EriusScenario, pipelineData []byte) error
	WriteContext(c context.Context, workID uuid.UUID, stage string, data []byte) error
	WriteTask(c context.Context,
		workID, versionID uuid.UUID, author string) error
	ChangeWorkStatus(c context.Context,
		workID uuid.UUID, status int) error
	GetExecutableScenarios(c context.Context) ([]entity.EriusScenario, error)
	GetExecutableByName(c context.Context, name string) (*entity.EriusScenario, error)

	ActiveAlertNGSA(c context.Context, sever int,
		state, source, eventType, cause, addInf, addTxt, moID, specProb, notID, usertext, moi, moc string) error
	ClearAlertNGSA(c context.Context, name string) error
	GetPipelineTasks(c context.Context, id uuid.UUID) (*entity.EriusTasks, error)
	GetVersionTasks(c context.Context, id uuid.UUID) (*entity.EriusTasks, error)
	GetTaskLog(c context.Context, id uuid.UUID) (*entity.EriusLog, error)
}
