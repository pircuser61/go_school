package kafka

import "github.com/google/uuid"

type RunnerOutMessage struct {
	TaskID          uuid.UUID              `json:"task_id"`
	FunctionMapping map[string]interface{} `json:"function_mapping"`
	FunctionName    string                 `json:"function_name"`
	RetryPolicy     string                 `json:"retry_policy"`
	SystemStand     string                 `json:"system_stand"`
}

type RunnerInMessage struct {
	NodeID          uuid.UUID              `json:"node_id"`
	FunctionMapping map[string]interface{} `json:"function_mapping"`
}
