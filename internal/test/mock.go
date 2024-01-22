package test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"

	"gitlab.services.mts.ru/jocasta/pipeliner/internal/db"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/entity"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/script"
	"gitlab.services.mts.ru/jocasta/pipeliner/internal/store"
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

		bytes, err := io.ReadAll(r.Body)
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

			bytes, err := io.ReadAll(r.Body)
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
		PipelineID: linearPipelineUUID,
		VersionID:  linearPipelineVersionUUID,
		Status:     db.StatusApproved,
		HasDraft:   false,
		Name:       "LinearPipeline",
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
		Pipeline: entity.PipelineType{
			Entrypoint: "Block1",
			Blocks: entity.BlocksType{
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
					Output: &script.JSONSchema{
						Type: "object",
						Properties: map[string]script.JSONSchemaPropertiesValue{
							"Output": {
								Type:   "string",
								Global: "Block1.Output",
							},
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
					Output: &script.JSONSchema{
						Type: "object",
						Properties: map[string]script.JSONSchemaPropertiesValue{
							"Output": {
								Type:   "string",
								Global: "Block2.Output",
							},
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
					Output: &script.JSONSchema{
						Type: "object",
						Properties: map[string]script.JSONSchemaPropertiesValue{
							"Output": {
								Type:   "string",
								Global: "Block3.Output",
							},
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
		PipelineID: ifPipelineUUID,
		VersionID:  ifPipelineVersionUUID,
		Status:     db.StatusApproved,
		HasDraft:   false,
		Name:       "IfPipeline",
		Input: []entity.EriusFunctionValue{
			{
				Name:   "Output",
				Type:   script.TypeString,
				Global: "IfPipeline.Output",
			},
		},
		Pipeline: entity.PipelineType{
			Entrypoint: "Block1",
			Blocks: entity.BlocksType{
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
					Output: &script.JSONSchema{
						Type: "object",
						Properties: map[string]script.JSONSchemaPropertiesValue{
							"Output": {
								Type:   script.TypeBool,
								Global: "Block1.Output",
							},
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
					Output: &script.JSONSchema{
						Type: "object",
						Properties: map[string]script.JSONSchemaPropertiesValue{
							"Output": {
								Type:   script.TypeString,
								Global: "BlockTrue.Output",
							},
						},
					},
					Next: map[string][]string{},
				},
				"BlockFalse": {
					BlockType: script.TypePython3,
					Title:     "BlockFalse",
					Output: &script.JSONSchema{
						Type: "object",
						Properties: map[string]script.JSONSchemaPropertiesValue{
							"Output": {
								Type:   script.TypeString,
								Global: "BlockTrue.Output",
							},
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
		PipelineID: forPipelineUUID,
		VersionID:  forPipelineVersionUUID,
		Status:     db.StatusApproved,
		HasDraft:   false,
		Name:       "ForPipeline",
		Input: []entity.EriusFunctionValue{
			{
				Name:   "Output",
				Type:   script.TypeNumber,
				Global: "ForPipeline.Output",
			},
		},
		Output: []entity.EriusFunctionValue{},
		Pipeline: entity.PipelineType{
			Entrypoint: "Block1",
			Blocks: entity.BlocksType{
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
					Output: &script.JSONSchema{
						Type: "object",
						Properties: map[string]script.JSONSchemaPropertiesValue{
							"Output": {
								Type:   script.TypeArray,
								Global: "Block1.Output",
							},
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
					Output: &script.JSONSchema{
						Type: "object",
						Properties: map[string]script.JSONSchemaPropertiesValue{
							"index": {
								Type:   script.TypeNumber,
								Global: "For.index",
							},
							"now_on": {
								Type:   script.TypeString,
								Global: "For.now_on",
							},
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
					Output:    nil,
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
		PipelineID: pipelineWithPipelineUUID,
		VersionID:  pipelineWithPipelineVersionUUID,
		Status:     db.StatusApproved,
		HasDraft:   false,
		Name:       "PipelineWithPipeline",
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
		Pipeline: entity.PipelineType{
			Entrypoint: "Block1",
			Blocks: entity.BlocksType{
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
					Output: &script.JSONSchema{
						Type: "object",
						Properties: map[string]script.JSONSchemaPropertiesValue{
							"Output": {
								Type:   "string",
								Global: "Block1.Output",
							},
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
					Output: &script.JSONSchema{
						Type: "object",
						Properties: map[string]script.JSONSchemaPropertiesValue{
							"Output": {
								Type:   script.TypeString,
								Global: "Scenario.Output",
							},
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
					Output: &script.JSONSchema{
						Type: "object",
						Properties: map[string]script.JSONSchemaPropertiesValue{
							"Output": {
								Type:   "string",
								Global: "Block2.Output",
							},
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
		PipelineID: forInForPipelineUUID,
		VersionID:  forInForPipelineVersionUUID,
		Status:     db.StatusApproved,
		HasDraft:   false,
		Name:       "ForInForPipeline",
		Pipeline: entity.PipelineType{
			Entrypoint: "MasGen1",
			Blocks: entity.BlocksType{
				"MasGen1": {
					BlockType: script.TypePython3,
					Title:     "MasGen",
					Output: &script.JSONSchema{
						Type: "object",
						Properties: map[string]script.JSONSchemaPropertiesValue{
							"Output": {
								Type:   script.TypeArray,
								Global: "MasGen1.Output",
							},
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
					Output: &script.JSONSchema{
						Type: "object",
						Properties: map[string]script.JSONSchemaPropertiesValue{
							"index": {
								Type:   script.TypeNumber,
								Global: "For1.index",
							},
							"now_on": {
								Type:   script.TypeString,
								Global: "For1.now_on",
							},
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
					Output: &script.JSONSchema{
						Type: "object",
						Properties: map[string]script.JSONSchemaPropertiesValue{
							"Output": {
								Type:   script.TypeArray,
								Global: "MasGen2.Output",
							},
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
					Output: &script.JSONSchema{
						Type: "object",
						Properties: map[string]script.JSONSchemaPropertiesValue{
							"index": {
								Type:   script.TypeNumber,
								Global: "For2.index",
							},
							"now_on": {
								Type:   script.TypeString,
								Global: "For2.now_on",
							},
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
		PipelineID: stringsEqualPipelineUUID,
		VersionID:  stringsEqualPipelineVersionUUID,
		Status:     db.StatusApproved,
		HasDraft:   false,
		Name:       "StringsEqualPipeline",
		Input:      nil,
		Output:     nil,
		Pipeline: entity.PipelineType{
			Entrypoint: "Block1",
			Blocks: entity.BlocksType{
				"Block1": {
					BlockType: script.TypePython3,
					Title:     "Block1",
					Input:     nil,
					Output: &script.JSONSchema{
						Type: "object",
						Properties: map[string]script.JSONSchemaPropertiesValue{
							"Output": {
								Type:   script.TypeString,
								Global: "Block1.Output",
							},
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
					Output: &script.JSONSchema{
						Type: "object",
						Properties: map[string]script.JSONSchemaPropertiesValue{
							"Output": {
								Type:   script.TypeBool,
								Global: "Block2.Output",
							},
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
					Output: &script.JSONSchema{
						Type: "object",
						Properties: map[string]script.JSONSchemaPropertiesValue{
							"Output": {
								Type:   script.TypeString,
								Global: "BlockTrue.Output",
							},
						},
					},
					Next: map[string][]string{},
				},
				"BlockFalse": {
					BlockType: script.TypePython3,
					Title:     "BlockFalse",
					Input:     nil,
					Output: &script.JSONSchema{
						Type: "object",
						Properties: map[string]script.JSONSchemaPropertiesValue{
							"Output": {
								Type:   script.TypeString,
								Global: "BlockTrue.Output",
							},
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
		PipelineID: connectorPipelineUUID,
		VersionID:  connectorPipelineVersionUUID,
		Status:     db.StatusApproved,
		HasDraft:   false,
		Name:       "ConnectorPipeline",
		Input:      nil,
		Output:     nil,
		Pipeline: entity.PipelineType{
			Entrypoint: "Block1",
			Blocks: entity.BlocksType{
				"Block1": {
					BlockType: script.TypePython3,
					Title:     "Block1",
					Input:     nil,
					Output: &script.JSONSchema{
						Type: "object",
						Properties: map[string]script.JSONSchemaPropertiesValue{
							"Output": {
								Type:   script.TypeArray,
								Global: "Block1.Output",
							},
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
					Output: &script.JSONSchema{
						Type: "object",
						Properties: map[string]script.JSONSchemaPropertiesValue{
							"Output": {
								Type:   script.TypeArray,
								Global: "Block2.Output",
							},
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
					Output: &script.JSONSchema{
						Type: "object",
						Properties: map[string]script.JSONSchemaPropertiesValue{
							"Output": {
								Type:   script.TypeArray,
								Global: "Connector.Output",
							},
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
		PipelineID: ngsaPipelineUUID,
		VersionID:  ngsaPipelineVersionUUID,
		Status:     db.StatusApproved,
		HasDraft:   false,
		Name:       "ngsa",
		Input:      nil,
		Output:     nil,
		Pipeline: entity.PipelineType{
			Entrypoint: "ngsa",
			Blocks: entity.BlocksType{
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

func (_m *MockDB) GetBlockState(ctx context.Context, blockId string) (entity.BlockState, error) {
	r0 := make(entity.BlockState, 0)
	return r0, nil
}

func (_m *MockDB) AllowRunAsOthers(ctx context.Context, versionID, systemID string, allowRunAsOthers bool) error {
	return nil
}

func (_m *MockDB) GetTaskMembers(ctx context.Context, workNumber string) ([]db.DbMember, error) {
	return nil, nil
}

func (_m *MockDB) RemoveObsoleteMapping(ctx context.Context, versionID string) error {
	return nil
}

//nolint:lll // its ok here
func (_m *MockDB) GetTasksForMonitoring(ctx context.Context, filters *entity.TasksForMonitoringFilters) (*entity.TasksForMonitoring, error) {
	return &entity.TasksForMonitoring{}, nil
}

func (_m *MockDB) GetBlockOutputs(ctx context.Context, blockId, blockName string) (entity.BlockOutputs, error) {
	return nil, errNotImplemented
}

func (_m *MockDB) GetBlockInputs(ctx context.Context, blockId, workNumber string) (entity.BlockInputs, error) {
	return nil, errNotImplemented
}

func (_m *MockDB) GetMergedVariableStorage(ctx context.Context, workId uuid.UUID, blockIds []string) (*store.VariableStore, error) {
	return nil, errNotImplemented
}

func (_m *MockDB) GetBlocksOutputs(ctx context.Context, blockId string) (entity.BlockOutputs, error) {
	return nil, nil
}

func (_m *MockDB) SaveExternalSystemSettings(
	ctx context.Context, versionID string, settings entity.ExternalSystem, schemaFlag *string) error {
	return nil
}

func (_m *MockDB) RemoveExternalSystem(ctx context.Context, versionID, systemID string) error {
	return nil
}

func (_m *MockDB) RemoveExternalSystemTaskSubscriptions(ctx context.Context, versionID, systemID string) error {
	return nil
}

func (_m *MockDB) GetExternalSystemSettings(ctx context.Context, versionID, systemID string) (entity.ExternalSystem, error) {
	return entity.ExternalSystem{}, nil
}

func (_m *MockDB) GetTaskEventsParamsByWorkNumber(ctx context.Context, workNumber, systemID string) (
	entity.ExternalSystemSubscriptionParams, error) {
	return entity.ExternalSystemSubscriptionParams{}, nil
}

func (_m *MockDB) GetExternalSystemTaskSubscriptions(ctx context.Context, versionID, systemID string) (
	entity.ExternalSystemSubscriptionParams, error) {
	return entity.ExternalSystemSubscriptionParams{}, nil
}

func (_m *MockDB) GetExternalSystemsIDs(ctx context.Context, versionID string) ([]uuid.UUID, error) {
	return nil, nil
}

func (_m *MockDB) AddExternalSystemToVersion(ctx context.Context, versionID, systemID string) error {
	return nil
}

func (_m *MockDB) GetVersionSettings(ctx context.Context, id string) (entity.ProcessSettings, error) {
	return entity.ProcessSettings{}, nil
}

func (_m *MockDB) SaveVersionSettings(ctx context.Context, settings entity.ProcessSettings, schemaFlag *string) error {
	return nil
}

func (_m *MockDB) GetTasksCount(
	ctx context.Context,
	currentUser string,
	delegationsByApprovement, delegationsByExecution []string) (*entity.CountTasks, error) {
	return nil, nil
}

func (_m *MockDB) SendTaskToArchive(ctx context.Context, taskID uuid.UUID) (err error) {
	return nil
}

func (_m *MockDB) CheckIsArchived(ctx context.Context, taskID uuid.UUID) (bool, error) {
	return false, nil
}

func (_m *MockDB) UpdateTaskStatus(_ context.Context, _ uuid.UUID, _ int, _, _ string) error {
	return nil
}

func (_m *MockDB) UpdateTaskRate(ctx context.Context, req *db.UpdateTaskRate) error {
	return nil
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

func (_m *MockDB) GetOnApproveVersions(c context.Context) ([]entity.EriusScenarioInfo, error) {
	return []entity.EriusScenarioInfo{}, nil
}

func (_m *MockDB) GetApprovedVersions(c context.Context) ([]entity.EriusScenarioInfo, error) {
	return []entity.EriusScenarioInfo{}, nil
}

func (_m *MockDB) GetParentTaskStepByName(_ context.Context, _ uuid.UUID, _ string) (*entity.Step, error) {
	return &entity.Step{}, nil
}

func (_m *MockDB) GetCanceledTaskSteps(ctx context.Context, taskID uuid.UUID) ([]entity.Step, error) {
	return nil, nil
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

func (m *MockDB) UpdateTaskHumanStatus(_ context.Context, _ uuid.UUID, _, _ string) (*entity.EriusTask, error) {
	return &entity.EriusTask{}, nil
}

func (m *MockDB) GetApplicationData(workNumber string) (string, error) {
	return "", nil
}

//nolint:gocritic //filters
func (m *MockDB) GetTasks(c context.Context, filters entity.TaskFilter,
	delegations []string) (*entity.EriusTasksPage, error) {
	return nil, errNotImplemented
}

func (m *MockDB) ParallelIsFinished(_ context.Context, _ string, _ string) (bool, error) {
	return false, nil
}

func (m *MockDB) GetTaskStepsToWait(_ context.Context, _, _ string) ([]string, error) {
	return nil, nil
}

func (m *MockDB) GetUnfinishedTaskStepsByWorkIdAndStepType(ctx context.Context, id uuid.UUID, stepType string,
	action entity.TaskUpdateAction) (entity.TaskSteps, error) {
	return nil, errNotImplemented
}

func (m *MockDB) GetTaskStepById(ctx context.Context, id uuid.UUID) (*entity.Step, error) {
	return nil, errNotImplemented
}

func (m *MockDB) GetPipelineTasks(c context.Context, id uuid.UUID) (*entity.EriusTasks, error) {
	return nil, errNotImplemented
}

func (m *MockDB) GetPipelinesWithLatestVersion(ctx context.Context,
	authorLogin string,
	publishedPipelines bool,
	page, perPage *int,
	filter string) ([]entity.EriusScenarioInfo, error) {
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

func (m *MockDB) GetTask(_ context.Context, _, _ []string, _, _ string) (*entity.EriusTask, error) {
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

func (m *MockDB) CreateVersion(c context.Context,
	p *entity.EriusScenario, author string, pipelineData []byte, oldVersionID uuid.UUID) error {
	return errNotImplemented
}

func (m *MockDB) DeleteVersion(c context.Context, versionID uuid.UUID) error {
	return errNotImplemented
}

func (m *MockDB) DeletePipeline(c context.Context, id uuid.UUID) error {
	return errNotImplemented
}

func (m *MockDB) GetPipeline(c context.Context, id uuid.UUID) (*entity.EriusScenario, error) {
	return m.GetPipelineVersion(c, id, true)
}

func (m *MockDB) GetPipelineVersion(c context.Context, id uuid.UUID, checkNotDeleted bool) (*entity.EriusScenario, error) {
	for i := range m.pipelines {
		if m.pipelines[i].PipelineID == id {
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

func (m *MockDB) GetTaskFormSchemaID(_, _ string) (string, error) {
	return "", nil
}

func (m *MockDB) SaveStepContext(_ context.Context, _ *db.SaveStepRequest) (uuid.UUID, time.Time, error) {
	return db.NullUuid, time.Time{}, nil
}

func (m *MockDB) UpdateStepContext(_ context.Context, _ *db.UpdateStepRequest) error {
	return nil
}

func (m *MockDB) CreateTask(_ context.Context, _ *db.CreateTaskDTO) (*entity.EriusTask, error) {
	return &entity.EriusTask{}, nil
}

func (m *MockDB) ChangeTaskStatus(_ context.Context, _ uuid.UUID, _ int) error {
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

func (m *MockDB) GetBlockDataFromVersion(_ context.Context, _, _ string) (*entity.EriusFunc, error) {
	return nil, errNotImplemented
}

func (m *MockDB) PipelineRemovable(c context.Context, id uuid.UUID) (bool, error) {
	return false, errNotImplemented
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

func (m *MockDB) GetVersionByPipelineID(c context.Context, pipelineID string) (*entity.EriusScenario, error) {
	return nil, errNotImplemented
}

func (m *MockDB) GetPipelinesByNameOrId(c context.Context, dto *db.SearchPipelineRequest) ([]entity.SearchPipeline, error) {
	return nil, errNotImplemented
}

// nolint:gocritic // it's ok
func (m *MockDB) CheckUserCanEditForm(_ context.Context, _ string, _ string, _ string) (bool, error) {
	return false, errNotImplemented
}

func (m *MockDB) GetTaskRunContext(_ context.Context, _ string) (entity.TaskRunContext, error) {
	return entity.TaskRunContext{}, errNotImplemented
}

func (_m *MockDB) UpdateBlockStateInOthers(_ context.Context, _, _ string, _ []byte) error {
	return nil
}

func (_m *MockDB) UpdateBlockVariablesInOthers(_ context.Context, _ string, _ map[string]interface{}) error {
	return nil
}

func (m *MockDB) GetUsersWithReadWriteFormAccess(
	_ context.Context,
	_ string,
	_ string) ([]entity.UsersWithFormAccess, error) {
	return nil, errNotImplemented
}

func (m *MockDB) StopTaskBlocks(_ context.Context, _ uuid.UUID) error {
	return errNotImplemented
}

func (m *MockDB) GetTaskHumanStatus(_ context.Context, _ uuid.UUID) (string, error) {
	return "", errNotImplemented
}

func (m *MockDB) GetTaskStatus(_ context.Context, _ uuid.UUID) (int, error) {
	return -1, errNotImplemented
}

func (m *MockDB) GetTaskStatusWithReadableString(_ context.Context, _ uuid.UUID) (int, string, error) {
	return -1, "", errNotImplemented
}

func (m *MockDB) GetVariableStorageForStep(_ context.Context, _ uuid.UUID, _ string) (*store.VariableStore, error) {
	return nil, errNotImplemented
}

func (m *MockDB) GetBlocksBreachedSLA(_ context.Context) ([]db.StepBreachedSLA, error) {
	return nil, errNotFound
}

func (m *MockDB) StartTransaction(_ context.Context) (db.Database, error) {
	return nil, errNotFound
}

func (m *MockDB) CommitTransaction(_ context.Context) error {
	return errNotFound
}

func (m *MockDB) RollbackTransaction(_ context.Context) error {
	return errNotFound
}

func (m *MockDB) Ping(_ context.Context) error {
	return errNotFound
}

func (m *MockDB) GetMeanTaskSolveTime(_ context.Context, _ string) ([]entity.TaskCompletionInterval, error) {
	return nil, errNotImplemented
}

func (m *MockDB) GetTaskForMonitoring(ctx context.Context, workNumber string) ([]entity.MonitoringTaskNode, error) {
	return nil, errNotImplemented
}

func (m *MockDB) GetWorksForUserWithGivenTimeRange(
	ctx context.Context,
	hours int,
	login,
	versionID,
	excludeWorkNumber string) ([]*entity.EriusTask, error) {
	return nil, errNotImplemented
}

func (m *MockDB) SaveVersionMainSettings(ctx context.Context, params entity.ProcessSettings) error {
	return errNotImplemented
}

func (m *MockDB) SaveExternalSystemSubscriptionParams(ctx context.Context, versionID string,
	params *entity.ExternalSystemSubscriptionParams) error {
	return nil
}

func (m *MockDB) CheckPipelineNameExists(ctx context.Context, name string, checkNotDeleted bool) (*bool, error) {
	return nil, errNotImplemented
}

func (m *MockDB) UpdateEndingSystemSettings(ctx context.Context, versionID, systemID string,
	settings entity.EndSystemSettings) (err error) {
	return errNotImplemented
}
func (m *MockDB) GetTaskInWorkTime(ctx context.Context, workNumber string) (*entity.TaskCompletionInterval, error) {
	return nil, errNotImplemented
}

func (m *MockDB) SaveSlaVersionSettings(ctx context.Context, versionID string, s entity.SlaVersionSettings) (err error) {
	return errNotImplemented
}

func (m *MockDB) GetSlaVersionSettings(ctx context.Context, versionID string) (s entity.SlaVersionSettings, err error) {
	return entity.SlaVersionSettings{}, errNotImplemented
}

func (m *MockDB) CheckIsTest(ctx context.Context, taskID uuid.UUID) (bool, error) {
	return false, nil
}

func (m *MockDB) GetExecutorsFromPrevExecutionBlockRun(ctx context.Context, taskID uuid.UUID, n string) (
	exec map[string]struct{}, err error) {
	return map[string]struct{}{}, nil
}

func (m *MockDB) GetExecutorsFromPrevWorkVersionExecutionBlockRun(ctx context.Context, workNumber, name string) (
	exec map[string]struct{}, err error) {
	return map[string]struct{}{}, nil
}
