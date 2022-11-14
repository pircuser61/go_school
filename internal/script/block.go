package script

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type AuthorizationHeader struct{}

type Block int

const (
	Input Block = iota
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
			Sockets:   []Socket{{Id: DefaultSocketID, Title: DefaultSocketTitle}},
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
			Outputs: nil,
			Sockets: []Socket{
				{Id: trueSocketID, Title: trueSocketTitle},
				{Id: falseSocketID, Title: falseSocketTitle},
			},
			ShapeType: shapeRhombus,
		}
	case Vars:
		f = FunctionModel{
			BlockType: TypeInternal,
			Title:     "variables",
			Inputs:    nil,
			Outputs:   []FunctionValueModel{},
			Sockets:   []Socket{{Id: DefaultSocketID, Title: DefaultSocketTitle}},
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
			Sockets:   []Socket{{Id: DefaultSocketID, Title: DefaultSocketTitle}},
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
			Sockets: []Socket{
				{Id: trueSocketID, Title: trueSocketTitle},
				{Id: falseSocketID, Title: falseSocketTitle},
			},
		}
	}

	return f
}

type BlockUpdateData struct {
	Id         uuid.UUID
	ByLogin    string
	Action     string
	Parameters json.RawMessage
	WorkNumber string
	WorkTitle  string
	Author     string
	BlockStart time.Time
}
