package pipeline

type TaskHumanStatus string

const (
	StatusNew         TaskHumanStatus = "new"
	StatusApprovement TaskHumanStatus = "approvement"
	StatusApproved    TaskHumanStatus = "approved"
	StatusDone        TaskHumanStatus = "done"
)
