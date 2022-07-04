package pipeline

type TaskHumanStatus string

const (
	StatusNew                 TaskHumanStatus = "new"
	StatusApprovement         TaskHumanStatus = "approvement"
	StatusExecuted            TaskHumanStatus = "executed"
	StatusExecution           TaskHumanStatus = "execution"
	StatusExecutionRejected   TaskHumanStatus = "execution-reject"
	StatusApproved            TaskHumanStatus = "approved"
	StatusApprovementRejected TaskHumanStatus = "approvement-reject"
	StatusDone                TaskHumanStatus = "done"
)
