package entity

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"golang.org/x/exp/maps"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/people"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/servicedesc"
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

type BlocksType map[string]EriusFunc

const (
	BlockGoStartName       = "start"
	StartBlock0            = "start_0"
	BlockGoEndName         = "end"
	BlockSDName            = "servicedesk_application"
	BlockParallelStartName = "begin_parallel_task"
	BlockParallelEndName   = "wait_for_all_inputs"
)

const (
	checkSdBlueprint = "/api/herald/v1/schema/blueprint/"
)

const (
	PipelineValidateError         = "PipelineValidateError"
	ParallelNodeReturnCycle       = "ParallelNodeReturnCycle"
	ParallelNodeExitsNotConnected = "ParallelNodeExitsNotConnected"
	OutOfParallelNodesConnection  = "OutOfParallelNodesConnection"
	ParallelOutOfStartInsert      = "ParallelOutOfStartInsert"
	ParallelPathMixed             = "ParallelPathMixed"
)

func (bt *BlocksType) Validate(ctx context.Context, sd *servicedesc.Service) (valid bool, textErr string) {
	if !bt.EndExists() {
		return false, PipelineValidateError
	}

	if !bt.IsPipelineComplete() {
		return false, PipelineValidateError
	}

	ok, filledErr := bt.IsSocketsFilled()
	if !ok {
		return false, filledErr
	}

	if !bt.IsSdBlueprintFilled(ctx, sd) {
		return false, PipelineValidateError
	}
	ok, parallErr := bt.IsParallelNodesCorrect()
	if !ok {
		return false, parallErr
	}

	return true, ""
}

func (bt *BlocksType) EndExists() bool {
	return bt.blockTypeExists(BlockGoStartName) && bt.blockTypeExists(BlockGoEndName)
}

func (bt *BlocksType) IsPipelineComplete() bool {
	startNodes := bt.getNodesByType(BlockGoStartName)

	if len(startNodes) == 0 {
		return false
	}
	startNode := startNodes[maps.Keys(startNodes)[0]]

	nodesIds := bt.getNodesIds()
	relatedNodesNum := bt.countRelatedNodesIds(&startNode)

	return len(nodesIds) == relatedNodesNum
}

func (bt *BlocksType) IsSocketsFilled() (valid bool, textErr string) {
	for _, b := range *bt {
		if len(b.Next) != len(b.Sockets) {
			return false, ParallelNodeExitsNotConnected
		}

		nextNames := make(map[string]bool)
		for n, v := range b.Next {
			if len(v) == 0 {
				continue
			}
			nextNames[n] = true
		}

		for _, s := range b.Sockets {
			if !nextNames[s.Id] {
				return false, ""
			}
		}
	}
	return true, ""
}

func (bt *BlocksType) IsSdBlueprintFilled(ctx context.Context, sd *servicedesc.Service) bool {
	sdNodes := bt.getNodesByType(BlockSDName)
	if len(sdNodes) == 0 {
		return true
	}
	sdNode := sdNodes[maps.Keys(sdNodes)[0]]

	var params script.SdApplicationParams
	err := json.Unmarshal(sdNode.Params, &params)
	if err != nil {
		return false
	}
	checkUrl := sd.SdURL + checkSdBlueprint + params.BlueprintID
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checkUrl, http.NoBody)
	if err != nil {
		return false
	}
	resp, err := sd.Cli.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// nolint:gocognit //its ok here
func (bt *BlocksType) IsParallelNodesCorrect() (valid bool, textErr string) {
	parallelStartNodes := bt.getNodesByType(BlockParallelStartName)
	if len(parallelStartNodes) == 0 {
		return true, ""
	}
	var parallelExitsAsBlock = make(map[string]string, 0)
	for idx := range parallelStartNodes {
		parallelNode := parallelStartNodes[idx]
		var foundNode *string

		nodes := make(map[string]*EriusFunc, 0)
		visitedParallelNodes := make(map[string]EriusFunc, 0)
		visitedParallelNodes[idx] = parallelNode

		for _, socketOutNodes := range parallelNode.Next {
			for _, socketOutNode := range socketOutNodes {
				socketNode, ok := (*bt)[socketOutNode]
				if !ok {
					continue
				}
				nodes[socketOutNode] = &socketNode
			}
		}

		for {
			nodeKeys := maps.Keys(nodes)
			if len(nodeKeys) == 0 {
				break
			}

			nodeKey, node := nodeKeys[0], nodes[nodeKeys[0]]
			delete(nodes, nodeKey)
			if _, ok := visitedParallelNodes[nodeKey]; ok {
				continue
			}

			visitedParallelNodes[nodeKey] = *node
			if node.TypeID == BlockParallelEndName {
				if foundNode != nil && nodeKey != *foundNode {
					return false, PipelineValidateError
				}
				foundNode = &nodeKey
			} else if node.TypeID == BlockParallelStartName {
				continue
			} else {
				for _, socketOutNodes := range node.Next {
					for _, socketOutNode := range socketOutNodes {
						socketNode, ok := (*bt)[socketOutNode]
						if !ok {
							continue
						}
						if socketOutNode == idx {
							return false, ParallelNodeReturnCycle
						}
						nodes[socketOutNode] = &socketNode
					}
				}
			}
		}
		if foundNode == nil {
			return false, ""
		}
		afterEndOk, visitedEndNodes := bt.validateAfterEndParallelNodes(foundNode, &idx, visitedParallelNodes)
		if !afterEndOk {
			return false, OutOfParallelNodesConnection
		}
		if beforeStartOk, textStartErr := bt.validateBeforeStartParallelNodes(StartBlock0, idx, *foundNode, visitedParallelNodes, visitedEndNodes); !beforeStartOk {
			return false, textStartErr
		}
		parallelExitsAsBlock[idx] = *foundNode
	}
	mixOk := bt.validateIntersectingPathParallelNodes(parallelStartNodes, parallelExitsAsBlock)
	if !mixOk {
		return false, ParallelPathMixed
	}
	return true, ""
}

// nolint
func (bt *BlocksType) validateIntersectingPathParallelNodes(parallelStartNodes map[string]EriusFunc, parallelMap map[string]string) (valid bool) {
	for idx := range parallelStartNodes {
		parallelNode := parallelStartNodes[idx]

		nodes := make(map[string]*EriusFunc, 0)
		visitedParallelNodes := make(map[string]EriusFunc, 0)
		visitedParallelNodes[idx] = parallelNode

		for _, socketOutNodes := range parallelNode.Next {
			for _, socketOutNode := range socketOutNodes {
				socketNode, ok := (*bt)[socketOutNode]
				if !ok {
					continue
				}
				nodes[socketOutNode] = &socketNode

				var visitedBranchNodes = make(map[string]EriusFunc, 0)

				for {
					nodeKeys := maps.Keys(nodes)
					if len(nodeKeys) == 0 {
						break
					}

					nodeKey, node := nodeKeys[0], nodes[nodeKeys[0]]
					delete(nodes, nodeKey)
					if _, ok := visitedParallelNodes[nodeKey]; ok {
						continue
					}

					visitedParallelNodes[nodeKey] = *node
					visitedBranchNodes[nodeKey] = *node
					switch node.TypeID {
					case BlockParallelEndName:
						continue
					case BlockParallelStartName:
						{
							nodeParallEndKey := parallelMap[nodeKey]
							nodeParallEnd := (*bt)[nodeParallEndKey]
							for _, socketOutBranchNodes := range nodeParallEnd.Next {
								for _, socketOutBranchNode := range socketOutBranchNodes {
									if socketOutBranchNode == parallelMap[idx] {
										continue
									}
									_, okParallel := visitedParallelNodes[socketOutBranchNode]
									_, okBranch := visitedBranchNodes[socketOutBranchNode]
									if okParallel && !okBranch {
										return false
									}
									socketBranchNode, ok := (*bt)[socketOutBranchNode]
									if !ok {
										continue
									}
									nodes[socketOutBranchNode] = &socketBranchNode
								}
							}
						}
					default:
						{
							for _, socketOutBranchNodes := range node.Next {
								for _, socketOutBranchNode := range socketOutBranchNodes {
									if socketOutBranchNode == parallelMap[idx] {
										continue
									}
									_, okParallel := visitedParallelNodes[socketOutBranchNode]
									_, okBranch := visitedBranchNodes[socketOutBranchNode]
									if okParallel && !okBranch {
										return false
									}
									socketBranchNode, ok := (*bt)[socketOutBranchNode]
									if !ok {
										continue
									}
									nodes[socketOutBranchNode] = &socketBranchNode
								}
							}
						}
					}
				}
			}
		}
	}
	return true
}

func (bt *BlocksType) validateAfterEndParallelNodes(endNode, idx *string,
	visitedParallelNodes map[string]EriusFunc) (valid bool, visitedNodes map[string]EriusFunc) {
	parallelEndNode := (*bt)[*endNode]
	afterEndNodes := map[string]*EriusFunc{
		*endNode: &parallelEndNode,
	}
	visitedEndParallelNodes := make(map[string]EriusFunc, 0)

	for {
		endNodeKeys := maps.Keys(afterEndNodes)
		if len(endNodeKeys) == 0 {
			break
		}
		nodeKey, node := endNodeKeys[0], afterEndNodes[endNodeKeys[0]]
		delete(afterEndNodes, nodeKey)
		if _, ok := visitedEndParallelNodes[nodeKey]; ok {
			continue
		}
		visitedEndParallelNodes[nodeKey] = *node

		for _, socketOutNodes := range node.Next {
			for _, socketOutNode := range socketOutNodes {
				if socketOutNode == *idx {
					continue
				}
				_, ok := visitedParallelNodes[socketOutNode]
				if ok {
					return false, nil
				}
				socketNode, ok := (*bt)[socketOutNode]
				if !ok {
					continue
				}
				afterEndNodes[socketOutNode] = &socketNode
			}
		}
	}
	return true, visitedEndParallelNodes
}

func (bt *BlocksType) validateBeforeStartParallelNodes(startKey, idx, endNode string,
	visitedParallelNodes, visitedAfterEndNodes map[string]EriusFunc) (valid bool, textErr string) {
	parallelStartNode := (*bt)[startKey]
	BeforeStartNodes := map[string]*EriusFunc{
		startKey: &parallelStartNode,
	}
	visitedBeforStartParallelNodes := make(map[string]EriusFunc, 0)

	for {
		startNodeKeys := maps.Keys(BeforeStartNodes)
		if len(startNodeKeys) == 0 {
			break
		}
		nodeKey, node := startNodeKeys[0], BeforeStartNodes[startNodeKeys[0]]
		delete(BeforeStartNodes, nodeKey)
		if _, ok := visitedBeforStartParallelNodes[nodeKey]; ok {
			continue
		}
		visitedBeforStartParallelNodes[nodeKey] = *node

		for _, socketOutNodes := range node.Next {
			for _, socketOutNode := range socketOutNodes {
				if socketOutNode == idx {
					continue
				}
				if socketOutNode == endNode {
					return false, ParallelOutOfStartInsert
				}
				_, ok := visitedParallelNodes[socketOutNode]
				if ok {
					return false, OutOfParallelNodesConnection
				}
				_, alreadyVisited := visitedAfterEndNodes[socketOutNode]
				if alreadyVisited {
					continue
				}
				socketNode, ok := (*bt)[socketOutNode]
				if !ok {
					continue
				}
				BeforeStartNodes[socketOutNode] = &socketNode
			}
		}
	}
	return true, ""
}

func (bt *BlocksType) addDefaultStartNode() {
	(*bt)[StartBlock0] = EriusFunc{
		X:         0,
		Y:         0,
		TypeID:    BlockGoStartName,
		BlockType: script.TypeGo,
		Title:     "Начало",
		Output: &script.JSONSchema{
			Type: "object",
			Properties: script.JSONSchemaProperties{
				KeyOutputWorkNumber: {
					Type:   "string",
					Global: "start_0.workNumber",
				},
				KeyOutputApplicationInitiator: {
					Global:     "start_0.initiator",
					Type:       "object",
					Format:     "SsoPerson",
					Properties: people.GetSsoPersonSchemaProperties(),
				},
			},
		},
		Sockets: []Socket{
			{
				Id:         "default",
				Title:      "Выход по умолчанию",
				ActionType: "",
			},
		},
	}
}

func (bt *BlocksType) blockTypeExists(blockType string) bool {
	return len(bt.getNodesByType(blockType)) != 0
}

func (bt *BlocksType) getNodesByType(blockType string) map[string]EriusFunc {
	blocks := make(map[string]EriusFunc, 0)
	for id := range *bt {
		b := (*bt)[id]
		if b.TypeID == blockType {
			blocks[id] = b
		}
	}
	return blocks
}

func (bt *BlocksType) getNodeByID(blockId string) *EriusFunc {
	for blockKey, _ := range *bt {
		if blockKey == blockId {
			block := (*bt)[blockKey]

			return &block
		}
	}
	return nil
}

func (bt *BlocksType) getNodesIds() (res []string) {
	for k := range *bt {
		res = append(res, k)
	}
	return res
}

func (bt *BlocksType) countRelatedNodesIds(startNode *EriusFunc) (res int) {
	nodes := make([]*EriusFunc, 0)
	visited := make(map[string]bool)

	currentNode := startNode
	res++

	for {
		for _, s := range currentNode.Sockets {
			for _, blockId := range s.NextBlockIds {
				socketNode := (*bt)[blockId]

				if !visited[blockId] {
					visited[blockId] = true
					nodes = append(nodes, &socketNode)
					res++
				}
			}
		}

		if len(nodes) == 0 {
			break
		}

		currentNode = nodes[0]
		nodes = nodes[1:]
	}

	return res
}

type PipelineType struct {
	Entrypoint string     `json:"entrypoint"`
	Blocks     BlocksType `json:"blocks"`
}

func (p *PipelineType) FillEmptyPipeline() {
	p.Blocks.addDefaultStartNode()
	p.Entrypoint = StartBlock0
}

// nolint
type EriusScenario struct {
	ID              uuid.UUID            `json:"id" example:"916ad995-8d13-49fb-82ee-edd4f97649e2" format:"uuid"`
	VersionID       uuid.UUID            `json:"version_id" example:"916ad995-8d13-49fb-82ee-edd4f97649e2" format:"uuid"`
	Status          int                  `json:"status" enums:"1,2,3,4,5"` // 1 - Draft, 2 - Approved, 3 - Deleted, 4 - Rejected, 5 - On Approve
	HasDraft        bool                 `json:"hasDraft,omitempty"`
	Name            string               `json:"name" example:"ScenarioName"`
	Input           []EriusFunctionValue `json:"input,omitempty"`
	Output          []EriusFunctionValue `json:"output,omitempty"`
	Settings        ProcessSettings      `json:"process_settings"`
	Pipeline        PipelineType         `json:"pipeline"`
	CreatedAt       *time.Time           `json:"created_at" example:"2020-07-16T17:10:25.112704+03:00"`
	ApprovedAt      *time.Time           `json:"approved_at" example:"2020-07-16T17:10:25.112704+03:00"`
	Author          string               `json:"author" example:"testAuthor"`
	Tags            []EriusTagInfo       `json:"tags"`
	Comment         string               `json:"comment"`
	CommentRejected string               `json:"comment_rejected"`
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
	Output     *script.JSONSchema   `json:"output,omitempty"`
	ParamType  string               `json:"param_type,omitempty"`
	Params     json.RawMessage      `json:"params,omitempty" swaggertype:"object"`
	Next       map[string][]string  `json:"next,omitempty"`
	Sockets    []Socket             `json:"sockets,omitempty"`
}

type Socket struct {
	Id           string   `json:"id"`
	Title        string   `json:"title"`
	NextBlockIds []string `json:"nextBlockIds,omitempty"`
	ActionType   string   `json:"actionType"`
}

type EriusFunctionValue struct {
	Name   string `json:"name" example:"some_data"`
	Type   string `json:"type" example:"string"`
	Global string `json:"global,omitempty" example:"block.some_data"`
	Format string `json:"format" example:"string"`
}

type NodeSubscriptionEvents struct {
	NodeID string   `json:"node_id"`
	Notify bool     `json:"notify"`
	Events []string `json:"events"`
}

type ExternalSystemSubscriptionParams struct {
	SystemID           string                      `json:"system_id"`
	MicroserviceID     string                      `json:"microservice_id"`
	Path               string                      `json:"path"`
	Method             string                      `json:"method"`
	NotificationSchema script.JSONSchema           `json:"notification_schema"`
	Mapping            script.JSONSchemaProperties `json:"mapping"`
	Nodes              []NodeSubscriptionEvents    `json:"nodes"`
}

type ProcessSettingsWithExternalSystems struct {
	ExternalSystems    []ExternalSystem                   `json:"external_systems"`
	ProcessSettings    ProcessSettings                    `json:"process_settings"`
	TasksSubscriptions []ExternalSystemSubscriptionParams `json:"tasks_subscriptions"`
}

type ProcessSettings struct {
	Id                 string             `json:"version_id"`
	StartSchema        *script.JSONSchema `json:"start_schema"`
	EndSchema          *script.JSONSchema `json:"end_schema"`
	ResubmissionPeriod int                `json:"resubmission_period"`
	Name               string             `json:"name"`
	SLA                int                `json:"sla"`
	WorkType           string             `json:"work_type"`

	StartSchemaRaw []byte `json:"-"`
	EndSchemaRaw   []byte `json:"-"`
}

func (ps *ProcessSettings) UnmarshalJSON(bytes []byte) error {
	temp := struct {
		Id                 string           `json:"version_id"`
		StartSchema        *json.RawMessage `json:"start_schema"`
		EndSchema          *json.RawMessage `json:"end_schema"`
		ResubmissionPeriod int              `json:"resubmission_period"`
		Name               string           `json:"name"`
		SLA                int              `json:"sla"`
		WorkType           string           `json:"work_type"`
	}{}

	if err := json.Unmarshal(bytes, &temp); err != nil {
		return err
	}

	ps.Id = temp.Id
	ps.ResubmissionPeriod = temp.ResubmissionPeriod
	ps.Name = temp.Name
	ps.SLA = temp.SLA
	ps.WorkType = temp.WorkType

	if temp.StartSchema != nil {
		ps.StartSchemaRaw = *temp.StartSchema
	}
	if temp.EndSchema != nil {
		ps.EndSchemaRaw = *temp.EndSchema
	}
	return nil
}

func (ps *ProcessSettings) ValidateSLA() bool {
	if (ps.WorkType == "8/5" || ps.WorkType == "24/7" || ps.WorkType == "12/5") && ps.SLA > 0 {
		return true
	}
	return false
}

type ExternalSystem struct {
	Id   string `json:"system_id"`
	Name string `json:"name,omitempty"`

	InputSchema   *script.JSONSchema `json:"input_schema,omitempty"`
	OutputSchema  *script.JSONSchema `json:"output_schema,omitempty"`
	InputMapping  *script.JSONSchema `json:"input_mapping,omitempty"`
	OutputMapping *script.JSONSchema `json:"output_mapping,omitempty"`

	OutputSettings *EndSystemSettings `json:"output_settings,omitempty"`

	AllowRunAsOthers bool `json:"allow_run_as_others"`
}

type EndSystemSettings struct {
	URL            string `json:"URL"`
	Method         string `json:"method"`
	MicroserviceId string `json:"microservice_id"`
}

type SlaVersionSettings struct {
	Author   string `json:"author"`
	WorkType string `json:"work_type"`
	Sla      int    `json:"sla"`
}

type EndProcessData struct {
	Id         string `json:"id"`
	VersionId  string `json:"version_id"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at"`
	Status     string `json:"status"`
}

func (ps ProcessSettings) Validate() error {
	err := ps.StartSchema.Validate()
	if err != nil {
		return err
	}

	err = ps.EndSchema.Validate()
	if err != nil {
		return err
	}

	return nil
}

func (es ExternalSystem) ValidateSchemas() error {
	err := es.InputSchema.Validate()
	if err != nil {
		return err
	}

	err = es.OutputSchema.Validate()
	if err != nil {
		return err
	}

	err = es.InputMapping.Validate()
	if err != nil {
		return err
	}

	err = es.OutputMapping.Validate()
	if err != nil {
		return err
	}

	return nil
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
	PipelineId   string
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

const (
	KeyOutputWorkNumber           = "workNumber"
	KeyOutputApplicationInitiator = "initiator"
)

func (es EriusScenario) FillEntryPointOutput() (err error) {
	entryPoint := es.Pipeline.Blocks[es.Pipeline.Entrypoint]
	if es.Settings.StartSchema != nil {
		for k := range entryPoint.Output.Properties {
			val, ok := es.Settings.StartSchema.Properties[k]
			if !ok {
				continue
			}
			val.Global = es.Pipeline.Entrypoint + "." + k
			es.Settings.StartSchema.Properties[k] = val
		}
		entryPoint.Output = es.Settings.StartSchema
	}

	entryPoint.Output.Properties[KeyOutputWorkNumber] = script.JSONSchemaPropertiesValue{
		Type:   "string",
		Global: es.Pipeline.Entrypoint + "." + KeyOutputWorkNumber,
	}

	entryPoint.Output.Properties[KeyOutputApplicationInitiator] = script.JSONSchemaPropertiesValue{
		Global:     es.Pipeline.Entrypoint + "." + KeyOutputApplicationInitiator,
		Type:       "object",
		Format:     "SsoPerson",
		Properties: people.GetSsoPersonSchemaProperties(),
	}

	es.Pipeline.Blocks[es.Pipeline.Entrypoint] = entryPoint

	return nil
}
