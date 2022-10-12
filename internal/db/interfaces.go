package db

import (
	c "context"
	"time"

	"github.com/iancoleman/orderedmap"

	"github.com/google/uuid"

	e "gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

type PipelineStorager interface {
	GetWorkedVersions(ctx c.Context) ([]e.EriusScenario, error)
	GetPipeline(ctx c.Context, id uuid.UUID) (*e.EriusScenario, error)

	CreatePipeline(c c.Context, p *e.EriusScenario, author string, pipelineData []byte) error
	PipelineRemovable(ctx c.Context, id uuid.UUID) (bool, error)
	DeletePipeline(ctx c.Context, id uuid.UUID) error
	RenamePipeline(ctx c.Context, id uuid.UUID, name string) error
}

type TaskStorager interface {
	GetAdditionalForms(workNumber, nodeName string) ([]string, error)
	GetApplicationData(workNumber string) (*orderedmap.OrderedMap, error)
	SetApplicationData(workNumber string, data *orderedmap.OrderedMap) error
	GetTasks(ctx c.Context, filters e.TaskFilter) (*e.EriusTasksPage, error)
	GetTasksCount(ctx c.Context, userName string) (*e.CountTasks, error)
	GetPipelineTasks(ctx c.Context, pipelineID uuid.UUID) (*e.EriusTasks, error)
	GetTask(ctx c.Context, workNumber string) (*e.EriusTask, error)
	GetTaskSteps(ctx c.Context, id uuid.UUID) (e.TaskSteps, error)
	GetUnfinishedTaskStepsByWorkIdAndStepType(ctx c.Context, id uuid.UUID, stepType string) (e.TaskSteps, error)
	GetTaskStepById(ctx c.Context, id uuid.UUID) (*e.Step, error)
	GetParentTaskStepByName(ctx c.Context, workID uuid.UUID, stepName string) (*e.Step, error)
	GetTaskStepByName(ctx c.Context, workID uuid.UUID, stepName string) (*e.Step, error)
	GetVersionTasks(ctx c.Context, versionID uuid.UUID) (*e.EriusTasks, error)
	GetLastDebugTask(ctx c.Context, versionID uuid.UUID, author string) (*e.EriusTask, error)
	GetUnfinishedTasks(ctx c.Context) (*e.EriusTasks, error)

	CreateTask(ctx c.Context, dto *CreateTaskDTO) (*e.EriusTask, error)
	ChangeTaskStatus(ctx c.Context, taskID uuid.UUID, status int) error
	UpdateTaskHumanStatus(ctx c.Context, taskID uuid.UUID, status string) error
	CheckTaskStepsExecuted(ctx c.Context, workNumber string, blocks []string) (bool, error)
}

type SaveStepRequest struct {
	WorkID      uuid.UUID
	StepType    string
	StepName    string
	Content     []byte
	BreakPoints []string
	HasError    bool
	Status      string
}

type UpdateStepRequest struct {
	Id             uuid.UUID
	Content        []byte
	BreakPoints    []string
	HasError       bool
	Status         string
	WithoutContent bool
}

type UpdateTaskBlocksDataRequest struct {
	Id                     uuid.UUID
	ActiveBlocks           map[string]struct{}
	SkippedBlocks          map[string]struct{}
	NotifiedBlocks         map[string][]string
	PrevUpdateStatusBlocks map[string]string
}

type SearchPipelineRequest struct {
	PipelineId   *string
	PipelineName *string
	Limit        int
	Offset       int
}

//go:generate mockery --name=Database --structname=MockedDatabase
type Database interface {
	PipelineStorager
	TaskStorager

	GetPipelinesWithLatestVersion(ctx c.Context, author string) ([]e.EriusScenarioInfo, error)
	GetApprovedVersions(ctx c.Context) ([]e.EriusScenarioInfo, error)
	GetVersionsByStatus(ctx c.Context, status int, author string) ([]e.EriusScenarioInfo, error)
	GetDraftVersions(ctx c.Context, author string) ([]e.EriusScenarioInfo, error)
	GetOnApproveVersions(ctx c.Context) ([]e.EriusScenarioInfo, error)
	SwitchApproved(ctx c.Context, pipelineID, versionID uuid.UUID, author string) error
	VersionEditable(ctx c.Context, versionID uuid.UUID) (bool, error)
	CreateVersion(ctx c.Context, p *e.EriusScenario, author string, pipelineData []byte) error
	DeleteVersion(ctx c.Context, versionID uuid.UUID) error
	GetPipelineVersion(ctx c.Context, id uuid.UUID) (*e.EriusScenario, error)
	GetPipelineVersions(ctx c.Context, id uuid.UUID) ([]e.EriusVersionInfo, error)
	UpdateDraft(ctx c.Context, p *e.EriusScenario, pipelineData []byte) error
	SaveStepContext(ctx c.Context, dto *SaveStepRequest) (uuid.UUID, time.Time, error)
	UpdateStepContext(ctx c.Context, dto *UpdateStepRequest) error
	UpdateTaskBlocksData(ctx c.Context, dto *UpdateTaskBlocksDataRequest) error

	GetExecutableScenarios(ctx c.Context) ([]e.EriusScenario, error)
	GetExecutableByName(ctx c.Context, name string) (*e.EriusScenario, error)

	ActiveAlertNGSA(ctx c.Context, sever int,
		state, source, eventType, cause, addInf, addTxt, moID, specProb, notID, usertext, moi, moc string) error
	ClearAlertNGSA(ctx c.Context, name string) error
	CreateTag(ctx c.Context, e *e.EriusTagInfo, author string) (*e.EriusTagInfo, error)
	GetTag(ctx c.Context, e *e.EriusTagInfo) (*e.EriusTagInfo, error)
	EditTag(ctx c.Context, e *e.EriusTagInfo) error
	RemoveTag(ctx c.Context, id uuid.UUID) error
	GetAllTags(ctx c.Context) ([]e.EriusTagInfo, error)
	GetPipelineTag(ctx c.Context, id uuid.UUID) ([]e.EriusTagInfo, error)
	AttachTag(ctx c.Context, id uuid.UUID, p *e.EriusTagInfo) error
	DetachTag(ctx c.Context, id uuid.UUID, p *e.EriusTagInfo) error
	RemovePipelineTags(ctx c.Context, id uuid.UUID) error
	DeleteAllVersions(ctx c.Context, id uuid.UUID) error
	PipelineNameCreatable(ctx c.Context, name string) (bool, error)
	SwitchRejected(ctx c.Context, versionID uuid.UUID, comment, author string) error
	GetRejectedVersions(ctx c.Context) ([]e.EriusScenarioInfo, error)
	RollbackVersion(ctx c.Context, pipelineID, versionID uuid.UUID) error
	GetVersionsByPipelineID(ctx c.Context, blueprintID string) ([]e.EriusScenario, error)
	GetVersionByWorkNumber(ctx c.Context, workNumber string) (*e.EriusScenario, error)
	GetPipelinesByNameOrId(ctx c.Context, dto *SearchPipelineRequest) ([]e.SearchPipeline, error)
	GetTaskStepByWorkNumber(ctx c.Context, workNumber string, stepName string) (*e.Step, error)
}
