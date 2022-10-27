package test

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/iancoleman/orderedmap"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"

	"github.com/google/uuid"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
)

func OnlyReturnBlockGenerator(ret map[string]interface{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-type", "application/json")
		w.WriteHeader(http.StatusOK)

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
	errNotFound       = errors.New("not found")
	errNotImplemented = errors.New("not implemented")
)

// nolint //need globals
var (
	linearPipelineBlock = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		type InputStruct struct {
			Input string `json:"Output"`
		}
		type OutputStruct struct {
			Output string `json:"Output"`
		}

		w.Header().Set("Content-type", "application/json")

		bytes, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		var jsonInput InputStruct

		err = json.Unmarshal(bytes, &jsonInput)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		bytesOutput, err := json.Marshal(OutputStruct{Output: jsonInput.Input})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bytesOutput)
	})
	stringIsEqualToBlockGenerator = func(equalsTo string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			type InputStruct struct {
				Input string `json:"Output"`
			}
			type OutputStruct struct {
				Output bool `json:"Output"`
			}

			w.Header().Set("Content-type", "application/json")

			bytes, err := ioutil.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			var jsonInput InputStruct

			err = json.Unmarshal(bytes, &jsonInput)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			bytesOutput, err := json.Marshal(OutputStruct{Output: jsonInput.Input == equalsTo})
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(bytesOutput)
		}
	}
	emptyBlock = OnlyReturnBlockGenerator(map[string]interface{}{})

	linearPipelineUUID        = uuid.New()
	linearPipelineVersionUUID = uuid.New()

	ifPipelineUUID        = uuid.New()
	ifPipelineVersionUUID = uuid.New()

	forPipelineUUID        = uuid.New()
	forPipelineVersionUUID = uuid.New()

	pipelineWithPipelineUUID        = uuid.New()
	pipelineWithPipelineVersionUUID = uuid.New()

	forInForPipelineUUID        = uuid.New()
	forInForPipelineVersionUUID = uuid.New()

	stringsEqualPipelineUUID        = uuid.New()
	stringsEqualPipelineVersionUUID = uuid.New()

	connectorPipelineUUID        = uuid.New()
	connectorPipelineVersionUUID = uuid.New()

	linearPipeline = entity.EriusScenario{
		ID:        linearPipelineUUID,
		VersionID: linearPipelineVersionUUID,
		Status:    db.StatusApproved,
		HasDraft:  false,
		Name:      "LinearPipeline",
		Input: []entity.EriusFunctionValue{
			{
				Name:   "Output",
				Type:   "string",
				Global: "LinearPipeline.Output",
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
							Name:   "Output",
							Type:   "string",
							Global: "LinearPipeline.Output",
						},
					},
					Output: []entity.EriusFunctionValue{
						{
							Name:   "Output",
							Type:   "string",
							Global: "Block1.Output",
						},
					},
					Next: map[string][]string{
						"default": []string{"Block2"},
					},
				},
				"Block2": {
					BlockType: script.TypePython3,
					Title:     "Block2",
					Input: []entity.EriusFunctionValue{
						{
							Name:   "Output",
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
					Next: map[string][]string{
						"default": []string{"Block3"},
					},
				},
				"Block3": {
					BlockType: script.TypePython3,
					Title:     "Block3",
					Input: []entity.EriusFunctionValue{
						{
							Name:   "Output",
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
					Next: map[string][]string{},
				},
			},
		},
	}

	// Pipeline accept {"Output":"string"} and return {"Output":"string"}
	// Output string goes through pipeline
	// Block1, Block2, Block3 should accept {"Output":"string"} and return {"Output":"string"}
	// Test should check for block running sequence and block input
	//nolint:gochecknoglobals //need this as global
	LinearPipelineTestable = TestablePipeline{
		FunctionHandlers: map[string]http.HandlerFunc{
			"Block1": linearPipelineBlock,
			"Block2": linearPipelineBlock,
			"Block3": linearPipelineBlock,
		},
		PipelineUUID: linearPipelineUUID,
		pipeline:     &linearPipeline,
	}

	ifPipeline = entity.EriusScenario{
		ID:        ifPipelineUUID,
		VersionID: ifPipelineVersionUUID,
		Status:    db.StatusApproved,
		HasDraft:  false,
		Name:      "IfPipeline",
		Input: []entity.EriusFunctionValue{
			{
				Name:   "Output",
				Type:   script.TypeString,
				Global: "IfPipeline.Output",
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
							Name:   "Output",
							Type:   script.TypeString,
							Global: "IfPipeline.Output",
						},
					},
					Output: []entity.EriusFunctionValue{
						{
							Name:   "Output",
							Type:   script.TypeBool,
							Global: "Block1.Output",
						},
					},
					Next: map[string][]string{
						"default": []string{"BlockIf"},
					},
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
					Next: map[string][]string{
						"true":  []string{"BlockTrue"},
						"false": []string{"BlockFalse"},
					},
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
					Next: map[string][]string{},
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
					Next: map[string][]string{},
				},
			},
		},
	}

	// Pipeline accept {"Output":"string"} and returns none
	// Block1 should compare pipeline input with something inside and return bool
	// Depending on Block1.Output runs BlockTrue or BlockFalse
	// Test should check for block running sequence
	//nolint:gochecknoglobals //need this as global
	IfPipelineTestable = TestablePipeline{
		FunctionHandlers: map[string]http.HandlerFunc{
			"Block1":     stringIsEqualToBlockGenerator("Value"),
			"BlockTrue":  OnlyReturnBlockGenerator(map[string]interface{}{"Output": "true"}),
			"BlockFalse": OnlyReturnBlockGenerator(map[string]interface{}{"Output": "false"}),
		},
		PipelineUUID: ifPipelineUUID,
		pipeline:     &ifPipeline,
	}

	forPipeline = entity.EriusScenario{
		ID:        forPipelineUUID,
		VersionID: forPipelineVersionUUID,
		Status:    db.StatusApproved,
		HasDraft:  false,
		Name:      "ForPipeline",
		Input: []entity.EriusFunctionValue{
			{
				Name:   "Output",
				Type:   script.TypeNumber,
				Global: "ForPipeline.Output",
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
							Name:   "Output",
							Type:   script.TypeNumber,
							Global: "Block1.Output",
						},
					},
					Output: []entity.EriusFunctionValue{
						{
							Name:   "Output",
							Type:   script.TypeArray,
							Global: "Block1.Output",
						},
					},
					Next: map[string][]string{
						"default": []string{"For"},
					},
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
					Next: map[string][]string{
						"true":  []string{"Block3"},
						"false": []string{"Block2"},
					},
				},
				"Block2": {
					BlockType: script.TypePython3,
					Title:     "Block2",
					Input:     nil,
					Output:    nil,
					Next: map[string][]string{
						"default": []string{"For"},
					},
				},
				"Block3": {
					BlockType: script.TypePython3,
					Title:     "Block3",
					Input:     []entity.EriusFunctionValue{},
					Output:    []entity.EriusFunctionValue{},
					Next:      map[string][]string{},
				},
			},
		},
	}

	// Pipeline accept {"Output":123} and returns none
	// Block1 generates array
	// For every item in array run Block2
	// After loop run Block3
	// Test should check for block running sequence
	//nolint:gochecknoglobals //need this as global
	ForPipelineTestable = TestablePipeline{
		FunctionHandlers: map[string]http.HandlerFunc{
			"Block1": OnlyReturnBlockGenerator(map[string]interface{}{"Output": []string{"1", "2", "3"}}),
			"Block2": emptyBlock,
			"Block3": emptyBlock,
		},
		PipelineUUID: forPipelineUUID,
		pipeline:     &forPipeline,
	}

	pipelineWithPipeline = entity.EriusScenario{
		ID:        pipelineWithPipelineUUID,
		VersionID: pipelineWithPipelineVersionUUID,
		Status:    db.StatusApproved,
		HasDraft:  false,
		Name:      "PipelineWithPipeline",
		Input: []entity.EriusFunctionValue{
			{
				Name:   "Output",
				Type:   script.TypeString,
				Global: "PipelineWithPipeline.Output",
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
							Name:   "Output",
							Type:   "string",
							Global: "PipelineWithPipeline.Output",
						},
					},
					Output: []entity.EriusFunctionValue{
						{
							Name:   "Output",
							Type:   "string",
							Global: "Block1.Output",
						},
					},
					Next: map[string][]string{
						"default": []string{"Scenario"},
					},
				},
				"Scenario": {
					BlockType: script.TypeScenario,
					Title:     "LinearPipeline",
					Input: []entity.EriusFunctionValue{
						{
							Name:   "Output",
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
					Next: map[string][]string{
						"default": []string{"Block2"},
					},
				},
				"Block2": {
					BlockType: script.TypePython3,
					Title:     "Block2",
					Input: []entity.EriusFunctionValue{
						{
							Name:   "Output",
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
	// Block1, Block2, Block3 should accept {"Output":"string"} and return {"Output":"string"}
	// Test should check for block running sequence and block input
	//nolint:gochecknoglobals //need this as global
	PipelineWithPipelineTestable = TestablePipeline{
		FunctionHandlers: map[string]http.HandlerFunc{
			"Block1": linearPipelineBlock,
			"Block2": linearPipelineBlock,
			"Block3": linearPipelineBlock,
		},
		PipelineUUID: pipelineWithPipelineUUID,
		pipeline:     &pipelineWithPipeline,
	}

	forInForPipeline = entity.EriusScenario{
		ID:        forInForPipelineUUID,
		VersionID: forInForPipelineVersionUUID,
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
					Next: map[string][]string{
						"default": []string{"For1"},
					},
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
					Next: map[string][]string{
						"true":  []string{""},
						"false": []string{"MasGen2"},
					},
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
					Next: map[string][]string{
						"default": []string{"For2"},
					},
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
					Next: map[string][]string{
						"true":  []string{"For1"},
						"false": []string{"Block1"},
					},
				},
				"Block1": {
					BlockType: script.TypePython3,
					Title:     "Block1",
					Next: map[string][]string{
						"default": []string{"For2"},
					},
				},
			},
		},
	}

	// Runs loop inside loop
	// MasGen block should return {"Output":[]}, Block1 should be empty
	// Test should check for block running sequence
	//nolint:gochecknoglobals //need this as global
	ForInForPipelineTestable = TestablePipeline{
		FunctionHandlers: map[string]http.HandlerFunc{
			"MasGen": OnlyReturnBlockGenerator(map[string]interface{}{"Output": []string{"1", "2", "3"}}),
			"Block1": emptyBlock,
		},
		PipelineUUID: forInForPipelineUUID,
		pipeline:     &forInForPipeline,
	}

	stringsEqualPipeline = entity.EriusScenario{
		ID:        stringsEqualPipelineUUID,
		VersionID: stringsEqualPipelineVersionUUID,
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
					Next: map[string][]string{
						"default": []string{"Block2"},
					},
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
					Next: map[string][]string{
						"default": []string{"StringsEqual"},
					},
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
					Next: map[string][]string{
						"true":  []string{"BlockTrue"},
						"false": []string{"BlockFalse"},
					},
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
					Next: map[string][]string{},
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
					Next: map[string][]string{},
				},
			},
		},
	}

	// Pipeline passes Output of Block1 and Block2 to StringsEqual
	// Should run BlockTrue
	//nolint:gochecknoglobals //need this as global
	StringsEqualsPipelineTrueTestable = TestablePipeline{
		FunctionHandlers: map[string]http.HandlerFunc{
			"Block1":     OnlyReturnBlockGenerator(map[string]interface{}{"Output": "value"}),
			"Block2":     OnlyReturnBlockGenerator(map[string]interface{}{"Output": "value"}),
			"BlockTrue":  emptyBlock,
			"BlockFalse": emptyBlock,
		},
		PipelineUUID: stringsEqualPipelineUUID,
		pipeline:     &stringsEqualPipeline,
	}

	// Pipeline passes Output of Block1 and Block2 to StringsEqual
	// Should run BlockFalse
	//nolint:gochecknoglobals //need this as global
	StringsEqualsPipelineFalseTestable = TestablePipeline{
		FunctionHandlers: map[string]http.HandlerFunc{
			"Block1":     OnlyReturnBlockGenerator(map[string]interface{}{"Output": "value"}),
			"Block2":     OnlyReturnBlockGenerator(map[string]interface{}{"Output": "other value"}),
			"BlockTrue":  emptyBlock,
			"BlockFalse": emptyBlock,
		},
		PipelineUUID: stringsEqualPipelineUUID,
		pipeline:     &stringsEqualPipeline,
	}

	connectorPipeline = entity.EriusScenario{
		ID:        connectorPipelineUUID,
		VersionID: connectorPipelineVersionUUID,
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
					Next: map[string][]string{
						"default": []string{"Block2"},
					},
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
					Next: map[string][]string{
						"default": []string{"Connector"},
					},
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
					Next: map[string][]string{
						"default": []string{"Block3"},
					},
				},
				"Block3": {
					BlockType: script.TypePython3,
					Title:     "Block3",
					Input: []entity.EriusFunctionValue{
						{
							Name:   "Output",
							Type:   script.TypeArray,
							Global: "Connector.Output",
						},
					},
					Output: nil,
					Next:   map[string][]string{},
				},
			},
		},
	}

	ngsaPipelineUUID        = uuid.New()
	ngsaPipelineVersionUUID = uuid.New()
	ngsaPipeline            = entity.EriusScenario{
		ID:        ngsaPipelineUUID,
		VersionID: ngsaPipelineVersionUUID,
		Status:    db.StatusApproved,
		HasDraft:  false,
		Name:      "ngsa",
		Input:     nil,
		Output:    nil,
		Pipeline: struct {
			Entrypoint string                      `json:"entrypoint"`
			Blocks     map[string]entity.EriusFunc `json:"blocks"`
		}{
			Entrypoint: "ngsa",
			Blocks: map[string]entity.EriusFunc{
				"ngsa": {
					BlockType: script.TypeInternal,
					Title:     "ngsa-send-alarm",
					Input:     nil,
					Output:    nil,
				},
			},
		},
	}

	// Pipeline passes output of Block1 and Block2 to connector block
	// Block3 should receive Block1.Output
	//nolint:gochecknoglobals //need this as global
	ConnectorPipelineTestable = TestablePipeline{
		FunctionHandlers: map[string]http.HandlerFunc{
			"Block1": OnlyReturnBlockGenerator(map[string]interface{}{"Output": []string{"1", "2", "3"}}),
			"Block2": OnlyReturnBlockGenerator(map[string]interface{}{}),
			"Block3": emptyBlock,
		},
		PipelineUUID: connectorPipelineUUID,
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
		ngsaPipeline,
	}
)

type MockDB struct {
	VersionList []entity.EriusScenarioInfo

	pipelines []entity.EriusScenario
}

func (_m *MockDB) GetApproveActionNames(ctx context.Context) ([]entity.ApproveActionName, error) {
	return nil, nil
}

func (_m *MockDB) GetApproveStatuses(ctx context.Context) ([]entity.ApproveStatus, error) {
	return nil, nil
}

func (_m *MockDB) GetAdditionalForms(workNumber, nodeName string) ([]string, error) {
	return nil, nil
}

func (_m *MockDB) UpdateTaskBlocksData(ctx context.Context, dto *db.UpdateTaskBlocksDataRequest) error {
	return nil
}

func (_m *MockDB) GetUnfinishedTasks(ctx context.Context) (*entity.EriusTasks, error) {
	return &entity.EriusTasks{}, nil
}

func (_m *MockDB) GetOnApproveVersions(c context.Context) ([]entity.EriusScenarioInfo, error) {
	return []entity.EriusScenarioInfo{}, nil
}

func (_m *MockDB) GetApprovedVersions(c context.Context) ([]entity.EriusScenarioInfo, error) {
	return []entity.EriusScenarioInfo{}, nil
}

func (_m *MockDB) GetParentTaskStepByName(ctx context.Context, workID uuid.UUID, stepName string) (*entity.Step, error) {
	return &entity.Step{}, nil
}

func (_m *MockDB) GetTaskStepByName(ctx context.Context, workID uuid.UUID, stepName string) (*entity.Step, error) {
	return &entity.Step{}, nil
}

func (m *MockDB) GetVersionByWorkNumber(c context.Context, workNumber string) (*entity.EriusScenario, error) {
	return &entity.EriusScenario{}, nil
}

func (m *MockDB) GetLastDebugTask(c context.Context, versionID uuid.UUID, author string) (*entity.EriusTask, error) {
	return nil, errNotImplemented
}

func (m *MockDB) UpdateTaskHumanStatus(c context.Context, taskID uuid.UUID, status string) error {
	return nil
}

func (m *MockDB) GetApplicationData(workNumber string) (*orderedmap.OrderedMap, error) {
	return nil, nil
}

func (m *MockDB) SetApplicationData(workNumber string, data *orderedmap.OrderedMap) error {
	return nil
}

//nolint:gocritic //filters
func (m *MockDB) GetTasks(c context.Context, filters entity.TaskFilter) (*entity.EriusTasksPage, error) {
	return nil, errNotImplemented
}

func (m *MockDB) GetTasksCount(c context.Context, userName string) (*entity.CountTasks, error) {
	return nil, errNotImplemented
}

func (m *MockDB) CheckTaskStepsExecuted(ctx context.Context, workNumber string, blocks []string) (bool, error) {
	return false, nil
}

func (m *MockDB) GetUnfinishedTaskStepsByWorkIdAndStepType(ctx context.Context, id uuid.UUID, stepType string) (entity.TaskSteps, error) {
	return nil, errNotImplemented
}

func (m *MockDB) GetTaskStepById(ctx context.Context, id uuid.UUID) (*entity.Step, error) {
	return nil, errNotImplemented
}

func (m *MockDB) GetPipelineTasks(c context.Context, id uuid.UUID) (*entity.EriusTasks, error) {
	return nil, errNotImplemented
}

func (m *MockDB) GetPipelinesWithLatestVersion(c context.Context, author string) ([]entity.EriusScenarioInfo, error) {
	return nil, nil
}

func (m *MockDB) GetPipelineVersions(c context.Context, id uuid.UUID) ([]entity.EriusVersionInfo, error) {
	return nil, nil
}

func (m *MockDB) GetVersionTasks(c context.Context, id uuid.UUID) (*entity.EriusTasks, error) {
	return nil, errNotImplemented
}

func (m *MockDB) GetTaskSteps(c context.Context, id uuid.UUID) (entity.TaskSteps, error) {
	return nil, errNotImplemented
}

func (m *MockDB) GetTask(c context.Context, workNumber string) (*entity.EriusTask, error) {
	return nil, errNotFound
}

func NewMockDB() *MockDB {
	return &MockDB{pipelines: pipelines}
}

func (m *MockDB) GetVersionsByStatus(c context.Context, status int, author string) ([]entity.EriusScenarioInfo, error) {
	versionInfoList := make([]entity.EriusScenarioInfo, 0)

	e := entity.EriusScenarioInfo{
		ID:            uuid.UUID{},
		VersionID:     uuid.UUID{},
		CreatedAt:     time.Time{},
		ApprovedAt:    &time.Time{},
		Author:        "",
		Approver:      "",
		Name:          "",
		Tags:          nil,
		LastRun:       &time.Time{},
		LastRunStatus: new(string),
		Status:        0,
	}

	versionInfoList = append(versionInfoList, e)

	return versionInfoList, nil
}

func (m *MockDB) GetDraftVersions(c context.Context, author string) ([]entity.EriusScenarioInfo, error) {
	return nil, errNotImplemented
}

func (m *MockDB) GetWorkedVersions(c context.Context) ([]entity.EriusScenario, error) {
	return nil, errNotImplemented
}

func (m *MockDB) SwitchApproved(c context.Context, pipelineID, versionID uuid.UUID, author string) error {
	return errNotImplemented
}

func (m *MockDB) VersionEditable(c context.Context, versionID uuid.UUID) (bool, error) {
	return false, errNotImplemented
}

func (m *MockDB) CreatePipeline(c context.Context, p *entity.EriusScenario, author string, pipelineData []byte) error {
	return errNotImplemented
}

func (m *MockDB) CreateVersion(c context.Context, p *entity.EriusScenario, author string, pipelineData []byte) error {
	return errNotImplemented
}

func (m *MockDB) DeleteVersion(c context.Context, versionID uuid.UUID) error {
	return errNotImplemented
}

func (m *MockDB) DeletePipeline(c context.Context, id uuid.UUID) error {
	return errNotImplemented
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

	return nil, errNotFound
}

func (m *MockDB) RenamePipeline(c context.Context, id uuid.UUID, name string) error {
	return errNotImplemented
}

func (m *MockDB) UpdateDraft(c context.Context, p *entity.EriusScenario, pipelineData []byte) error {
	return errNotImplemented
}

func (m *MockDB) SaveStepContext(_ context.Context, _ *db.SaveStepRequest) (uuid.UUID, time.Time, error) {
	return db.NullUuid, time.Time{}, nil
}

func (m *MockDB) UpdateStepContext(_ context.Context, _ *db.UpdateStepRequest) error {
	return nil
}

func (m *MockDB) CreateTask(c context.Context, dto *db.CreateTaskDTO) (*entity.EriusTask, error) {
	return &entity.EriusTask{}, nil
}

func (m *MockDB) ChangeTaskStatus(c context.Context, workID uuid.UUID, status int) error {
	return nil
}

func (m *MockDB) GetExecutableScenarios(c context.Context) ([]entity.EriusScenario, error) {
	return []entity.EriusScenario{}, nil
}

func (m *MockDB) GetExecutableByName(c context.Context, name string) (*entity.EriusScenario, error) {
	for i := range m.pipelines {
		if m.pipelines[i].Name == name {
			return &m.pipelines[i], nil
		}
	}

	return nil, errNotFound
}

func (m *MockDB) ActiveAlertNGSA(c context.Context, sever int, state, source,
	eventType, cause, addInf, addTxt, moID, specProb, notID, usertext, moi, moc string) error {
	return nil
}

func (m *MockDB) ClearAlertNGSA(c context.Context, name string) error {
	return nil
}

func (m *MockDB) CreateTag(c context.Context, e *entity.EriusTagInfo, author string) (*entity.EriusTagInfo, error) {
	return nil, errNotImplemented
}

func (m *MockDB) GetTag(c context.Context, e *entity.EriusTagInfo) (*entity.EriusTagInfo, error) {
	return nil, errNotImplemented
}

func (m *MockDB) EditTag(c context.Context, e *entity.EriusTagInfo) error {
	return errNotImplemented
}

func (m *MockDB) RemoveTag(c context.Context, id uuid.UUID) error {
	return errNotImplemented
}

func (m *MockDB) GetAllTags(c context.Context) ([]entity.EriusTagInfo, error) {
	return nil, errNotImplemented
}

func (m *MockDB) GetPipelineTag(c context.Context, id uuid.UUID) ([]entity.EriusTagInfo, error) {
	return nil, errNotImplemented
}

func (m *MockDB) AttachTag(c context.Context, p uuid.UUID, e *entity.EriusTagInfo) error {
	return errNotImplemented
}

func (m *MockDB) DetachTag(c context.Context, p uuid.UUID, e *entity.EriusTagInfo) error {
	return errNotImplemented
}

func (m *MockDB) RemovePipelineTags(c context.Context, id uuid.UUID) error {
	return errNotImplemented
}

func (m *MockDB) PipelineRemovable(c context.Context, id uuid.UUID) (bool, error) {
	return false, errNotImplemented
}

func (m *MockDB) DeleteAllVersions(c context.Context, id uuid.UUID) error {
	return errNotImplemented
}

func (m *MockDB) PipelineNameCreatable(c context.Context, name string) (bool, error) {
	return false, errNotImplemented
}

func (m *MockDB) SwitchRejected(c context.Context, versionID uuid.UUID, comment, author string) error {
	return errNotImplemented
}

func (m *MockDB) GetRejectedVersions(c context.Context) ([]entity.EriusScenarioInfo, error) {
	return nil, errNotImplemented
}

func (m *MockDB) RollbackVersion(c context.Context, pipelineID, versionID uuid.UUID) error {
	return errNotImplemented
}

func (m *MockDB) GetVersionsByPipelineID(c context.Context, pipelineID string) ([]entity.EriusScenario, error) {
	return nil, errNotImplemented
}

func (m *MockDB) GetPipelinesByNameOrId(c context.Context, dto *db.SearchPipelineRequest) ([]entity.SearchPipeline, error) {
	return nil, errNotImplemented
}

// nolint:gocritic // it's ok
func (m *MockDB) CheckUserCanEditForm(ctx context.Context, workNumber string, stepName string, login string) (bool, error) {
	return false, errNotImplemented
}
