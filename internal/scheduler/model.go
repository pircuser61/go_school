package scheduler

type CreateTask struct {
	WorkNumber  string
	WorkID      string
	ActionName  string
	StepName    string
	WaitSeconds int
}

type DeleteTask struct {
	WorkID   string
	StepName string
}
