package script

const (
	shapeRectangle int = iota
	shapeRhombus
	shapeCircle
	shapeTriangle
	shapeIntegration

	onTrue string = "OnTrue"
	onFalse string = "OnFalse"
	next string = "Next"
	checkVarName string = "check"

	testVarNameString string = "teststring"
	testVarNameInt string = "testint"

	typeBool string = "bool"
	typeString string = "string"
	typeInt string = "int"

)

type SMFunc struct {
	BlockType string `json:"block_type"`
	Title string `json:"title"`
	Inputs []SMFuncValue `json:"inputs"`
	Outputs []SMFuncValue `json:"outputs"`
	ShapeType int `json:"shape_type"`
	NextFuncs []string `json:"next_funcs"`
}

type SMFuncValue struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
}

type Shape struct {
	ID int
	Title string
	Color string
	Icon string
}

func GetReadyFuncs(scriptManager string) ([]SMFunc, error)  {
	funcs := make([]SMFunc, 0)
	ifstate := SMFunc{
		BlockType: "if-statement",
		Title: "if",
		Inputs: []SMFuncValue{
			{
				Name: checkVarName,
				Type: typeBool,
			},
		},
		NextFuncs: []string{onTrue, onFalse},
		ShapeType: shapeRhombus,
	}

	testBlock := SMFunc{
		BlockType: "testblock",
		Title: "testblock",
		Inputs: []SMFuncValue{
			{
				Name: testVarNameString,
				Type: typeString,
			},
		},
		Outputs: []SMFuncValue{
			{
				Name: testVarNameInt,
				Type: typeInt,
			},
		},
		NextFuncs: []string{next},
		ShapeType: shapeRectangle,
	}

	funcs = append(funcs, ifstate, testBlock)

	return funcs, nil
}

func GetShapes() ([]Shape,error) {
	shapes := []Shape{
		{
			ID: shapeRectangle,
			Title: "rectangle",
			Color: "#123456",
			Icon: "rectangle",
		},
		{
			ID: shapeRhombus,
			Title: "rhombus",
			Color: "#7890AB",
			Icon: "rhombus",
		},
		{
			ID: shapeIntegration,
			Title: "integration",
			Color: "#CDEF12",
			Icon: "integration",
		},
		{
			ID: shapeCircle,
			Title: "circle",
			Color: "#345678",
			Icon: "circle",
		},
		{
			ID: shapeTriangle,
			Title: "triangle",
			Color: "#90ABCD",
			Icon: "triangle",
		},
	}
	return shapes, nil
}
