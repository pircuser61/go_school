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

func (a *FunctionParams) Validate() error {
	return nil
}

type FunctionValueModel struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Comment string `json:"comment"`
	Format  string `json:"format"`
}

type Socket struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	NextBlockIds []string `json:"nextBlockIds"`
	ActionType   string   `json:"actionType"`
}

const (
	approveSocketID    = "approve"
	approveSocketTitle = "Согласовать"

	rejectSocketID    = "reject"
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

	funcSLAExpiredSocketID    = "func_sla_expired"
	funcSLAExpiredSocketTitle = "Закончилось время"

	funcExecutedSocketTitle = "Выполнено"
)

//nolint:gochecknoglobals //new common socket
var (
	DefaultSocket = Socket{ID: DefaultSocketID, Title: DefaultSocketTitle}

	ApproveSocket = Socket{
		ID:         approveSocketID,
		Title:      approveSocketTitle,
		ActionType: "primary",
	}
	RejectSocket = Socket{
		ID:         rejectSocketID,
		Title:      RejectSocketTitle,
		ActionType: "secondary",
	}

	ApproverEditAppSocket = Socket{ID: approverEditAppSocketID, Title: approverEditAppSocketTitle}
	ExecutorEditAppSocket = Socket{ID: executorEditAppSocketID, Title: executorEditAppSocketTitle}

	NotExecutedSocket = Socket{ID: notExecutedSocketID, Title: notExecutedSocketTitle}
	ExecutedSocket    = Socket{ID: executedSocketID, Title: executedSocketTitle}

	SignedSocket   = Socket{ID: signSocketID, Title: signSocketTitle}
	RejectedSocket = Socket{ID: rejectedSocketID, Title: rejectedSocketTitle}

	ErrorSocket = Socket{ID: errorSocketID, Title: errorSocketTitle}

	FuncExecutedSocket    = Socket{ID: DefaultSocketID, Title: funcExecutedSocketTitle}
	FuncErrorSocket       = Socket{ID: errorSocketID, Title: errorSocketTitle}
	FuncTimeExpiredSocket = Socket{ID: funcSLAExpiredSocketID, Title: funcSLAExpiredSocketTitle}

	DelegationsCollection = "delegations_collection"
)

func NewSocket(id string, nexts []string) Socket {
	return Socket{
		ID:           id,
		NextBlockIds: nexts,
	}
}

func GetNexts(from []Socket, socketID string) ([]string, bool) {
	for _, socket := range from {
		if socket.ID == socketID {
			return socket.NextBlockIds, true
		}
	}

	return nil, false
}
