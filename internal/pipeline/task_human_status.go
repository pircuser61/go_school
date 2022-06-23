package pipeline

type TaskHumanStatus string

const (
	StatusNew         TaskHumanStatus = "new"
	StatusApprovement TaskHumanStatus = "approvement"
	StatusExecution   TaskHumanStatus = "execution"
	StatusApproved    TaskHumanStatus = "approved"
	StatusDone        TaskHumanStatus = "done"
)
