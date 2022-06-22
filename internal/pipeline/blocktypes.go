package pipeline

const (
	BlockScenario = "scenario"

	BlockInternalIf           = "if"
	BlockInternalStringsEqual = "strings_equal"
	BlockInternalForState     = "for_state"
	BlockInternalConnector    = "connector"

	// BlockGoTestTitle represents title of approver block (using in script.FunctionModel)
	BlockGoTestTitle = "go_test_block"
	// BlockGoTestID represents id/type of approver block (using in script.FunctionModel)
	BlockGoTestID = "go_test_block"

	// BlockGoApproverTitle represents title of approver block (using in script.FunctionModel)
	BlockGoApproverTitle = "approver"
	// BlockGoApproverID represents id/type of approver block (using in script.FunctionModel)
	BlockGoApproverID = "approver"

	BlockGoStartId = "start"
	BlockGoEndId   = "end"

	// BlockGoSdApplicationTitle represents id/type of sd block (using in script.FunctionModel)
	BlockGoSdApplicationTitle = "servicedesk_application"
	// BlockGoSdApplicationID represents id/type of sd block (using in script.FunctionModel)
	BlockGoSdApplicationID = "servicedesk_application"
)
