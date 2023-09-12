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
	Outputs   *JSONSchema          `json:"outputs,omitempty"`
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
	Format  string `json:"format"`
}

type Socket struct {
	Id           string   `json:"id"`
	Title        string   `json:"title"`
	NextBlockIds []string `json:"nextBlockIds"`
	ActionType   string   `json:"actionType"`
}

const (
	approveSocketId    = "approve"
	approveSocketTitle = "Согласовать"

	rejectSocketId    = "reject"
	RejectSocketTitle = "Отклонить"

	approverEditAppSocketID    = "approver_send_edit_app"
	approverEditAppSocketTitle = "На доработку"

	executorEditAppSocketID    = "executor_send_edit_app"
	executorEditAppSocketTitle = "На доработку"

	executedSocketID    = "executed"
	executedSocketTitle = "Исполнено"

	notExecutedSocketID    = "not_executed"
	notExecutedSocketTitle = "Не исполнено"

	signSocketID    = "signed"
	signSocketTitle = "Подписано"

	rejectedSocketID    = "rejected"
	rejectedSocketTitle = "Отклонено"

	errorSocketID    = "error"
	errorSocketTitle = "Ошибка"

	DefaultSocketID    = "default"
	DefaultSocketTitle = "Выход по умолчанию"

	funcTimeExpiredSocketID    = "func_time_expired"
	funcTimeExpiredSocketTitle = "Закончилось время"

	funcExecutedSocketTitle = "Выполнено"
)

var (
	DefaultSocket = Socket{Id: DefaultSocketID, Title: DefaultSocketTitle}

	ApproveSocket = Socket{
		Id:         approveSocketId,
		Title:      approveSocketTitle,
		ActionType: "primary",
	}
	RejectSocket = Socket{
		Id:         rejectSocketId,
		Title:      RejectSocketTitle,
		ActionType: "secondary",
	}

	ApproverEditAppSocket = Socket{Id: approverEditAppSocketID, Title: approverEditAppSocketTitle}
	ExecutorEditAppSocket = Socket{Id: executorEditAppSocketID, Title: executorEditAppSocketTitle}

	NotExecutedSocket = Socket{Id: notExecutedSocketID, Title: notExecutedSocketTitle}
	ExecutedSocket    = Socket{Id: executedSocketID, Title: executedSocketTitle}

	//nolint:gochecknoglobals //sign node sockets
	SignedSocket   = Socket{Id: signSocketID, Title: signSocketTitle}
	RejectedSocket = Socket{Id: rejectedSocketID, Title: rejectedSocketTitle}

	//nolint:gochecknoglobals //new common socket
	ErrorSocket = Socket{Id: errorSocketID, Title: errorSocketTitle}

	FuncExecutedSocket    = Socket{Id: DefaultSocketID, Title: funcExecutedSocketTitle}
	FuncTimeExpiredSocket = Socket{Id: funcTimeExpiredSocketID, Title: funcTimeExpiredSocketTitle}

	DelegationsCollection = "delegations_collection"
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
