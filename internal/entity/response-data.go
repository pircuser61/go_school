package entity

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
)

type EriusScenarioInfo struct {
	ID              uuid.UUID      `json:"id" example:"916ad995-8d13-49fb-82ee-edd4f97649e2" format:"uuid"`
	VersionID       uuid.UUID      `json:"version_id" example:"916ad995-8d13-49fb-82ee-edd4f97649e2" format:"uuid"`
	CreatedAt       time.Time      `json:"created_at" example:"2020-07-16T17:10:25.112704+03:00"`
	ApprovedAt      *time.Time     `json:"approved_at" example:"2020-07-16T17:10:25.112704+03:00"`
	Author          string         `json:"author" example:"testAuthor"`
	Approver        string         `json:"approver" example:"testApprover"`
	Name            string         `json:"name" example:"ScenarioName"`
	Tags            []EriusTagInfo `json:"tags"`
	LastRun         *time.Time     `json:"last_run" example:"2020-07-16T17:10:25.112704+03:00"`
	LastRunStatus   *string        `json:"last_run_status"`
	Status          int            `json:"status" enums:"1,2,3,4,5"` // 1 - Draft, 2 - Approved, 3 - Deleted, 4 - Rejected, 5 - On Approve
	Comment         string         `json:"comment"`
	CommentRejected string         `json:"comment_rejected"`
}

type EriusVersionInfo struct {
	VersionID  uuid.UUID  `json:"version_id" example:"916ad995-8d13-49fb-82ee-edd4f97649e2" format:"uuid"`
	ApprovedAt *time.Time `json:"approved_at" example:"2020-07-16T17:10:25.112704+03:00"`
	Approver   string     `json:"approver,omitempty" example:"testApprover"`
	Author     string     `json:"author" example:"testAuthor"`
	CreatedAt  time.Time  `json:"created_at" example:"2020-07-16T17:10:25.112704+03:00"`
	UpdatedAt  time.Time  `json:"updated_at" example:"2020-07-16T17:10:25.112704+03:00"`
	IsActual   bool       `json:"is_actual"`
	Status     int        `json:"status"`
}

type EriusTagInfo struct {
	ID       uuid.UUID `json:"id" example:"916ad995-8d13-49fb-82ee-edd4f97649e2" format:"uuid"`
	Name     string    `json:"name"`
	Status   int       `json:"status" enums:"1,3"` // 1 - Created, 3 - Deleted
	Color    string    `json:"color"`
	IsMarker bool      `json:"isMarker"`
}

type EriusScenario struct {
	ID        uuid.UUID            `json:"id" example:"916ad995-8d13-49fb-82ee-edd4f97649e2" format:"uuid"`
	VersionID uuid.UUID            `json:"version_id" example:"916ad995-8d13-49fb-82ee-edd4f97649e2" format:"uuid"`
	Status    int                  `json:"status" enums:"1,2,3,4,5"` // 1 - Draft, 2 - Approved, 3 - Deleted, 4 - Rejected, 5 - On Approve
	HasDraft  bool                 `json:"hasDraft,omitempty"`
	Name      string               `json:"name" example:"ScenarioName"`
	Input     []EriusFunctionValue `json:"input,omitempty"`
	Output    []EriusFunctionValue `json:"output,omitempty"`
	Pipeline  struct {
		Entrypoint string               `json:"entrypoint"`
		Blocks     map[string]EriusFunc `json:"blocks"`
	} `json:"pipeline"`
	CreatedAt       *time.Time     `json:"created_at" example:"2020-07-16T17:10:25.112704+03:00"`
	ApprovedAt      *time.Time     `json:"approved_at" example:"2020-07-16T17:10:25.112704+03:00"`
	Author          string         `json:"author" example:"testAuthor"`
	Tags            []EriusTagInfo `json:"tags"`
	Comment         string         `json:"comment"`
	CommentRejected string         `json:"comment_rejected"`
}

type EriusFunctionList struct {
	Functions []script.FunctionModel `json:"funcs"`
	Shapes    []script.ShapeEntity   `json:"shapes"`
}

type EriusFunc struct {
	X          int                  `json:"x,omitempty"`
	Y          int                  `json:"y,omitempty"`
	TypeID     string               `json:"type_id" example:"approver"`
	BlockType  string               `json:"block_type" enums:"python3,go,internal,term,scenario" example:"python3"`
	Title      string               `json:"title" example:"lock-bts"`
	ShortTitle string               `json:"short_title,omitempty" example:"lock-bts"`
	Input      []EriusFunctionValue `json:"input,omitempty"`
	Output     []EriusFunctionValue `json:"output,omitempty"`
	ParamType  string               `json:"param_type,omitempty"`
	Params     json.RawMessage      `json:"params,omitempty" swaggertype:"object"`
	Next       map[string][]string  `json:"next,omitempty"`
	Sockets    []Socket             `json:"sockets,omitempty"`
}

type Socket struct {
	Id           string   `json:"id"`
	Title        string   `json:"title"`
	NextBlockIds []string `json:"nextBlockIds,omitempty"`
}

type EriusFunctionValue struct {
	Name   string `json:"name" example:"some_data"`
	Type   string `json:"type" example:"string"`
	Global string `json:"global,omitempty" example:"block.some_data"`
}

type UsageResponse struct {
	Name      string   `json:"name"` // Имя блока
	Used      bool     `json:"used"`
	Pipelines []UsedBy `json:"pipelines"`
}

type AllUsageResponse struct {
	Functions map[string][]string `json:"pipelines"`
}

type UsedBy struct {
	Name string    `json:"name"` // Имя сценария
	ID   uuid.UUID `json:"id"`   // ID сценария
}

type Shapes struct {
	Shapes []script.ShapeEntity `json:"shapes"`
}

type RunResponse struct {
	PipelineID uuid.UUID   `json:"pipeline_id" example:"916ad995-8d13-49fb-82ee-edd4f97649e2" format:"uuid"`
	WorkNumber string      `json:"work_number"`
	Status     string      `json:"status" example:"runned"`
	Output     interface{} `json:"output"`
	Errors     []string    `json:"errors"`
}

type SchedulerTasksResponse struct {
	Result bool `json:"result"`
}

type DebugResult struct {
	BlockName   string     `json:"block_name"`
	BlockStatus string     `json:"status" example:"run,error,finished,created"` // todo define values
	BreakPoints []string   `json:"break_points"`
	Task        *EriusTask `json:"task"`
}

type SearchPipeline struct {
	PipelineName string
	PipelineId   uuid.UUID `pg:"type:uuid" example:"916ad995-8d13-49fb-82ee-edd4f97649e2" format:"uuid"`
	Total        int
}

func ConvertSocket(sockets []Socket) []script.Socket {
	var result = make([]script.Socket, 0)

	for _, socket := range sockets {
		result = append(result, script.Socket{
			Id:           socket.Id,
			Title:        socket.Title,
			NextBlockIds: socket.NextBlockIds,
		})
	}

	return result
}
