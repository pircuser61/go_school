package script

type Embedder interface {
	Model() FunctionModel
}

const (
	shapeFunction int = iota
	shapeRhombus
	shapeScenario
	ShapeIntegration
	shapeConnector
	shapeVariable

	OnTrue       string = "OnTrue"
	OnFalse      string = "OnFalse"
	Next         string = "Next"
	Final        string = "OnTrue"
	OnIter       string = "OnFalse"
	checkVarName string = "check"

	TypeBool   string = "bool"
	TypeString string = "string"
	TypeArray  string = "array"
	TypeNumber string = "number"

	IconFunction     = "X24function"
	IconTerms        = "X24terms"
	IconIntegrations = "X24external"
	IconScenario     = "X24scenario"
	IconConnector    = "X24connector"
	IconVariable     = "X24variable"

	firstStringName  string = "first_string"
	secondStringName string = "second_string"

	functionDeployed string = "deployed"

	TypeIF       = "term"
	TypeInternal = "internal"

)

type FunctionModels []FunctionModel

type FunctionModel struct {
	BlockType string               `json:"block_type"`
	Title     string               `json:"title"`
	Inputs    []FunctionValueModel `json:"inputs"`
	Outputs   []FunctionValueModel `json:"outputs"`
	ShapeType int                  `json:"shape_type"`
	NextFuncs []string             `json:"next_funcs"`
}

type FunctionValueModel struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Comment string `json:"comment"`
}