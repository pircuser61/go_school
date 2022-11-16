package db

import (
	c "context"
	"golang.org/x/net/context"
	"time"

	"github.com/google/uuid"
	"github.com/iancoleman/orderedmap"

	"github.com/jackc/pgx/v4"

	e "gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type DictionaryStorager interface {
	GetApproveActionNames(ctx c.Context) ([]e.ApproveActionName, error)
	GetApproveStatuses(ctx c.Context) ([]e.ApproveStatus, error)
}

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
	GetParentTaskStepByName(ctx c.Context, tx pgx.Tx, workID uuid.UUID, stepName string) (*e.Step, error)
	GetTaskStepByName(ctx c.Context, workID uuid.UUID, stepName string) (*e.Step, error)
	GetVersionTasks(ctx c.Context, versionID uuid.UUID) (*e.EriusTasks, error)
	GetLastDebugTask(ctx c.Context, versionID uuid.UUID, author string) (*e.EriusTask, error)
	GetUnfinishedTasks(ctx c.Context) (*e.EriusTasks, error)
	GetUsersWithReadWriteFormAccess(ctx c.Context, tx pgx.Tx, workNumber, stepName string) ([]e.UsersWithFormAccess, error)

	CreateTask(ctx c.Context, tx pgx.Tx, dto *CreateTaskDTO) (*e.EriusTask, error)
	ChangeTaskStatus(ctx c.Context, tx pgx.Tx, taskID uuid.UUID, status int) error
	GetTaskStatus(ctx c.Context, tx pgx.Tx, taskID uuid.UUID) (int, error)
	StopTaskBlocks(ctx c.Context, tx pgx.Tx, taskID uuid.UUID) error
	UpdateTaskHumanStatus(ctx c.Context, tx pgx.Tx, taskID uuid.UUID, status string) error
	CheckTaskStepsExecuted(ctx c.Context, tx pgx.Tx, workNumber string, blocks []string) (bool, error)
	CheckUserCanEditForm(ctx c.Context, tx pgx.Tx, workNumber string, stepName string, login string) (bool, error)
	GetTaskRunContext(ctx c.Context, tx pgx.Tx, workNumber string) (e.TaskRunContext, error)
	GetBlockDataFromVersion(ctx c.Context, workNumber, blockName string) (*e.EriusFunc, error)
	GetVariableStorageForStep(ctx c.Context, taskID uuid.UUID, stepType string) (*store.VariableStore, error)
}

type SaveStepRequest struct {
	WorkID      uuid.UUID
	StepType    string
	StepName    string
	Content     []byte
	BreakPoints []string
	HasError    bool
	Status      string
	Members     map[string]struct{}
	CheckSLA    bool
	SLADeadline time.Time
}

type UpdateStepRequest struct {
	Id             uuid.UUID
	Content        []byte
	BreakPoints    []string
	HasError       bool
	Status         string
	WithoutContent bool
	Members        map[string]struct{}
	SLADeadline    time.Time
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
	DictionaryStorager

	MakeTransaction(ctx context.Context) (pgx.Tx, error)

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
	SaveStepContext(ctx c.Context, tx pgx.Tx, dto *SaveStepRequest) (uuid.UUID, time.Time, error)
	UpdateStepContext(ctx c.Context, tx pgx.Tx, dto *UpdateStepRequest) error
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
	PipelineNameCreatable(ctx c.Context, name string) (bool, error)
	SwitchRejected(ctx c.Context, versionID uuid.UUID, comment, author string) error
	GetRejectedVersions(ctx c.Context) ([]e.EriusScenarioInfo, error)
	RollbackVersion(ctx c.Context, pipelineID, versionID uuid.UUID) error
	GetVersionsByPipelineID(ctx c.Context, blueprintID string) ([]e.EriusScenario, error)
	GetVersionByWorkNumber(ctx c.Context, workNumber string) (*e.EriusScenario, error)
	GetPipelinesByNameOrId(ctx c.Context, dto *SearchPipelineRequest) ([]e.SearchPipeline, error)
}
