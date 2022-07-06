package script

type Embedder interface {
	Model() FunctionModel
}

// do not touch, only add in the end.
const (
	shapeFunction int = iota
	shapeRhombus
	ShapeScenario
	ShapeIntegration
	shapeConnector
	shapeVariable

	// TODO: rm copy
	defaultSocket = "default"
	trueSocket    = "true"
	falseSocket   = "false"

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

	TypeIF          = "term"
	TypeInternal    = "internal"
	TypeScenario    = "scenario"
	TypePython3     = "python3"
	TypePythonFlask = "python3-flask"
	TypePythonHTTP  = "python3-http"

	TypeGo = "go"
)

type FunctionModels []FunctionModel

type FunctionModel struct {
	BlockType string               `json:"block_type"`
	Title     string               `json:"title"`
	Inputs    []FunctionValueModel `json:"inputs,omitempty"`
	Outputs   []FunctionValueModel `json:"outputs,omitempty"`
	ShapeType int                  `json:"shape_type"`
	ID        string               `json:"id"`
	Params    *FunctionParams      `json:"params,omitempty"`
	Sockets   []string             `json:"sockets"`
}

// TODO: find a better way to implement oneOf
type FunctionParams struct {
	Type   string      `json:"type" enums:"approver,servicedesk_application,conditions" example:"approver"`
	Params interface{} `json:"params,omitempty"`
}

type FunctionValueModel struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Comment string `json:"comment"`
}
