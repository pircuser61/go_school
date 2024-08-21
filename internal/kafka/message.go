package kafka

import (
	"time"

	"github.com/google/uuid"

	"github.com/iancoleman/orderedmap"
)

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

type TimedRunnerInMessage struct {
	Msg RunnerInMessage

	TimeNow time.Time
}

type RunnerInMessage struct {
	TaskID             uuid.UUID              `json:"task_id"`
	IsAsyncResult      bool                   `json:"is_async_result"`
	FunctionMapping    map[string]interface{} `json:"function_mapping"`
	FunctionErrMapping map[string]interface{} `json:"function_err_mapping"`
	Err                string                 `json:"err"`
	DoRetry            bool                   `json:"do_retry"`
}

type TimedRunTaskMessage struct {
	Msg RunTaskMessage

	TimeNow time.Time
}

type RunTaskMessage struct {
	WorkNumber        string            `json:"work_number"`
	Description       string            `json:"description"`
	PipelineID        string            `json:"pipeline_id"`
	AttachmentFields  []string          `json:"attachment_fields"`
	Keys              map[string]string `json:"keys"`
	IsTestApplication bool              `json:"is_test_application"`
	CustomTitle       string            `json:"custom_title"`

	ClientID  string `json:"client_id"`
	RequestID string `json:"request_id"`
	Username  string `json:"user_name"`
	XAsOther  string `json:"x_as_other"`

	ApplicationBody orderedmap.OrderedMap `json:"application_body"`
}
