package pipeline

type TaskHumanStatus string

const (
	StatusNew                 TaskHumanStatus = "new"
	StatusApprovement         TaskHumanStatus = "approvement"
	StatusApproveView         TaskHumanStatus = "approve-view"
	StatusApproveInform       TaskHumanStatus = "approve-inform"
	StatusApproveSign         TaskHumanStatus = "approve-sign"
	StatusApproveSignUkep     TaskHumanStatus = "approve-sign-ukep"
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
	StatusCancel              TaskHumanStatus = "cancel"
	StatusSigning             TaskHumanStatus = "signing"
	StatusSignSigned          TaskHumanStatus = "signed"
	StatusSignRejected        TaskHumanStatus = "sign-reject"
	StatusProcessingError     TaskHumanStatus = "error"
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
	StatusCancel:              "отозвана администратором",
	StatusSigning:             "на подписании",
	StatusSignSigned:          "подписана",
	StatusSignRejected:        "отклонена",
	StatusProcessingError:     "обработана с ошибкой",
}

var statusToTaskAction = map[TaskHumanStatus]string{
	StatusApprovement: "согласования",
	StatusExecution:   "исполнения",
}
