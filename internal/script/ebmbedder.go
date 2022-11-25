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

	TypeInternal    = "internal"
	TypeScenario    = "scenario"
	TypePython3     = "python3"
	TypePythonFlask = "python3-flask"
	TypePythonHTTP  = "python3-http"

	TypeGo       = "go"
	TypeExternal = "external"
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
	Sockets   []Socket             `json:"sockets"`
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

type Socket struct {
	Id           string   `json:"id"`
	Title        string   `json:"title"`
	NextBlockIds []string `json:"nextBlockIds"`
}

const (
	approvedSocketID    = "approved"
	approvedSocketTitle = "Согласовать"

	rejectedSocketID    = "rejected"
	RejectedSocketTitle = "Отклонить"

	editAppSocketID    = "edit_app"
	editAppSocketTitle = "На доработку"

	executedSocketID    = "executed"
	executedSocketTitle = "Исполнено"

	notExecutedSocketID    = "not_executed"
	notExecutedSocketTitle = "Не исполнено"

	DefaultSocketID    = "default"
	DefaultSocketTitle = "Выход по умолчанию"
)

var (
	DefaultSocket = Socket{Id: DefaultSocketID, Title: DefaultSocketTitle}

	ApprovedSocket = Socket{Id: approvedSocketID, Title: approvedSocketTitle}
	RejectedSocket = Socket{Id: rejectedSocketID, Title: RejectedSocketTitle}
	EditAppSocket  = Socket{Id: editAppSocketID, Title: editAppSocketTitle}

	NotExecutedSocket = Socket{Id: notExecutedSocketID, Title: notExecutedSocketTitle}
	ExecutedSocket    = Socket{Id: executedSocketID, Title: executedSocketTitle}
)

func NewSocket(id string, nexts []string) Socket {
	return Socket{
		Id:           id,
		NextBlockIds: nexts,
	}
}

func GetNexts(from []Socket, socketId string) ([]string, bool) {
	for _, socket := range from {
		if socket.Id == socketId {
			return socket.NextBlockIds, true
		}
	}

	return nil, false
}
