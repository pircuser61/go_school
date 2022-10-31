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
	StatusRevoke              TaskHumanStatus = "revoke"
)

var statusToTaskState = map[TaskHumanStatus]string{
	StatusNew:                 "была создана",
	StatusApproved:            "согласована",
	StatusApprovementRejected: "отклонена",
	StatusExecution:           "взята в работу",
	StatusExecutionRejected:   "отклонена исполнителем",
	StatusDone:                "выполнена исполнителем",
	StatusRevoke:              "отозвана инициатором",
}

var statusToTaskAction = map[TaskHumanStatus]string{
	StatusApprovement: "согласования",
	StatusExecution:   "исполнения",
}
