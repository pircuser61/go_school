package pipeline

type TaskHumanStatus string

const (
	StatusNew                 TaskHumanStatus = "new"
	StatusApprovement         TaskHumanStatus = "approvement"
	StatusExecution           TaskHumanStatus = "processing"
	StatusExecutionRejected   TaskHumanStatus = "executor-reject"
	StatusApproved            TaskHumanStatus = "approved"
	StatusApprovementRejected TaskHumanStatus = "approvement-reject"
	StatusDone                TaskHumanStatus = "done"
)
