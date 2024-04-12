package pipeline

import (
	c "context"
	"net/http"

	"github.com/google/uuid"
	"go.opencensus.io/trace"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	file_registry "gitlab.services.mts.ru/jocasta/pipeliner/internal/fileregistry"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/functions"
	human_tasks "gitlab.services.mts.ru/jocasta/pipeliner/internal/humantasks"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/integrations"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/kafka"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/mail"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/scheduler"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
	"gitlab.services.mts.ru/jocasta/pipeliner/utils"
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
	Scheduler     *scheduler.Service

	FaaS string

	RunContext *BlockRunContext

	happenedEvents      []entity.NodeEvent
	happenedKafkaEvents []entity.NodeKafkaEvent
}

func (gb *ExecutablePipeline) CurrentExecutorData() CurrentExecutorData {
	return CurrentExecutorData{}
}

func (gb *ExecutablePipeline) GetNewEvents() []entity.NodeEvent {
	return gb.happenedEvents
}

func (gb *ExecutablePipeline) GetNewKafkaEvents() []entity.NodeKafkaEvent {
	return gb.happenedKafkaEvents
}

func (gb *ExecutablePipeline) GoNotificationBlock() []entity.NodeKafkaEvent {
	return gb.happenedKafkaEvents
}

func (gb *ExecutablePipeline) Members() []Member {
	return nil
}

func (gb *ExecutablePipeline) Deadlines(_ c.Context) ([]Deadline, error) {
	return []Deadline{}, nil
}

func (gb *ExecutablePipeline) GetStatus() Status {
	switch {
	case gb.IsOver():
		return StatusFinished
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

func (gb *ExecutablePipeline) GetTaskHumanStatus() (status TaskHumanStatus, comment, action string) {
	// TODO: проверять, что нет ошибок (потому что только тогда мы Done)
	if len(gb.ActiveBlocks) == 0 {
		return StatusDone, "", ""
	}

	return StatusNew, "", ""
}

type CreateTaskDTO struct {
	Author     string
	RealAuthor string
	IsDebug    bool
	Params     []byte
	WorkNumber string
	RunCtx     entity.TaskRunContext
}

//nolint:gocritic //in struct field without pointer
func NewCreateTaskDTO(
	author, realAuthor string,
	isDebug bool,
	params []byte,
	workNumber string,
	runCtx entity.TaskRunContext,
) CreateTaskDTO {
	return CreateTaskDTO{
		Author:     author,
		RealAuthor: realAuthor,
		IsDebug:    isDebug,
		Params:     params,
		WorkNumber: workNumber,
		RunCtx:     runCtx,
	}
}

func (gb *ExecutablePipeline) CreateTask(ctx c.Context, dto *CreateTaskDTO) error {
	gb.TaskID = uuid.New()

	taskDTO := db.NewCreateTaskDTO(
		gb.TaskID,
		gb.VersionID,
		dto.Author,
		dto.RealAuthor,
		dto.WorkNumber,
		dto.IsDebug,
		dto.Params,
		dto.RunCtx,
	)

	task, err := gb.Storage.CreateTask(ctx, &taskDTO)
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

func (gb *ExecutablePipeline) CreateBlocks(ctx c.Context, source map[string]*entity.EriusFunc) error {
	gb.Blocks = make(map[string]Runner)

	ctx, s := trace.StartSpan(ctx, "create_blocks")
	defer s.End()

	props, err := gb.Storage.GetTaskCustomProps(ctx, gb.TaskID)
	if err != nil {
		return err
	}

	for k := range source {
		bl := source[k]

		block, _, err := CreateBlock(ctx, k, bl, &BlockRunContext{
			TaskID:     gb.TaskID,
			WorkNumber: gb.WorkNumber,
			WorkTitle:  gb.Name,
			Initiator:  gb.RunContext.Initiator,
			Services: RunContextServices{
				HTTPClient:    gb.RunContext.Services.HTTPClient,
				Storage:       gb.Storage,
				Sender:        gb.Sender,
				Kafka:         gb.Kafka,
				People:        gb.People,
				ServiceDesc:   gb.ServiceDesc,
				FunctionStore: gb.FunctionStore,
				HumanTasks:    gb.HumanTasks,
				Integrations:  gb.Integrations,
				FileRegistry:  gb.FileRegistry,
				FaaS:          gb.FaaS,
				HrGate:        gb.RunContext.Services.HrGate,
				Scheduler:     gb.RunContext.Services.Scheduler,
				SLAService:    gb.RunContext.Services.SLAService,
			},
			BlockRunResults: &BlockRunResults{},

			VarStore: gb.VarStore,

			UpdateData: nil,
			IsTest:     props.IsTest,
			NotifName:  utils.MakeTaskTitle(gb.Name, props.CustomTitle, props.IsTest),
			Productive: true,
		})
		if err != nil {
			return err
		}

		gb.Blocks[k] = block
	}

	return nil
}

func (gb *ExecutablePipeline) BlockAttachments() (ids []string) {
	return ids
}

func (gb *ExecutablePipeline) UpdateStateUsingOutput(_ c.Context, _ []byte) (state map[string]interface{}, err error) {
	return nil, nil
}

func (gb *ExecutablePipeline) UpdateOutputUsingState(_ c.Context) (output map[string]interface{}, err error) {
	return nil, nil
}
