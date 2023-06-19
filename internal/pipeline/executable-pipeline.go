package pipeline

import (
	c "context"
	"net/http"

	"go.opencensus.io/trace"

	"golang.org/x/net/context"

	"github.com/google/uuid"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	file_registry "gitlab.services.mts.ru/jocasta/pipeliner/internal/file-registry"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/functions"
	human_tasks "gitlab.services.mts.ru/jocasta/pipeliner/internal/human-tasks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/integrations"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/kafka"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
)

type void = struct{}

type ExecutablePipeline struct {
	TaskID        uuid.UUID
	WorkNumber    string
	PipelineID    uuid.UUID
	VersionID     uuid.UUID
	Storage       db.Database
	EntryPoint    string
	StepType      string
	ActiveBlocks  map[string]void
	SkippedBlocks map[string]void
	VarStore      *store.VariableStore
	Blocks        map[string]Runner
	Nexts         map[string][]string
	Sockets       []script.Socket
	Input         map[string]string
	Output        map[string]string
	Name          string
	PipelineModel *entity.EriusScenario
	HTTPClient    *http.Client
	Remedy        string
	Sender        *mail.Service
	Kafka         *kafka.Service
	People        *people.Service
	ServiceDesc   *servicedesc.Service
	FunctionStore *functions.Service
	HumanTasks    *human_tasks.Service
	Integrations  *integrations.Service
	FileRegistry  *file_registry.Service

	FaaS string

	RunContext *BlockRunContext
}

func (gb *ExecutablePipeline) Members() []Member {
	return nil
}

func (gb *ExecutablePipeline) Deadlines(_ context.Context) ([]Deadline, error) {
	return []Deadline{}, nil
}

func (gb *ExecutablePipeline) GetStatus() Status {
	switch {
	case gb.IsOver():
		return StatusFinished
	case gb.ReadyToStart():
		return StatusReady
	case len(gb.ActiveBlocks) != 0:
		return StatusRunning
	default:
		return StatusIdle
	}
}

func (gb *ExecutablePipeline) UpdateManual() bool {
	return false
}

func (gb *ExecutablePipeline) IsOver() bool {
	return len(gb.ActiveBlocks) == 0
}

func (gb *ExecutablePipeline) ReadyToStart() bool {
	return len(gb.ActiveBlocks) == 0 && gb.EntryPoint == BlockGoFirstStart
}

func (gb *ExecutablePipeline) GetTaskHumanStatus() TaskHumanStatus {
	// TODO: проверять, что нет ошибок (потому что только тогда мы Done)
	if len(gb.ActiveBlocks) == 0 {
		return StatusDone
	}
	return StatusNew
}

type CreateTaskDTO struct {
	Author     string
	IsDebug    bool
	Params     []byte
	WorkNumber string
	RunCtx     entity.TaskRunContext
}

func (gb *ExecutablePipeline) CreateTask(ctx c.Context, dto *CreateTaskDTO) error {
	gb.TaskID = uuid.New()

	task, err := gb.Storage.CreateTask(ctx, &db.CreateTaskDTO{
		TaskID:     gb.TaskID,
		VersionID:  gb.VersionID,
		Author:     dto.Author,
		WorkNumber: dto.WorkNumber,
		IsDebug:    dto.IsDebug,
		Params:     dto.Params,
		RunCtx:     dto.RunCtx,
	})
	if err != nil {
		return err
	}

	gb.WorkNumber = task.WorkNumber
	return nil
}

func (gb *ExecutablePipeline) Next(_ *store.VariableStore) ([]string, bool) {
	nexts, ok := script.GetNexts(gb.Sockets, DefaultSocketID)
	if !ok {
		return nil, false
	}
	return nexts, true
}

func (gb *ExecutablePipeline) GetState() interface{} {
	return nil
}

func (gb *ExecutablePipeline) Update(_ c.Context) (interface{}, error) {
	return nil, nil
}

func (gb *ExecutablePipeline) CreateBlocks(ctx c.Context, source map[string]entity.EriusFunc) error {
	gb.Blocks = make(map[string]Runner)

	ctx, s := trace.StartSpan(ctx, "create_blocks")
	defer s.End()
	isTest, err := gb.Storage.CheckIsTest(ctx, gb.TaskID)
	if err != nil {
		return err
	}
	notifName := gb.Name
	if isTest {
		notifName = notifName + " (ТЕСТОВАЯ ЗАЯВКА)"
	}
	for k := range source {
		bl := source[k]

		block, _, err := CreateBlock(ctx, k, &bl, &BlockRunContext{
			TaskID:     gb.TaskID,
			WorkNumber: gb.WorkNumber,
			WorkTitle:  gb.Name,
			Initiator:  gb.RunContext.Initiator,
			Storage:    gb.Storage,
			VarStore:   gb.VarStore,

			Sender:        gb.Sender,
			Kafka:         gb.Kafka,
			People:        gb.People,
			ServiceDesc:   gb.ServiceDesc,
			FunctionStore: gb.FunctionStore,
			HumanTasks:    gb.HumanTasks,
			Integrations:  gb.Integrations,
			FaaS:          gb.FaaS,
			HrGate:        gb.RunContext.HrGate,
			UpdateData:    nil,
			IsTest:        isTest,
			NotifName:     notifName,
		})
		if err != nil {
			return err
		}

		gb.Blocks[k] = block
	}

	return nil
}
