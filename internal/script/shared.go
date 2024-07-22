package script

const (
	MethodPost    = "post"
	MethodGet     = "get"
	TransportREST = "rest"
	Method        = "method"
	MainFuncName  = "mainFuncName"
	FuncName      = "funcName"
	Transport     = "transport"
	LogVersion    = "logVersion"
	TraceID       = "traceID"
	WorkID        = "workID"
	StepName      = "stepName"
	WorkNumber    = "workNumber"
	PipelineID    = "pipelineID"
	VersionID     = "versionID"
	StepID        = "stepID"
)

type AccessType string

type FormAccessibility struct {
	NodeID      string     `json:"node_id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	AccessType  AccessType `json:"accessType"`
}

type TaskSolveTime struct {
	MeanWorkHours float64 `json:"meanWorkHours"`
}
