package entity

type MonitoringTaskNode struct {
	WorkNumber   string `json:"work_number"`
	VersionId    string `json:"version_id"`
	Author       string `json:"author"`
	CreationTime string `json:"creation_time"`
	ScenarioName string `json:"scenario_name"`
	NodeId       string `json:"node_id"`
	RealName     string `json:"real_name"`
	Status       string `json:"status"`
	StepName     string `json:"step_name"`
}

type BlockOutputs []BlockOutputValue

type BlockOutputValue struct {
	StepName string
	Name     string
	Value    interface{}
}

type BlockInputs []BlockInputValue

type BlockInputValue struct {
	Name  string
	Value interface{}
}
