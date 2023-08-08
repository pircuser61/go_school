package scheduler

type CreateTask struct {
	WorkNumber  string
	WorkID      string
	ActionName  string
	WaitSeconds int
}
