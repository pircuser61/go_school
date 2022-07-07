package script

import (
	"encoding/json"

	"github.com/google/uuid"
)

type Block int

const (
	IfState Block = iota
	Input
	Equal
	Vars
	Connector
	ForState
)

func (m Block) Model() FunctionModel {
	f := FunctionModel{}

	switch m {
	case Input:
		f = FunctionModel{
			BlockType: TypeInternal,
			Title:     "input",
			Inputs:    nil,
			Outputs: []FunctionValueModel{
				{
					Name: "notification",
					Type: TypeString,
				},
				{
					Name: "action",
					Type: TypeString,
				},
			},
			ShapeType: shapeFunction,
			Sockets:   []string{defaultSocket},
		}
	case Equal:
		f = FunctionModel{
			BlockType: TypeIF,
			Title:     "strings_is_equal",
			Inputs: []FunctionValueModel{
				{
					Name: firstStringName,
					Type: TypeString,
				},
				{
					Name: secondStringName,
					Type: TypeString,
				},
			},
			Outputs:   nil,
			Sockets:   []string{trueSocket, falseSocket},
			ShapeType: shapeRhombus,
		}
	case Vars:
		f = FunctionModel{
			BlockType: TypeInternal,
			Title:     "variables",
			Inputs:    nil,
			Outputs:   []FunctionValueModel{},
			Sockets:   []string{defaultSocket},
			ShapeType: shapeVariable,
		}
	case Connector:
		f = FunctionModel{
			BlockType: TypeInternal,
			Title:     "connector",
			Inputs: []FunctionValueModel{
				{
					Name: "non_block",
					Type: TypeArray,
				}, {
					Name: "block",
					Type: TypeArray,
				},
			},
			Outputs: []FunctionValueModel{
				{
					Name: "final_list",
					Type: TypeArray,
				},
			},
			Sockets:   []string{defaultSocket},
			ShapeType: shapeConnector,
		}
	case ForState:
		f = FunctionModel{
			BlockType: TypeIF,
			Title:     "for",
			Inputs: []FunctionValueModel{
				{
					Name: "iter",
					Type: TypeArray,
				},
			},
			Outputs: []FunctionValueModel{
				{
					Name: "now_on",
					Type: TypeString,
				},
				{
					Name: "index",
					Type: TypeNumber,
				},
			},
			ShapeType: shapeRhombus,
			Sockets:   []string{trueSocket, falseSocket},
		}
	}

	return f
}

type BlockUpdateData struct {
	Id         uuid.UUID
	ByLogin    string
	Action     string
	Parameters json.RawMessage
}
