package db

import (
	"context"

	"github.com/google/uuid"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

type PipelineStorager interface {
	CreatePipeline(c context.Context,
		p *entity.EriusScenario, author string, pipelineData []byte) error
	GetWorkedVersions(c context.Context) ([]entity.EriusScenario, error)
	GetPipeline(c context.Context, id uuid.UUID) (*entity.EriusScenario, error)
	PipelineRemovable(c context.Context, id uuid.UUID) (bool, error)
	DeletePipeline(c context.Context, id uuid.UUID) error
}

type TaskStorager interface {
	GetTasks(c context.Context, filters entity.TaskFilter) (*entity.EriusTasksPage, error)
	GetPipelineTasks(c context.Context, pipelineID uuid.UUID) (*entity.EriusTasks, error)
	GetTask(c context.Context, id uuid.UUID) (*entity.EriusTask, error)
	GetTaskSteps(c context.Context, id uuid.UUID) (entity.TaskSteps, error)
	GetTaskStepById(ctx context.Context, id uuid.UUID) (*entity.Step, error)
	CreateTask(c context.Context,
		taskID, versionID uuid.UUID, author string, isDebugMode bool, parameters []byte) (*entity.EriusTask, error)
	ChangeTaskStatus(c context.Context, taskID uuid.UUID, status int) error
	GetVersionTasks(c context.Context, versionID uuid.UUID) (*entity.EriusTasks, error)
	GetLastDebugTask(c context.Context, versionID uuid.UUID, author string) (*entity.EriusTask, error)
}

type SaveStepRequest struct {
	WorkID      uuid.UUID
	StepType    string
	StepName    string
	Content     []byte
	BreakPoints []string
	HasError    bool
	IsFinished  bool
}

type UpdateStepRequest struct {
	Id          uuid.UUID
	Content     []byte
	BreakPoints []string
	HasError    bool
	IsFinished  bool
}

//go:generate mockery --name=Database --structname=MockedDatabase
type Database interface {
	PipelineStorager
	TaskStorager

	GetApprovedVersions(c context.Context) ([]entity.EriusScenarioInfo, error)
	GetVersionsByStatus(c context.Context, status int) ([]entity.EriusScenarioInfo, error)
	GetDraftVersions(c context.Context) ([]entity.EriusScenarioInfo, error)
	GetOnApproveVersions(c context.Context) ([]entity.EriusScenarioInfo, error)
	SwitchApproved(c context.Context, pipelineID, versionID uuid.UUID, author string) error
	VersionEditable(c context.Context, versionID uuid.UUID) (bool, error)
	CreateVersion(c context.Context,
		p *entity.EriusScenario, author string, pipelineData []byte) error
	DeleteVersion(c context.Context, versionID uuid.UUID) error
	GetPipelineVersion(c context.Context, id uuid.UUID) (*entity.EriusScenario, error)
	UpdateDraft(c context.Context,
		p *entity.EriusScenario, pipelineData []byte) error
	SaveStepContext(c context.Context, dto *SaveStepRequest) (uuid.UUID, error)
	UpdateStepContext(c context.Context, dto *UpdateStepRequest) error

	GetExecutableScenarios(c context.Context) ([]entity.EriusScenario, error)
	GetExecutableByName(c context.Context, name string) (*entity.EriusScenario, error)

	ActiveAlertNGSA(c context.Context, sever int,
		state, source, eventType, cause, addInf, addTxt, moID, specProb, notID, usertext, moi, moc string) error
	ClearAlertNGSA(c context.Context, name string) error
	CreateTag(c context.Context, e *entity.EriusTagInfo, author string) (*entity.EriusTagInfo, error)
	GetTag(c context.Context, e *entity.EriusTagInfo) (*entity.EriusTagInfo, error)
	EditTag(c context.Context, e *entity.EriusTagInfo) error
	RemoveTag(c context.Context, id uuid.UUID) error
	GetAllTags(c context.Context) ([]entity.EriusTagInfo, error)
	GetPipelineTag(c context.Context, id uuid.UUID) ([]entity.EriusTagInfo, error)
	AttachTag(c context.Context, p uuid.UUID, e *entity.EriusTagInfo) error
	DetachTag(c context.Context, p uuid.UUID, e *entity.EriusTagInfo) error
	RemovePipelineTags(c context.Context, id uuid.UUID) error
	DraftPipelineCreatable(c context.Context, id uuid.UUID, author string) (bool, error)
	DeleteAllVersions(c context.Context, id uuid.UUID) error
	PipelineNameCreatable(c context.Context, name string) (bool, error)
	SwitchRejected(c context.Context, versionID uuid.UUID, comment, author string) error
	GetRejectedVersions(c context.Context) ([]entity.EriusScenarioInfo, error)
	RollbackVersion(c context.Context, pipelineID, versionID uuid.UUID) error
}
