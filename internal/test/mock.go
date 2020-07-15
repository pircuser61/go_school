package test

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"gitlab.services.mts.ru/erius/pipeliner/internal/script"

	"github.com/google/uuid"
	"gitlab.services.mts.ru/erius/pipeliner/internal/db"
	"gitlab.services.mts.ru/erius/pipeliner/internal/entity"
)

func OnlyReturnBlockGenerator(ret map[string]interface{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-type", "application/json")
		w.WriteHeader(200)

		bytesOutput, _ := json.Marshal(ret)
		_, _ = w.Write(bytesOutput)
	}
}

type TestablePipeline struct {
	FunctionHandlers map[string]http.HandlerFunc
	PipelineUUID     uuid.UUID
	pipeline         *entity.EriusScenario
}

var (
	LinearPipelineBlock = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		type InputStruct struct {
			Input string `json:"Input"`
		}
		type OutputStruct struct {
			Output string `json:"Output"`
		}

		w.Header().Set("Content-type", "application/json")

		bytes, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(500)
			return
		}

		var jsonInput InputStruct

		err = json.Unmarshal(bytes, &jsonInput)
		if err != nil {
			w.WriteHeader(500)
			return
		}

		bytesOutput, err := json.Marshal(OutputStruct{Output: jsonInput.Input})
		if err != nil {
			w.WriteHeader(500)
			return
		}

		w.WriteHeader(200)
		_, _ = w.Write(bytesOutput)
	})
	StringIsEqualToBlockGenerator = func(equalsTo string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			type InputStruct struct {
				Input string `json:"Input"`
			}
			type OutputStruct struct {
				Output bool `json:"Output"`
			}

			w.Header().Set("Content-type", "application/json")

			bytes, err := ioutil.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(500)
				return
			}

			var jsonInput InputStruct

			err = json.Unmarshal(bytes, &jsonInput)
			if err != nil {
				w.WriteHeader(500)
				return
			}

			bytesOutput, err := json.Marshal(OutputStruct{Output: jsonInput.Input == equalsTo})
			if err != nil {
				w.WriteHeader(500)
				return
			}

			w.WriteHeader(200)
			_, _ = w.Write(bytesOutput)
		}
	}
	EmptyBlock = OnlyReturnBlockGenerator(map[string]interface{}{})

	LinearPipelineUUID        = uuid.New()
	LinearPipelineVersionUUID = uuid.New()

	IfPipelineUUID        = uuid.New()
	IfPipelineVersionUUID = uuid.New()

	ForPipelineUUID        = uuid.New()
	ForPipelineVersionUUID = uuid.New()

	PipelineWithPipelineUUID        = uuid.New()
	PipelineWithPipelineVersionUUID = uuid.New()

	ForInForPipelineUUID        = uuid.New()
	ForInForPipelineVersionUUID = uuid.New()

	StringsEqualPipelineUUID        = uuid.New()
	StringsEqualPipelineVersionUUID = uuid.New()

	ConnectorPipelineUUID        = uuid.New()
	ConnectorPipelineVersionUUID = uuid.New()

	linearPipeline = entity.EriusScenario{
		ID:        LinearPipelineUUID,
		VersionID: LinearPipelineVersionUUID,
		Status:    db.StatusApproved,
		HasDraft:  false,
		Name:      "LinearPipeline",
		Input: []entity.EriusFunctionValue{
			{
				Name:   "Input",
				Type:   "string",
				Global: "LinearPipeline.Input",
			},
		},
		Output: []entity.EriusFunctionValue{
			{
				Name:   "Output",
				Type:   "string",
				Global: "Block3.Output",
			},
		},
		Pipeline: struct {
			Entrypoint string                      `json:"entrypoint"`
			Blocks     map[string]entity.EriusFunc `json:"blocks"`
		}{
			Entrypoint: "Block1",
			Blocks: map[string]entity.EriusFunc{
				"Block1": {
					BlockType: script.TypePython3,
					Title:     "Block1",
					Input: []entity.EriusFunctionValue{
						{
							Name:   "Input",
							Type:   "string",
							Global: "LinearPipeline.Input",
						},
					},
					Output: []entity.EriusFunctionValue{
						{
							Name:   "Output",
							Type:   "string",
							Global: "Block1.Output",
						},
					},
					Next: "Block2",
				},
				"Block2": {
					BlockType: script.TypePython3,
					Title:     "Block2",
					Input: []entity.EriusFunctionValue{
						{
							Name:   "Input",
							Type:   "string",
							Global: "Block1.Output",
						},
					},
					Output: []entity.EriusFunctionValue{
						{
							Name:   "Output",
							Type:   "string",
							Global: "Block2.Output",
						},
					},
					Next: "Block3",
				},
				"Block3": {
					BlockType: script.TypePython3,
					Title:     "Block3",
					Input: []entity.EriusFunctionValue{
						{
							Name:   "Input",
							Type:   "string",
							Global: "Block2.Output",
						},
					},
					Output: []entity.EriusFunctionValue{
						{
							Name:   "Output",
							Type:   "string",
							Global: "Block3.Output",
						},
					},
					Next: "",
				},
			},
		},
	}

	// Pipeline accept {"Input":"string"} and return {"Output":"string"}
	// Input string goes through pipeline
	// Block1, Block2, Block3 should accept {"Input":"string"} and return {"Output":"string"}
	// Test should check for block running sequence and block input
	LinearPipelineTestable = TestablePipeline{
		FunctionHandlers: map[string]http.HandlerFunc{
			"Block1": LinearPipelineBlock,
			"Block2": LinearPipelineBlock,
			"Block3": LinearPipelineBlock,
		},
		PipelineUUID: LinearPipelineUUID,
		pipeline:     &linearPipeline,
	}

	ifPipeline = entity.EriusScenario{
		ID:        IfPipelineUUID,
		VersionID: IfPipelineVersionUUID,
		Status:    db.StatusApproved,
		HasDraft:  false,
		Name:      "IfPipeline",
		Input: []entity.EriusFunctionValue{
			{
				Name:   "Input",
				Type:   script.TypeString,
				Global: "IfPipeline.Input",
			},
		},
		Pipeline: struct {
			Entrypoint string                      `json:"entrypoint"`
			Blocks     map[string]entity.EriusFunc `json:"blocks"`
		}{
			Entrypoint: "Block1",
			Blocks: map[string]entity.EriusFunc{
				"Block1": {
					BlockType: script.TypePython3,
					Title:     "Block1",
					Input: []entity.EriusFunctionValue{
						{
							Name:   "Input",
							Type:   script.TypeString,
							Global: "IfPipeline.Input",
						},
					},
					Output: []entity.EriusFunctionValue{
						{
							Name:   "Output",
							Type:   script.TypeBool,
							Global: "Block1.Output",
						},
					},
					Next: "BlockIf",
				},
				"BlockIf": {
					BlockType: script.TypeInternal,
					Title:     "if",
					Input: []entity.EriusFunctionValue{
						{
							Name:   "check",
							Type:   script.TypeBool,
							Global: "Block1.Output",
						},
					},
					OnTrue:  "BlockTrue",
					OnFalse: "BlockFalse",
				},
				"BlockTrue": {
					BlockType: script.TypePython3,
					Title:     "BlockTrue",
					Output: []entity.EriusFunctionValue{
						{
							Name:   "Output",
							Type:   script.TypeString,
							Global: "BlockTrue.Output",
						},
					},
					Next: "",
				},
				"BlockFalse": {
					BlockType: script.TypePython3,
					Title:     "BlockFalse",
					Output: []entity.EriusFunctionValue{
						{
							Name:   "Output",
							Type:   script.TypeString,
							Global: "BlockTrue.Output",
						},
					},
					Next: "",
				},
			},
		},
	}

	// Pipeline accept {"Input":"string"} and returns none
	// Block1 should compare pipeline input with something inside and return bool
	// Depending on Block1.Output runs BlockTrue or BlockFalse
	// Test should check for block running sequence
	IfPipelineTestable = TestablePipeline{
		FunctionHandlers: map[string]http.HandlerFunc{
			"Block1":     StringIsEqualToBlockGenerator("Value"),
			"BlockTrue":  OnlyReturnBlockGenerator(map[string]interface{}{"Output": "true"}),
			"BlockFalse": OnlyReturnBlockGenerator(map[string]interface{}{"Output": "false"}),
		},
		PipelineUUID: IfPipelineUUID,
		pipeline:     &ifPipeline,
	}

	forPipeline = entity.EriusScenario{
		ID:        ForPipelineUUID,
		VersionID: ForPipelineVersionUUID,
		Status:    db.StatusApproved,
		HasDraft:  false,
		Name:      "ForPipeline",
		Input: []entity.EriusFunctionValue{
			{
				Name:   "Input",
				Type:   script.TypeNumber,
				Global: "ForPipeline.Input",
			},
		},
		Output: []entity.EriusFunctionValue{},
		Pipeline: struct {
			Entrypoint string                      `json:"entrypoint"`
			Blocks     map[string]entity.EriusFunc `json:"blocks"`
		}{
			Entrypoint: "Block1",
			Blocks: map[string]entity.EriusFunc{
				"Block1": {
					BlockType: script.TypePython3,
					Title:     "Block1",
					Input: []entity.EriusFunctionValue{
						{
							Name:   "Input",
							Type:   script.TypeNumber,
							Global: "Block1.Input",
						},
					},
					Output: []entity.EriusFunctionValue{
						{
							Name:   "Output",
							Type:   script.TypeArray,
							Global: "Block1.Output",
						},
					},
					Next: "For",
				},
				"For": {
					BlockType: script.TypeInternal,
					Title:     "for",
					Input: []entity.EriusFunctionValue{
						{
							Name:   "iter",
							Type:   script.TypeArray,
							Global: "Block1.Output",
						},
					},
					Output: []entity.EriusFunctionValue{
						{
							Name:   "index",
							Type:   script.TypeNumber,
							Global: "For.index",
						},
						{
							Name:   "now_on",
							Type:   script.TypeString,
							Global: "For.now_on",
						},
					},
					OnTrue:  "Block3", // done
					OnFalse: "Block2", // iteration
				},
				"Block2": {
					BlockType: script.TypePython3,
					Title:     "Block2",
					Input:     nil,
					Output:    nil,
					Next:      "For",
				},
				"Block3": {
					BlockType: script.TypePython3,
					Title:     "Block3",
					Input:     []entity.EriusFunctionValue{},
					Output:    []entity.EriusFunctionValue{},
					Next:      "",
				},
			},
		},
	}

	// Pipeline accept {"Input":123} and returns none
	// Block1 generates array
	// For every item in array run Block2
	// After loop run Block3
	// Test should check for block running sequence
	ForPipelineTestable = TestablePipeline{
		FunctionHandlers: map[string]http.HandlerFunc{
			"Block1": OnlyReturnBlockGenerator(map[string]interface{}{"Output": []string{"1", "2", "3"}}),
			"Block2": EmptyBlock,
			"Block3": EmptyBlock,
		},
		PipelineUUID: ForPipelineUUID,
		pipeline:     &forPipeline,
	}

	pipelineWithPipeline = entity.EriusScenario{
		ID:        PipelineWithPipelineUUID,
		VersionID: PipelineWithPipelineVersionUUID,
		Status:    db.StatusApproved,
		HasDraft:  false,
		Name:      "PipelineWithPipeline",
		Input: []entity.EriusFunctionValue{
			{
				Name:   "Input",
				Type:   script.TypeString,
				Global: "PipelineWithPipeline.Input",
			},
		},
		Output: []entity.EriusFunctionValue{
			{
				Name:   "Output",
				Type:   script.TypeString,
				Global: "Block2.Output",
			},
		},
		Pipeline: struct {
			Entrypoint string                      `json:"entrypoint"`
			Blocks     map[string]entity.EriusFunc `json:"blocks"`
		}{
			Entrypoint: "Block1",
			Blocks: map[string]entity.EriusFunc{
				"Block1": {
					BlockType: script.TypePython3,
					Title:     "Block1",
					Input: []entity.EriusFunctionValue{
						{
							Name:   "Input",
							Type:   "string",
							Global: "PipelineWithPipeline.Input",
						},
					},
					Output: []entity.EriusFunctionValue{
						{
							Name:   "Output",
							Type:   "string",
							Global: "Block1.Output",
						},
					},
					Next: "Scenario",
				},
				"Scenario": {
					BlockType: script.TypeScenario,
					Title:     "LinearPipeline",
					Input: []entity.EriusFunctionValue{
						{
							Name:   "Input",
							Type:   script.TypeString,
							Global: "Block1.Output",
						},
					},
					Output: []entity.EriusFunctionValue{
						{
							Name:   "Output",
							Type:   script.TypeString,
							Global: "Scenario.Output",
						},
					},
					Next: "Block2",
				},
				"Block2": {
					BlockType: script.TypePython3,
					Title:     "Block2",
					Input: []entity.EriusFunctionValue{
						{
							Name:   "Input",
							Type:   "string",
							Global: "Scenario.Output",
						},
					},
					Output: []entity.EriusFunctionValue{
						{
							Name:   "Output",
							Type:   "string",
							Global: "Block2.Output",
						},
					},
				},
			},
		},
	}

	// Same as linear pipeline, but with linear pipeline inside
	// Block1, Block2, Block3 should accept {"Input":"string"} and return {"Output":"string"}
	// Test should check for block running sequence and block input
	PipelineWithPipelineTestable = TestablePipeline{
		FunctionHandlers: map[string]http.HandlerFunc{
			"Block1": LinearPipelineBlock,
			"Block2": LinearPipelineBlock,
			"Block3": LinearPipelineBlock,
		},
		PipelineUUID: PipelineWithPipelineUUID,
		pipeline:     &pipelineWithPipeline,
	}

	forInForPipeline = entity.EriusScenario{
		ID:        ForInForPipelineUUID,
		VersionID: ForInForPipelineVersionUUID,
		Status:    db.StatusApproved,
		HasDraft:  false,
		Name:      "ForInForPipeline",
		Pipeline: struct {
			Entrypoint string                      `json:"entrypoint"`
			Blocks     map[string]entity.EriusFunc `json:"blocks"`
		}{
			Entrypoint: "MasGen1",
			Blocks: map[string]entity.EriusFunc{
				"MasGen1": {
					BlockType: script.TypePython3,
					Title:     "MasGen",
					Output: []entity.EriusFunctionValue{
						{
							Name:   "Output",
							Type:   script.TypeArray,
							Global: "MasGen1.Output",
						},
					},
					Next: "For1",
				},
				"For1": {
					BlockType: script.TypeInternal,
					Title:     "for",
					Input: []entity.EriusFunctionValue{
						{
							Name:   "iter",
							Type:   script.TypeArray,
							Global: "MasGen1.Output",
						},
					},
					Output: []entity.EriusFunctionValue{
						{
							Name:   "index",
							Type:   script.TypeNumber,
							Global: "For1.index",
						},
						{
							Name:   "now_on",
							Type:   script.TypeString,
							Global: "For1.now_on",
						},
					},
					OnTrue:  "",
					OnFalse: "MasGen2",
				},
				"MasGen2": {
					BlockType: script.TypePython3,
					Title:     "MasGen",
					Output: []entity.EriusFunctionValue{
						{
							Name:   "Output",
							Type:   script.TypeArray,
							Global: "MasGen2.Output",
						},
					},
					Next: "For2",
				},
				"For2": {
					BlockType: script.TypeInternal,
					Title:     "for",
					Input: []entity.EriusFunctionValue{
						{
							Name:   "iter",
							Type:   script.TypeArray,
							Global: "MasGen2.Output",
						},
					},
					Output: []entity.EriusFunctionValue{
						{
							Name:   "index",
							Type:   script.TypeNumber,
							Global: "For2.index",
						},
						{
							Name:   "now_on",
							Type:   script.TypeString,
							Global: "For2.now_on",
						},
					},
					OnTrue:  "For1",
					OnFalse: "Block1",
				},
				"Block1": {
					BlockType: script.TypePython3,
					Title:     "Block1",
					Next:      "For2",
				},
			},
		},
	}

	// Runs loop inside loop
	// MasGen block should return {"Output":[]}, Block1 should be empty
	// Test should check for block running sequence
	ForInForPipelineTestable = TestablePipeline{
		FunctionHandlers: map[string]http.HandlerFunc{
			"MasGen": OnlyReturnBlockGenerator(map[string]interface{}{"Output": []string{"1", "2", "3"}}),
			"Block1": EmptyBlock,
		},
		PipelineUUID: ForInForPipelineUUID,
		pipeline:     &forInForPipeline,
	}

	stringsEqualPipeline = entity.EriusScenario{
		ID:        StringsEqualPipelineUUID,
		VersionID: StringsEqualPipelineVersionUUID,
		Status:    db.StatusApproved,
		HasDraft:  false,
		Name:      "StringsEqualPipeline",
		Input:     nil,
		Output:    nil,
		Pipeline: struct {
			Entrypoint string                      `json:"entrypoint"`
			Blocks     map[string]entity.EriusFunc `json:"blocks"`
		}{
			Entrypoint: "Block1",
			Blocks: map[string]entity.EriusFunc{
				"Block1": {
					BlockType: script.TypePython3,
					Title:     "Block1",
					Input:     nil,
					Output: []entity.EriusFunctionValue{
						{
							Name:   "Output",
							Type:   script.TypeString,
							Global: "Block1.Output",
						},
					},
					Next: "Block2",
				},
				"Block2": {
					BlockType: script.TypePython3,
					Title:     "Block2",
					Input:     nil,
					Output: []entity.EriusFunctionValue{
						{
							Name:   "Output",
							Type:   script.TypeBool,
							Global: "Block2.Output",
						},
					},
					Next: "StringsEqual",
				},
				"StringsEqual": {
					BlockType: script.TypeInternal,
					Title:     "strings_is_equal",
					Input: []entity.EriusFunctionValue{
						{
							Name:   "a",
							Type:   script.TypeBool,
							Global: "Block1.Output",
						},
						{
							Name:   "b",
							Type:   script.TypeBool,
							Global: "Block2.Output",
						},
					},
					OnTrue:  "BlockTrue",
					OnFalse: "BlockFalse",
				},
				"BlockTrue": {
					BlockType: script.TypePython3,
					Title:     "BlockTrue",
					Input:     nil,
					Output: []entity.EriusFunctionValue{
						{
							Name:   "Output",
							Type:   script.TypeString,
							Global: "BlockTrue.Output",
						},
					},
					Next: "",
				},
				"BlockFalse": {
					BlockType: script.TypePython3,
					Title:     "BlockFalse",
					Input:     nil,
					Output: []entity.EriusFunctionValue{
						{
							Name:   "Output",
							Type:   script.TypeString,
							Global: "BlockTrue.Output",
						},
					},
					Next: "",
				},
			},
		},
	}

	// Pipeline passes Output of Block1 and Block2 to StringsEqual
	// Should run BlockTrue
	StringsEqualsPipelineTrueTestable = TestablePipeline{
		FunctionHandlers: map[string]http.HandlerFunc{
			"Block1":     OnlyReturnBlockGenerator(map[string]interface{}{"Output": "value"}),
			"Block2":     OnlyReturnBlockGenerator(map[string]interface{}{"Output": "value"}),
			"BlockTrue":  EmptyBlock,
			"BlockFalse": EmptyBlock,
		},
		PipelineUUID: StringsEqualPipelineUUID,
		pipeline:     &stringsEqualPipeline,
	}

	// Pipeline passes Output of Block1 and Block2 to StringsEqual
	// Should run BlockFalse
	StringsEqualsPipelineFalseTestable = TestablePipeline{
		FunctionHandlers: map[string]http.HandlerFunc{
			"Block1":     OnlyReturnBlockGenerator(map[string]interface{}{"Output": "value"}),
			"Block2":     OnlyReturnBlockGenerator(map[string]interface{}{"Output": "other value"}),
			"BlockTrue":  EmptyBlock,
			"BlockFalse": EmptyBlock,
		},
		PipelineUUID: StringsEqualPipelineUUID,
		pipeline:     &stringsEqualPipeline,
	}

	connectorPipeline = entity.EriusScenario{
		ID:        ConnectorPipelineUUID,
		VersionID: ConnectorPipelineVersionUUID,
		Status:    db.StatusApproved,
		HasDraft:  false,
		Name:      "ConnectorPipeline",
		Input:     nil,
		Output:    nil,
		Pipeline: struct {
			Entrypoint string                      `json:"entrypoint"`
			Blocks     map[string]entity.EriusFunc `json:"blocks"`
		}{
			Entrypoint: "Block1",
			Blocks: map[string]entity.EriusFunc{
				"Block1": {
					BlockType: script.TypePython3,
					Title:     "Block1",
					Input:     nil,
					Output: []entity.EriusFunctionValue{
						{
							Name:   "Output",
							Type:   script.TypeArray,
							Global: "Block1.Output",
						},
					},
					Next: "Block2",
				},
				"Block2": {
					BlockType: script.TypePython3,
					Title:     "Block2",
					Input:     nil,
					Output: []entity.EriusFunctionValue{
						{
							Name:   "Output",
							Type:   script.TypeArray,
							Global: "Block2.Output",
						},
					},
					Next: "Connector",
				},
				"Connector": {
					BlockType: script.TypeInternal,
					Title:     "connector",
					Input: []entity.EriusFunctionValue{
						{
							Name:   "a",
							Type:   script.TypeArray,
							Global: "Block1.Output",
						},
						{
							Name:   "b",
							Type:   script.TypeArray,
							Global: "Block2.Output",
						},
					},
					Output: []entity.EriusFunctionValue{
						{
							Name:   "Output",
							Type:   script.TypeArray,
							Global: "Connector.Output",
						},
					},
					Next: "Block3",
				},
				"Block3": {
					BlockType: script.TypePython3,
					Title:     "Block3",
					Input: []entity.EriusFunctionValue{
						{
							Name:   "Input",
							Type:   script.TypeArray,
							Global: "Connector.Output",
						},
					},
					Output: nil,
					Next:   "",
				},
			},
		},
	}

	// Pipeline passes output of Block1 and Block2 to connector block
	// Block3 should receive Block1.Output
	ConnectorPipelineTestable = TestablePipeline{
		FunctionHandlers: map[string]http.HandlerFunc{
			"Block1": OnlyReturnBlockGenerator(map[string]interface{}{"Output": []string{"1", "2", "3"}}),
			"Block2": OnlyReturnBlockGenerator(map[string]interface{}{}),
			"Block3": EmptyBlock,
		},
		PipelineUUID: ConnectorPipelineUUID,
		pipeline:     &connectorPipeline,
	}

	pipelines = []entity.EriusScenario{
		linearPipeline,
		ifPipeline,
		forPipeline,
		pipelineWithPipeline,
		forInForPipeline,
		stringsEqualPipeline,
		connectorPipeline,
	}
)

type MockDB struct {
	VersionList []entity.EriusScenarioInfo

	pipelines []entity.EriusScenario
}

func NewMockDB() *MockDB {
	return &MockDB{pipelines: pipelines}
}

func (m *MockDB) GetApprovedVersions(c context.Context) ([]entity.EriusScenarioInfo, error) {
	return m.GetVersionsByStatus(c, db.StatusApproved)
}

func (m *MockDB) GetVersionsByStatus(c context.Context, status int) ([]entity.EriusScenarioInfo, error) {
	versionInfoList := make([]entity.EriusScenarioInfo, 0)

	e := entity.EriusScenarioInfo{
		ID:            uuid.UUID{},
		VersionID:     uuid.UUID{},
		CreatedAt:     time.Time{},
		ApprovedAt:    time.Time{},
		Author:        "",
		Approver:      "",
		Name:          "",
		Tags:          nil,
		LastRun:       time.Time{},
		LastRunStatus: "",
		Status:        0,
	}

	versionInfoList = append(versionInfoList, e)

	return versionInfoList, nil
}

func (m *MockDB) GetDraftVersions(c context.Context, author string) ([]entity.EriusScenarioInfo, error) {
	panic("implement me")
}

func (m *MockDB) GetOnApproveVersions(c context.Context) ([]entity.EriusScenarioInfo, error) {
	panic("implement me")
}

func (m *MockDB) GetWorkedVersions(c context.Context) ([]entity.EriusScenario, error) {
	panic("implement me")
}

func (m *MockDB) GetVersionsByStatusAndAuthor(c context.Context, status int, author string) ([]entity.EriusScenarioInfo, error) {
	panic("implement me")
}

func (m *MockDB) SwitchApproved(c context.Context, pipelineID, versionID uuid.UUID, author string) error {
	panic("implement me")
}

func (m *MockDB) VersionEditable(c context.Context, versionID uuid.UUID) (bool, error) {
	panic("implement me")
}

func (m *MockDB) CreatePipeline(c context.Context, p *entity.EriusScenario, author string, pipelineData []byte) error {
	panic("implement me")
}

func (m *MockDB) CreateVersion(c context.Context, p *entity.EriusScenario, author string, pipelineData []byte) error {
	panic("implement me")
}

func (m *MockDB) DeleteVersion(c context.Context, versionID uuid.UUID) error {
	panic("implement me")
}

func (m *MockDB) DeletePipeline(c context.Context, id uuid.UUID) error {
	panic("implement me")
}

func (m *MockDB) GetPipeline(c context.Context, id uuid.UUID) (*entity.EriusScenario, error) {
	return m.GetPipelineVersion(c, id)
}

func (m *MockDB) GetPipelineVersion(c context.Context, id uuid.UUID) (*entity.EriusScenario, error) {
	for i := range m.pipelines {
		if m.pipelines[i].ID == id {
			return &m.pipelines[i], nil
		}
	}

	return nil, fmt.Errorf("not found")
}

func (m *MockDB) UpdateDraft(c context.Context, p *entity.EriusScenario, pipelineData []byte) error {
	panic("implement me")
}

func (m *MockDB) WriteContext(c context.Context, workID uuid.UUID, stage string, data []byte) error {
	return nil
}

func (m *MockDB) WriteTask(c context.Context, workID, versionID uuid.UUID, author string) error {
	return nil
}

func (m *MockDB) ChangeWorkStatus(c context.Context, workID uuid.UUID, status int) error {
	return nil
}

func (m *MockDB) GetExecutableScenarios(c context.Context) ([]entity.EriusScenario, error) {
	panic("implement me")
}

func (m *MockDB) GetExecutableByName(c context.Context, name string) (*entity.EriusScenario, error) {
	for i := range m.pipelines {
		if m.pipelines[i].Name == name {
			return &m.pipelines[i], nil
		}
	}

	return nil, fmt.Errorf("not found")
}

func (m *MockDB) ActiveAlertNGSA(c context.Context, sever int, state, source, eventType, cause, addInf, addTxt, moID, specProb, notID, usertext, moi, moc string) error {
	panic("implement me")
}

func (m *MockDB) ClearAlertNGSA(c context.Context, name string) error {
	panic("implement me")
}
