package pipeline

type TaskHumanStatus string

const (
	StatusNew                 TaskHumanStatus = "new"
	StatusApprovement         TaskHumanStatus = "approvement"
	StatusApproveView         TaskHumanStatus = "approve-view"
	StatusApproveInform       TaskHumanStatus = "approve-inform"
	StatusApproveSign         TaskHumanStatus = "approve-sign"
	StatusApproveConfirm      TaskHumanStatus = "approve-confirm"
	StatusExecution           TaskHumanStatus = "processing"
	StatusExecutionRejected   TaskHumanStatus = "executor-reject"
	StatusApproved            TaskHumanStatus = "approved"
	StatusApproveViewed       TaskHumanStatus = "approve-viewed"
	StatusApproveInformed     TaskHumanStatus = "approve-informed"
	StatusApproveSigned       TaskHumanStatus = "approve-signed"
	StatusApproveConfirmed    TaskHumanStatus = "approve-confirmed"
	StatusApprovementRejected TaskHumanStatus = "approvement-reject"
	StatusDone                TaskHumanStatus = "done"
	StatusWait                TaskHumanStatus = "wait"
	StatusRevoke              TaskHumanStatus = "revoke"
)

var statusToTaskState = map[TaskHumanStatus]string{
	StatusNew:                 "была создана",
	StatusApproved:            "согласована",
	StatusApproveViewed:       "ознакомлено",
	StatusApproveInformed:     "проинформировано",
	StatusApproveSigned:       "подписана",
	StatusApproveConfirmed:    "утверждена",
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
