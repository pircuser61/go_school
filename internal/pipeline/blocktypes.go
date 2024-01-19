package pipeline

const (
	BlockScenario = "scenario"

	BlockInternalStringsEqual = "strings_equal"
	BlockInternalForState     = "for_state"
	BlockInternalConnector    = "connector"

	// BlockGoTestTitle represents title of approver block (using in script.FunctionModel)
	BlockGoTestTitle = "go_test_block"
	// BlockGoTestID represents id/type of approver block (using in script.FunctionModel)
	BlockGoTestID = "go_test_block"

	// BlockGoApproverID represents id/type of approver block (using in script.FunctionModel)
	BlockGoApproverID = "approver"

	BlockGoFormID = "form"

	BlockGoStartID    = "start"
	BlockGoFirstStart = "start_0"
	BlockGoStartTitle = "start"
	BlockGoEndID      = "end"
	BlockGoEndTitle   = "end"

	BlockWaitForAllInputsID      = "wait_for_all_inputs"
	BlockGoWaitForAllInputsTitle = "wait_for_all_inputs"

	BlockGoBeginParallelTaskID    = "begin_parallel_task"
	BlockGoBeginParallelTaskTitle = "begin_parallel_task"

	// BlockGoSdApplicationTitle represents id/type of sd block (using in script.FunctionModel)
	BlockGoSdApplicationTitle = "servicedesk_application"
	// BlockGoSdApplicationID represents id/type of sd block (using in script.FunctionModel)
	BlockGoSdApplicationID = "servicedesk_application"

	// BlockGoIfTitle represents id/type of sd block (using in script.FunctionModel)
	BlockGoIfTitle = "if"
	// BlockGoIfID represents id/type of sd block (using in script.FunctionModel)
	BlockGoIfID = "if"

	// BlockGoExecutionID represents id/type of execution block (using in script.FunctionModel)
	BlockGoExecutionID = "execution"

	// BlockGoSignID represents id/type of sign block (using in script.FunctionModel)
	BlockGoSignID = "sign"

	BlockGoNotificationID    = "notification"
	BlockGoNotificationTitle = "notification"

	BlockExecutableFunctionID    = "executable_function"
	BlockExecutableFunctionTitle = "executable_function"

	BlockTimerID    = "timer"
	BlockTimerTitle = "Таймер"

	BlockPlaceholderID    = "placeholder"
	BlockPlaceholderTitle = "placeholder"
)
