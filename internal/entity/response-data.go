package entity

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/pkg/errors"

	"golang.org/x/exp/maps"

	"gitlab.services.mts.ru/abp/myosotis/logger"

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

type BlocksType map[string]*EriusFunc

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
	ParallelPathIntersected       = "ParallelPathIntersected"
)

func (bt *BlocksType) Validate(ctx context.Context, sd servicedesc.ServiceInterface, log logger.Logger) (valid bool, textErr string) {
	if !bt.EndExists(log) {
		return false, PipelineValidateError
	}

	ok, filledErr := bt.IsSocketsFilled(log)
	if !ok {
		return false, filledErr
	}

	if !bt.IsSdBlueprintFilled(ctx, sd) {
		log.WithField("funcName", "Validate").Error(errors.New("blueprint is not filled"))

		return false, PipelineValidateError
	}

	ok, parallErr := bt.IsParallelNodesCorrect(log)
	if !ok {
		return false, parallErr
	}

	return true, ""
}

func (bt *BlocksType) EndExists(log logger.Logger) bool {
	return bt.blockTypeExists(BlockGoStartName, log) && bt.blockTypeExists(BlockGoEndName, log)
}

//nolint:gocritic //
func (bt *BlocksType) IsSocketsFilled(log logger.Logger) (valid bool, textErr string) {
	for _, b := range *bt {
		if len(b.Next) != len(b.Sockets) {
			log.WithField("funcName", "IsSocketsFilled").Error(errors.New("sockets not connected"))

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
			if !nextNames[s.ID] {
				log.WithField("funcName", "IsSocketsFilled").
					Error(fmt.Errorf("socket %s %s not connected", s.ID, s.Title))

				return false, ""
			}
		}
	}

	return true, ""
}

func (bt *BlocksType) IsSdBlueprintFilled(ctx context.Context, sd servicedesc.ServiceInterface) bool {
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

	checkURL := sd.GetSdURL() + checkSdBlueprint + params.BlueprintID

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checkURL, http.NoBody)
	if err != nil {
		return false
	}

	resp, err := sd.GetCli().Do(req)
	if err != nil {
		return false
	}

	resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

//nolint:all //its ok here // не ок
func (bt *BlocksType) IsParallelNodesCorrect(log logger.Logger) (valid bool, textErr string) {
	return true, ""
	// TODO return Validation
	parallelStartNodes := bt.getNodesByType(BlockParallelStartName)
	if len(parallelStartNodes) == 0 {
		return true, ""
	}
	parallelExitsAsBlock := make(map[string]string, 0)
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
				nodes[socketOutNode] = socketNode
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
						nodes[socketOutNode] = socketNode
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

	intersectOk := bt.validateIntersectingPathParallelNodes(parallelStartNodes, parallelExitsAsBlock)
	if !intersectOk {
		return false, ParallelPathIntersected
	}

	return true, ""
}

//nolint:all //см TODO выше
func (bt *BlocksType) validateIntersectingPathParallelNodes(parallelStartNodes map[string]EriusFunc, parallelMap map[string]string) (valid bool) {
	for idx := range parallelStartNodes {
		parallelNode := parallelStartNodes[idx]

		nodes := make(map[string]*EriusFunc, 0)
		visitedParallelNodes := make(map[string]EriusFunc, 0)
		visitedParallelNodes[idx] = parallelNode

		for _, socketOutNodes := range parallelNode.Next {
			for _, socketOutNode := range socketOutNodes {
				_, ok := visitedParallelNodes[socketOutNode]
				if ok {
					return false
				}
				socketNode, ok := (*bt)[socketOutNode]

				if !ok {
					continue
				}

				nodes[socketOutNode] = socketNode

				visitedBranchNodes := make(map[string]EriusFunc, 0)

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
									nodes[socketOutBranchNode] = socketBranchNode
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
									nodes[socketOutBranchNode] = socketBranchNode
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

//nolint:all //см TODO выше
func (bt *BlocksType) validateAfterEndParallelNodes(endNode, idx *string,
	visitedParallelNodes map[string]EriusFunc,
) (valid bool, visitedNodes map[string]EriusFunc) {
	parallelEndNode := (*bt)[*endNode]
	afterEndNodes := map[string]*EriusFunc{
		*endNode: parallelEndNode,
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

				afterEndNodes[socketOutNode] = socketNode
			}
		}
	}

	return true, visitedEndParallelNodes
}

//nolint:all //см TODO выше
func (bt *BlocksType) validateBeforeStartParallelNodes(startKey, idx, endNode string,
	visitedParallelNodes, visitedAfterEndNodes map[string]EriusFunc,
) (valid bool, textErr string) {
	parallelStartNode := (*bt)[startKey]
	BeforeStartNodes := map[string]*EriusFunc{
		startKey: parallelStartNode,
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

				BeforeStartNodes[socketOutNode] = socketNode
			}
		}
	}

	return true, ""
}

func (bt *BlocksType) addDefaultStartNode() {
	(*bt)[StartBlock0] = &EriusFunc{
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
				ID:         "default",
				Title:      "Выход по умолчанию",
				ActionType: "",
			},
		},
	}
}

func (bt *BlocksType) GetExecutableFunctions() ([]script.FunctionParam, error) {
	functionIDs := make([]script.FunctionParam, 0)

	for key := range *bt {
		block := (*bt)[key]
		if block.TypeID == "executable_function" {
			var p script.ExecutableFunctionParams
			if err := json.Unmarshal(block.Params, &p); err != nil {
				return nil, err
			}

			if p.Function.FunctionID != "" {
				functionIDs = append(functionIDs, p.Function)
			}
		}
	}

	return functionIDs, nil
}

func (bt *BlocksType) blockTypeExists(blockType string, log logger.Logger) bool {
	if len(bt.getNodesByType(blockType)) == 0 {
		log.WithField("funcName", "blockTypeExists").Error(fmt.Errorf("%s does not exist", blockType))

		return false
	}

	return true
}

func (bt *BlocksType) getNodesByType(blockType string) map[string]EriusFunc {
	blocks := make(map[string]EriusFunc, 0)

	for id := range *bt {
		b := (*bt)[id]
		if b.TypeID == blockType {
			blocks[id] = *b
		}
	}

	return blocks
}

type PipelineType struct {
	Entrypoint string     `json:"entrypoint"`
	Blocks     BlocksType `json:"blocks"`
}

func (p *PipelineType) FillEmptyPipeline() {
	p.Blocks.addDefaultStartNode()
	p.Entrypoint = StartBlock0
}

func (p *PipelineType) ChangeOutput(keyOutputs map[string]string) {
	for block := range p.Blocks {
		if _, ok := keyOutputs[p.Blocks[block].TypeID]; !ok {
			continue
		}

		keyOutput := keyOutputs[p.Blocks[block].TypeID]
		outputBlock := p.Blocks[block].Output.Properties[keyOutput]
		outputBlock.Type = "object"
		outputBlock.Format = "SsoPerson"
		outputBlock.Properties = people.GetSsoPersonSchemaProperties()

		p.Blocks[block].Output.Properties[keyOutput] = outputBlock
	}
}

// nolint
type EriusScenario struct {
	PipelineID      uuid.UUID            `json:"id" example:"916ad995-8d13-49fb-82ee-edd4f97649e2" format:"uuid"`
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
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	NextBlockIds []string `json:"nextBlockIds,omitempty"`
	ActionType   string   `json:"actionType"`
}

type EriusFunctionValue struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Global string `json:"global,omitempty"`
	Format string `json:"format"`
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
	PipelineID   string
	Total        int
}

func ConvertSocket(sockets []Socket) []script.Socket {
	result := make([]script.Socket, 0)

	for _, socket := range sockets {
		result = append(result, script.Socket{
			ID:           socket.ID,
			Title:        socket.Title,
			NextBlockIds: socket.NextBlockIds,
		})
	}

	return result
}

const (
	KeyOutputWorkNumber           = "workNumber"
	KeyOutputApplicationInitiator = "initiator"
	KeyOutputApplicationBody      = "application_body"
)

func (es *EriusScenario) FillEntryPointOutput() (err error) {
	entryPoint := es.Pipeline.Blocks[es.Pipeline.Entrypoint]

	if entryPoint == nil || entryPoint.Output == nil || entryPoint.Output.Properties == nil {
		return nil
	}

	entryPoint.Output.Properties = script.JSONSchemaProperties{}

	if es.Settings.StartSchema != nil {
		entryPoint.Output.Properties[KeyOutputApplicationBody] = script.JSONSchemaPropertiesValue{
			Type:       "object",
			Global:     es.Pipeline.Entrypoint + "." + KeyOutputApplicationBody,
			Properties: es.Settings.StartSchema.Properties,
		}
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

type NodeGroup struct {
	EndNode   string       `json:"end_node"`
	Nodes     []*NodeGroup `json:"nodes"`
	Prev      string       `json:"prev"`
	StartNode string       `json:"start_node"`
}

// nolint
func (bt *BlocksType) GetGroups() (nodeGroups []*NodeGroup, err error) {
	startBlock := (*bt)[StartBlock0]
	if startBlock == nil {
		return nil, fmt.Errorf("%s not found", StartBlock0)
	}

	blocks := map[string]EriusFunc{
		StartBlock0: *startBlock,
	}
	visitedNodes := make(map[string]*EriusFunc, 0)
	prevNodeMap := make(map[string]string, 0)

	nodeGroups = make([]*NodeGroup, 0)

	for {
		nodeKeys := maps.Keys(blocks)
		if len(nodeKeys) == 0 {
			break
		}
		nodeKey, node := nodeKeys[0], blocks[nodeKeys[0]]
		delete(blocks, nodeKey)
		visitedNodes[nodeKey] = &node
		if node.TypeID == BlockParallelStartName {
			parallelGroup, exitParallelIdx, fillErr := bt.fillPrlGroups(nodeKey, prevNodeMap[nodeKey], 0, &node, visitedNodes)
			if fillErr != nil {
				return nil, fillErr
			}
			nodeGroups = append(nodeGroups, parallelGroup)
			endNode := (*bt)[exitParallelIdx]

			if endNode == nil {
				return nil, fmt.Errorf("end node for %s not found", nodeKey)
			}

			for _, socketOutNodes := range endNode.Next {
				for _, socketOutNode := range socketOutNodes {
					_, ok := visitedNodes[socketOutNode]
					if ok {
						continue
					}
					socketNode, ok := (*bt)[socketOutNode]
					if !ok {
						continue
					}
					blocks[socketOutNode] = *socketNode
					prevNodeMap[socketOutNode] = exitParallelIdx
				}
			}
		} else {
			nodeGroups = append(nodeGroups, &NodeGroup{
				EndNode:   nodeKey,
				Nodes:     nil,
				Prev:      prevNodeMap[nodeKey],
				StartNode: nodeKey,
			})
			for _, socketOutNodes := range node.Next {
				for _, socketOutNode := range socketOutNodes {
					_, ok := visitedNodes[socketOutNode]
					if ok {
						continue
					}
					socketNode, ok := (*bt)[socketOutNode]
					if !ok {
						continue
					}
					blocks[socketOutNode] = *socketNode
					prevNodeMap[socketOutNode] = nodeKey
				}
			}
		}
	}
	return nodeGroups, nil
}

// nolint
func (bt *BlocksType) fillPrlGroups(nodeKey, prev string, its int, bl *EriusFunc,
	visitedNodes map[string]*EriusFunc,
) (group *NodeGroup, exitIdx string, err error) {
	its++
	if its > 5 {
		return nil, "", errors.New("took too long")
	}

	blocks := map[string]*EriusFunc{
		nodeKey: bl,
	}

	prevNodeMap := map[string]string{
		nodeKey: prev,
	}

	group = &NodeGroup{
		EndNode:   "",
		Nodes:     []*NodeGroup{},
		Prev:      prev,
		StartNode: nodeKey,
	}

	for {
		startNodeKeys := maps.Keys(blocks)
		if len(startNodeKeys) == 0 {
			break
		}
		parallNodeKey, parallNode := startNodeKeys[0], blocks[startNodeKeys[0]]
		if parallNode.TypeID == BlockParallelEndName && len(startNodeKeys) != 1 {
			parallNodeKey, parallNode = startNodeKeys[1], blocks[startNodeKeys[1]]
		}
		delete(blocks, parallNodeKey)
		visitedNodes[parallNodeKey] = parallNode

		switch parallNode.TypeID {
		case BlockParallelEndName:
			group.Nodes = append(group.Nodes, &NodeGroup{
				EndNode:   parallNodeKey,
				Nodes:     nil,
				Prev:      "",
				StartNode: parallNodeKey,
			})
			group.EndNode = parallNodeKey
			exitIdx = parallNodeKey

			return
		case BlockParallelStartName:
			if parallNodeKey != nodeKey {
				newGroup, extIdx, fillErr := bt.fillPrlGroups(parallNodeKey, prevNodeMap[parallNodeKey], its, parallNode, visitedNodes)
				if fillErr != nil {
					return nil, "", fillErr
				}
				group.Nodes = append(group.Nodes, newGroup)
				endNode := (*bt)[extIdx]
				for _, socketOutNodes := range endNode.Next {
					for _, socketOutNode := range socketOutNodes {
						_, ok := visitedNodes[socketOutNode]
						if ok {
							continue
						}
						socketNode, ok := (*bt)[socketOutNode]
						if !ok {
							continue
						}
						blocks[socketOutNode] = socketNode
						prevNodeMap[socketOutNode] = extIdx
					}
				}
			} else {
				group.Nodes = append(group.Nodes, &NodeGroup{
					EndNode:   parallNodeKey,
					Nodes:     nil,
					Prev:      prevNodeMap[parallNodeKey],
					StartNode: parallNodeKey,
				})
				for _, socketOutNodes := range parallNode.Next {
					for _, socketOutNode := range socketOutNodes {
						_, ok := visitedNodes[socketOutNode]
						if ok {
							continue
						}
						socketNode, ok := (*bt)[socketOutNode]
						if !ok {
							continue
						}
						blocks[socketOutNode] = socketNode
						prevNodeMap[socketOutNode] = parallNodeKey
					}
				}
			}
		default:
			group.Nodes = append(group.Nodes, &NodeGroup{
				EndNode:   parallNodeKey,
				Nodes:     nil,
				Prev:      prevNodeMap[parallNodeKey],
				StartNode: parallNodeKey,
			})
			for _, socketOutNodes := range parallNode.Next {
				for _, socketOutNode := range socketOutNodes {
					_, ok := visitedNodes[socketOutNode]
					if ok {
						continue
					}
					socketNode, ok := (*bt)[socketOutNode]
					if !ok {
						continue
					}
					blocks[socketOutNode] = socketNode
					prevNodeMap[socketOutNode] = parallNodeKey
				}
			}
		}
	}
	return group, exitIdx, nil
}
