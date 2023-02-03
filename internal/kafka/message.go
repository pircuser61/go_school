package kafka

import "github.com/google/uuid"

type RunnerOutMessage struct {
	TaskID          uuid.UUID              `json:"task_id"`
	FunctionMapping map[string]interface{} `json:"function_mapping"`
	Contracts       string                 `json:"contracts"`
	FunctionName    string                 `json:"function_name"`
	RetryPolicy     string                 `json:"retry_policy"`
}

type RunnerInMessage struct {
	TaskID          uuid.UUID              `json:"task_id"`
	FunctionMapping map[string]interface{} `json:"function_mapping"`
	Err             string                 `json:"err"`
}
