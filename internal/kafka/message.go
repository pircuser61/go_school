package kafka

import "github.com/google/uuid"

type RunnerOutMessage struct {
	TaskID          uuid.UUID              `json:"task_id"`
	PipelineID      uuid.UUID              `json:"pipeline_id"`
	VersionID       uuid.UUID              `json:"version_id"`
	ClientID        string                 `json:"client_id"`
	WorkNumber      string                 `json:"work_number"`
	FunctionMapping map[string]interface{} `json:"function_mapping"`
	Contracts       string                 `json:"contracts"`
	FunctionName    string                 `json:"function_name"`
	FunctionVersion string                 `json:"function_version"`
	RetryPolicy     string                 `json:"retry_policy"`
}

type RunnerInMessage struct {
	TaskID          uuid.UUID              `json:"task_id"`
	FunctionMapping map[string]interface{} `json:"function_mapping"`
	Err             string                 `json:"err"`
	DoRetry         bool                   `json:"do_retry"`
}
