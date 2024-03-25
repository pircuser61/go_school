package pipeline

const (
	DefaultSocketID = "default"

	//nolint:varcheck,deadcode //used in tests
	signedSocketID           = "signed"
	rejectedSocketID         = "rejected"
	approverEditAppSocketID  = "approver_send_edit_app"
	executionEditAppSocketID = "executor_send_edit_app"
	executedSocketID         = "executed"
	notExecutedSocketID      = "not_executed"
	requestAddInfoSocketID   = "req_add_info"
	errorSocketID            = "error"
	funcTimeExpired          = "func_sla_expired"
	retryCountExceeded       = "retry_count_exceeded"
)
