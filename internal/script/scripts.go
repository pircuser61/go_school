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
	BlockType string
	Title string
	Inputs []SMFuncValue
	Outputs []SMFuncValue
	ShapeType int
	NextFuncs []string
}

type SMFuncValue struct {
	Name   string
	Type   string
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