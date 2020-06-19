package script

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
	case IfState:
		f = FunctionModel{
			BlockType: TypeIF,
			Title:     "if",
			Inputs: []FunctionValueModel{
				{
					Name: checkVarName,
					Type: TypeBool,
				},
			},
			NextFuncs: []string{OnTrue, OnFalse},
			ShapeType: shapeRhombus,
		}
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
			NextFuncs: []string{Next},
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
			NextFuncs: []string{OnTrue, OnFalse},
			ShapeType: shapeRhombus,
		}
	case Vars:
		f = FunctionModel{
			BlockType: TypeInternal,
			Title:     "variables",
			Inputs:    nil,
			Outputs:   []FunctionValueModel{},
			NextFuncs: []string{Next},
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
			NextFuncs: []string{Next},
			ShapeType: shapeConnector,
		}
	case ForState:
		f = FunctionModel{
			BlockType: TypeInternal,
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
			NextFuncs: []string{OnTrue, OnFalse},
		}
	}
	return f
}
