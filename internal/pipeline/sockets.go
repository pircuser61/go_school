package pipeline

const (
	DefaultSocketID = "default"

	//nolint:varcheck,deadcode //used in tests
	rejectedSocketID         = "rejected"
	approverEditAppSocketID  = "approver_send_edit_app"
	executionEditAppSocketID = "send_edit_app"
	executedSocketID         = "executed"
	notExecutedSocketID      = "not_executed"
	requestAddInfoSocketID   = "req_add_info"
)
