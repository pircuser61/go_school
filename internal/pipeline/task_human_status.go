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
	StatusWait                TaskHumanStatus = "wait"
)

var statusToTaskState = map[TaskHumanStatus]string{
	StatusNew:                 "была создана",
	StatusApproved:            "согласована",
	StatusApprovementRejected: "отклонена",
	StatusExecution:           "взята в работу",
	StatusExecutionRejected:   "отклонена исполнителем",
	StatusDone:                "выполнена исполнителем",
}

var statusToTaskAction = map[TaskHumanStatus]string{
	StatusApprovement: "согласования",
	StatusExecution:   "исполнения",
}
