package pipeline

type TaskHumanStatus string

const (
	StatusNew                 TaskHumanStatus = "new"
	StatusApprovement         TaskHumanStatus = "approvement"
	StatusApproveView         TaskHumanStatus = "approve-view"
	StatusApproveInform       TaskHumanStatus = "approve-inform"
	StatusApproveSignUkep     TaskHumanStatus = "approve-sign-ukep"
	StatusApproveConfirm      TaskHumanStatus = "approve-confirm"
	StatusExecution           TaskHumanStatus = "processing"
	StatusExecutionRejected   TaskHumanStatus = "executor-reject"
	StatusApproved            TaskHumanStatus = "approved"
	StatusApproveViewed       TaskHumanStatus = "approve-viewed"
	StatusApproveInformed     TaskHumanStatus = "approve-informed"
	StatusApproveConfirmed    TaskHumanStatus = "approve-confirmed"
	StatusApprovementRejected TaskHumanStatus = "approvement-reject"
	StatusDone                TaskHumanStatus = "done"
	StatusWait                TaskHumanStatus = "wait"
	StatusRevoke              TaskHumanStatus = "revoke"
	StatusCancel              TaskHumanStatus = "cancel"
	StatusSigning             TaskHumanStatus = "signing"
	StatusSigned              TaskHumanStatus = "signed"
	StatusRejected            TaskHumanStatus = "rejected"
	StatusProcessingError     TaskHumanStatus = "error"
)

//nolint:gochecknoglobals // тут слишком много завязано на глобальных переменных
var statusToTaskState = map[TaskHumanStatus]string{
	StatusNew:                 "успешно создана",
	StatusApproved:            "согласована",
	StatusApproveViewed:       "ознакомлено",
	StatusApproveInformed:     "проинформировано",
	StatusApproveConfirmed:    "утверждена",
	StatusApprovementRejected: "отклонена",
	StatusExecution:           "взята в работу",
	StatusExecutionRejected:   "отклонена исполнителем",
	StatusDone:                "выполнена исполнителем",
	StatusRevoke:              "отозвана инициатором",
	StatusCancel:              "отозвана администратором",
	StatusSigning:             "на подписании",
	StatusSigned:              "подписана",
	StatusRejected:            "отклонена",
	StatusProcessingError:     "обработана с ошибкой",
}

//nolint:gochecknoglobals // тут слишком много завязано на глобальных переменных
var positiveTaskState = []string{
	"согласована",
	"ознакомлено",
	"проинформировано",
	"утверждена",
	"выполнена исполнителем",
	"подписана",
}

//nolint:gochecknoglobals // тут слишком много завязано на глобальных переменных
var statusToTaskAction = map[TaskHumanStatus]string{
	StatusApprovement: "согласования",
	StatusExecution:   "исполнения",
}
